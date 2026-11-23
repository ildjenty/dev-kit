package refine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/jev"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func reflectionArtifacts(t *testing.T, documents map[string]string, ds ...doc) (string, string) {
	t.Helper()
	root := t.TempDir()
	artifacts := filepath.Join(root, "artifacts")
	if len(ds) == 0 {
		ds = items(draft()["decisions"])
	}
	testutil.OK(t, writeJSON(file(artifacts, "input.json"), doc{"folders": doc{"docs": "docs"}}))
	testutil.OK(t, writeJSON(file(artifacts, "draft.json"), doc{"decisions": ds}))
	testutil.OK(t, writeJSON(file(artifacts, "check.json"), doc{"valid": true}))
	for path, text := range documents {
		testutil.Put(t, root, path, text)
	}
	return root, artifacts
}
func reflectionResponse(t *testing.T, r *http.Request, w http.ResponseWriter, values map[string]float64) doc {
	t.Helper()
	var input doc
	testutil.OK(t, json.NewDecoder(r.Body).Decode(&input))
	answers := doc{}
	for id := range obj(input["questions"]) {
		answers[id] = doc{"type": "noul", "noul": values[id]}
	}
	testutil.OK(t, json.NewEncoder(w).Encode(doc{"model": "jev-reported", "answers": answers}))
	return input
}
func TestReflectJevThresholdAndMultipleDecisionsPerChunk(t *testing.T) {
	first := clone(items(draft()["decisions"])[0])
	second := clone(first)
	second["key"], second["title"], second["body"] = "N2", "Other choice", "Choose the other option, following N1."
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": "# Topic\nN1 and N2 choose employees and another option.\n", "docs/unrelated.md": "Not cited."}, first, second)
	seen := make(chan doc, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.Equal(t, r.Header.Get("Authorization"), "Bearer secret")
		seen <- reflectionResponse(t, r, w, map[string]float64{"N1": .5, "N2": .499999})
	}))
	defer server.Close()
	result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
	testutil.Equal(t, result["available"], true)
	testutil.Equal(t, result["model"], "jev-reported")
	missing := items(result["not_reflected"])
	testutil.Equal(t, len(missing), 1)
	testutil.Equal(t, missing[0], doc{"decision": "N2", "title": "Other choice", "documents": []string{"docs/topic.md"}, "probability": .499999})
	request := <-seen
	testutil.Equal(t, request["model"], "jev-latest")
	testutil.Equal(t, len(obj(request["questions"])), 2)
	for _, d := range []doc{first, second} {
		question := obj(obj(request["questions"])[str(d["key"])])
		testutil.Equal(t, question["type"], "noul")
		testutil.Contains(t, str(question["instructions"]), str(d["title"]))
		testutil.Contains(t, str(question["instructions"]), str(d["body"]))
		testutil.Contains(t, str(obj(question["criteria"])["false"]), "rejected or superseded")
	}
	testutil.Equal(t, obj(request["state"]), doc{"document": "docs/topic.md", "text": "# Topic\nN1 and N2 choose employees and another option.\n"})
	testutil.Equal(t, len(items(result["probabilities"])), 2)
}
func TestReflectJevAnyCitingDocumentOrChunkCanReflect(t *testing.T) {
	text := "# Earlier\nN1 was considered but rejected.\n" + strings.Repeat("old wording\n", 500) + "# Current\nChoose employees as N1 requires.\n"
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/one.md": text, "docs/two.md": "N1 says something different."})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data := testutil.Must(io.ReadAll(r.Body))
		p := .1
		if bytes.Contains(data, []byte("# Current")) {
			p = .9
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		reflectionResponse(t, r, w, map[string]float64{"N1": p})
	}))
	defer server.Close()
	result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
	testutil.Equal(t, result["available"], true)
	testutil.Equal(t, len(items(result["not_reflected"])), 0)
	testutil.Equal(t, calls.Load(), int32(2))
	testutil.Equal(t, len(items(result["probabilities"])), 2)
	root, artifacts = reflectionArtifacts(t, map[string]string{"docs/one.md": "N1 was rejected.", "docs/two.md": "N1 is chosen."})
	result = reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
	missing := items(result["not_reflected"])
	testutil.Equal(t, len(missing), 1)
	testutil.Equal(t, missing[0]["documents"], []string{"docs/one.md", "docs/two.md"})
	testutil.Equal(t, missing[0]["probability"], .1)
}
func TestDocumentChunksAtHeadingsPreserveTextAndSize(t *testing.T) {
	section := func(title string) string { return "## " + title + "\n" + strings.Repeat("日本語 text\n", 350) }
	text := section("First") + section("Second") + section("Third")
	chunks := documentChunks(text)
	testutil.Equal(t, len(chunks), 3)
	testutil.Equal(t, strings.Join(chunks, ""), text)
	for _, chunk := range chunks {
		testutil.Equal(t, strings.HasPrefix(chunk, "## "), true)
		if len(chunk) > reflectChunkBytes {
			t.Fatal("oversized chunk")
		}
	}
	for _, text := range []string{strings.Repeat("日本語", reflectChunkBytes), "Title\n===\n" + strings.Repeat("word\n", 1000) + "Next\n---\n" + strings.Repeat("word\n", 1000), "# Title\n```md\n# Not a heading\n```\n"} {
		chunks := documentChunks(text)
		testutil.Equal(t, strings.Join(chunks, ""), text)
		for _, chunk := range chunks {
			if !utf8.ValidString(chunk) || len(chunk) > reflectChunkBytes {
				t.Fatal("invalid chunk")
			}
		}
	}
}
func TestReflectJevChunkRequestsAndEncodedSize(t *testing.T) {
	text := "# First\nN1\n" + strings.Repeat("日本語\x00\n", 1000) + "# Second\n" + strings.Repeat("quoted \" \n", 1200)
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": text})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data := testutil.Must(io.ReadAll(r.Body))
		if len(data) > reflectRequestBytes {
			t.Error("oversized request")
		}
		var input doc
		testutil.OK(t, json.Unmarshal(data, &input))
		part := str(obj(input["state"])["text"])
		if !utf8.ValidString(part) || len(part) > reflectChunkBytes {
			t.Error("invalid chunk")
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		reflectionResponse(t, r, w, map[string]float64{"N1": .5})
	}))
	defer server.Close()
	result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
	testutil.Equal(t, result["available"], true)
	if calls.Load() < 3 {
		t.Fatal("long document was not split")
	}
	testutil.Equal(t, len(items(result["probabilities"])), int(calls.Load()))
}
func TestReflectJevUsesReproposalCitationsAndRewritesDecisionKeys(t *testing.T) {
	d := clone(items(draft()["decisions"])[0])
	d["body"] = "N1 chooses employees."
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": "D-abcdef chooses employees."}, d)
	testutil.OK(t, writeJSON(file(artifacts, "input.json"), doc{"folders": doc{"docs": "docs"}, "pending_proposal": doc{"content": doc{"mapping": doc{"N1": "D-abcdef"}}}}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := reflectionResponse(t, r, w, map[string]float64{"N1": 1})
		testutil.Contains(t, str(obj(obj(input["questions"])["N1"])["instructions"]), "D-abcdef chooses employees.")
	}))
	defer server.Close()
	result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
	testutil.Equal(t, result["available"], true)
	testutil.Equal(t, len(items(result["not_reflected"])), 0)
}
func TestNewDecisionMustBeCitedByADocument(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.put("docs/refined.md", "# No citation\nChoose employees. N10 is another key.\n")
	testutil.Contains(t, r.errors(draft()), "N1: new decision is not reflected in a document")
	r.put("docs/refined.md", "# Cited\nN1\n")
	testutil.Equal(t, len(r.errors(draft())), 0)
	r.put("docs/refined.md", "# Cited\nN1\n")
	checked := testutil.Must(check(r.ctx, r.artifacts, "document"))
	testutil.Equal(t, checked["valid"], true)
	input := testutil.Must(readJSON(file(r.artifacts, "reflect-input.json")))
	testutil.Equal(t, len(items(input["documents"])), 1)
}
func TestReflectJevUnavailableDoesNotUsePartialResults(t *testing.T) {
	for _, bad := range []string{"not JSON secret", `{}`, `{"model":"m","answers":{}}`, `{"model":"m","answers":{"N1":{"type":"noul","noul":null}}}`, `{"model":"m","answers":{"N1":{"type":"noul","noul":1.01}}}`, `{"model":"m","answers":{"N1":{"type":"noul","noul":-0.1}}}`, `{"model":"m","answers":{"N1":{"type":"text","noul":0.1}}}`} {
		t.Run(bad, func(t *testing.T) {
			root, artifacts := reflectionArtifacts(t, map[string]string{"docs/good.md": "N1", "docs/bad.md": "N1"})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data := testutil.Must(io.ReadAll(r.Body))
				if bytes.Contains(data, []byte("docs/bad.md")) {
					fmt.Fprint(w, bad)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(data))
				reflectionResponse(t, r, w, map[string]float64{"N1": 0})
			}))
			defer server.Close()
			result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
			testutil.Equal(t, result["available"], false)
			testutil.Equal(t, len(items(result["not_reflected"])), 0)
			testutil.Equal(t, len(items(result["probabilities"])), 1)
			if bytes.Contains(testutil.Must(json.Marshal(result)), []byte("secret")) {
				t.Fatal("key leaked")
			}
		})
	}
}
func TestReflectJevUnavailableInputsAndOversizedQuestions(t *testing.T) {
	for _, mode := range []string{"missing check", "invalid check", "missing draft", "missing input", "no citation", "questions too large"} {
		t.Run(mode, func(t *testing.T) {
			root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": "N1"})
			switch mode {
			case "missing check":
				testutil.OK(t, os.Remove(file(artifacts, "check.json")))
			case "invalid check":
				testutil.OK(t, writeJSON(file(artifacts, "check.json"), doc{"valid": false}))
			case "missing draft":
				testutil.OK(t, os.Remove(file(artifacts, "draft.json")))
			case "missing input":
				testutil.OK(t, os.Remove(file(artifacts, "input.json")))
			case "no citation":
				testutil.Put(t, root, "docs/topic.md", "no citation")
			case "questions too large":
				d := clone(items(draft()["decisions"])[0])
				d["body"] = strings.Repeat("x", reflectRequestBytes)
				testutil.OK(t, writeJSON(file(artifacts, "draft.json"), doc{"decisions": []doc{d}}))
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") }))
			defer server.Close()
			result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
			testutil.Equal(t, result["available"], false)
			testutil.Equal(t, len(items(result["not_reflected"])), 0)
		})
	}
}
func TestReflectJevHTTPFailuresAndTimeout(t *testing.T) {
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/topic.md": "N1"})
	for _, status := range []int{402, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, "secret")
			}))
			defer server.Close()
			result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, server.Client(), 0))
			testutil.Equal(t, result["available"], false)
			want := int32(2)
			if status == 402 {
				want = 1
			}
			testutil.Equal(t, calls.Load(), want)
			testutil.Equal(t, len(items(result["not_reflected"])), 0)
			if bytes.Contains(testutil.Must(json.Marshal(result)), []byte("secret")) {
				t.Fatal("key leaked")
			}
		})
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); time.Sleep(80 * time.Millisecond) }))
	defer server.Close()
	client := server.Client()
	client.Timeout = 30 * time.Millisecond
	result := reflectJev(root, artifacts, jev.NewClient("secret", server.URL, client, 0))
	testutil.Equal(t, result["available"], false)
	testutil.Contains(t, str(result["reason"]), "timed out")
	testutil.Equal(t, calls.Load(), int32(2))
}
func TestReflectJevCommandNoKeyAndSecretRedaction(t *testing.T) {
	root, artifacts := reflectionArtifacts(t, map[string]string{"docs/secret.md": "N1"})
	testutil.Put(t, root, "typesafe-apikey.txt", "secret")
	d := clone(items(draft()["decisions"])[0])
	d["title"] = "secret title"
	testutil.OK(t, writeJSON(file(artifacts, "draft.json"), doc{"decisions": []doc{d}}))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"model":"secret-model","answers":{"N1":{"type":"noul","noul":0.4}}}`)
	}))
	defer server.Close()
	testutil.Put(t, root, "pack/refine/commands/reflect.md", "Check $ARTIFACTS_DIR/refine/reflect-input.json")
	fake := filepath.Join(root, "fake-codex")
	testutil.OK(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf '{\"not_reflected\":[]}' > \"$9\"\n"), 0700))
	for _, key := range []string{"", "secret"} {
		var stdout, stderr bytes.Buffer
		env := envVars(map[string]string{"ARTIFACTS_DIR": artifacts, "TYPESAFE_API_KEY": key, "TYPESAFE_API_URL": server.URL, "PACK_DIR": filepath.Join(root, "pack"), "DEVKIT_CODEX": fake})
		env.Cwd = root
		env.Stdout = &stdout
		env.Stderr = &stderr
		testutil.Equal(t, cli.New("", Commands()).Execute([]string{"refine", "reflect"}, env), 0)
		var result doc
		testutil.OK(t, json.Unmarshal(stdout.Bytes(), &result))
		testutil.Equal(t, result["source"], map[bool]string{true: "jev", false: "codex"}[key != ""])
		artifact := testutil.Must(os.ReadFile(file(artifacts, "reflect-jev.json")))
		for _, data := range [][]byte{stdout.Bytes(), stderr.Bytes(), artifact} {
			if bytes.Contains(data, []byte("secret")) {
				t.Fatal("key leaked")
			}
		}
		testutil.Equal(t, result["probabilities"], nil)
		if key != "" {
			testutil.Contains(t, string(artifact), `"probability": 0.4`)
		} else {
			testutil.Equal(t, calls.Load(), int32(0))
		}
	}
	testutil.Equal(t, calls.Load(), int32(1))
}
func TestReflectionFindingsBlockUntilNextRoundClears(t *testing.T) {
	for _, source := range []string{"jev", "codex"} {
		t.Run(source, func(t *testing.T) {
			r := setup(t, nil)
			r.begin()
			r.writeDraft()
			reflection := doc{"source": source, "not_reflected": []doc{{"decision": "N1", "documents": []string{"docs/refined.md"}, "reason": "Rejected only", "probability": .4}}}
			j := testutil.Must(roundReflection(envVars(map[string]string{"INPUTS_REFLECT": string(testutil.Must(json.Marshal(reflection)))}), judgment()))
			result := r.round(j)
			testutil.Equal(t, result["blocking_count"], 1)
			entry := testutil.Must(readJSON(file(r.artifacts, "document/rounds/latest.json")))
			testutil.Equal(t, entry["cleared"], false)
			finding := items(obj(entry["judgment"])["findings"])[0]
			testutil.Equal(t, finding["aspect"], "reflection")
			testutil.Contains(t, str(finding["failure_scenario"]), "N1")
			testutil.Contains(t, str(finding["failure_scenario"]), "docs/refined.md")
			testutil.Equal(t, r.round(reflectionJudgment(judgment(), doc{"not_reflected": []doc{}}))["blocking_count"], 0)
			testutil.Equal(t, testutil.Must(readJSON(file(r.artifacts, "document/rounds/latest.json")))["cleared"], true)
		})
	}
	_, err := roundReflection(envVars(map[string]string{"INPUTS_REFLECT": "null"}), judgment())
	testutil.Fails(t, err, "reflection result is required")
}
func TestReflectWorkflowFallbackContract(t *testing.T) {
	var workflow doc
	testutil.OK(t, yaml.Unmarshal(testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/refine/refine.yaml")), &workflow))
	nodes := map[string]doc{}
	for _, node := range items(workflow["nodes"]) {
		if node["id"] == "document" {
			for _, n := range items(obj(node["loop_group"])["nodes"]) {
				nodes[str(n["id"])] = n
			}
		}
	}
	testutil.Equal(t, nodes["reflect-jev"], nil)
	testutil.Equal(t, nodes["reflect"]["when"], "$check-document.output.valid == true")
	testutil.Equal(t, nodes["reflect"]["bash"], "devkit refine reflect")
	for _, id := range []string{"judge-document", "round-document"} {
		testutil.Equal(t, nodes[id]["trigger_rule"], "none_failed_min_one_success")
	}
	testutil.Contains(t, str(nodes["round-document"]["bash"]), "INPUTS_REFLECT=")
}

func TestRoundRejectsMalformedReflection(t *testing.T) {
	for _, reflection := range []string{`{}`, `{"not_reflected":null}`, `{"not_reflected":[{}]}`, `{"not_reflected":[{"decision":"N1","documents":[]}]}`, `invalid`} {
		_, err := roundReflection(envVars(map[string]string{"INPUTS_REFLECT": reflection}), judgment())
		if err == nil {
			t.Fatal("malformed reflection result accepted")
		}
	}
}
