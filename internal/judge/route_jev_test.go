package judge

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
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func jevArtifacts(t *testing.T, files []doc, patch string) string {
	t.Helper()
	a := t.TempDir()
	testutil.Standards(t, a, "criteria/fixed")
	testutil.Put(t, a, "judge/diff.patch", patch)
	testutil.OK(t, save(filepath.Join(a, "judge/target.json"), doc{"standards": "fixed"}))
	testutil.OK(t, save(filepath.Join(a, "judge/changes.json"), doc{"files": files}))
	return a
}
func jevInput(t *testing.T) string {
	return jevArtifacts(t, []doc{{"path": "odd/file", "status": "M"}}, "diff --git a/odd/file b/odd/file\n--- a/odd/file\n+++ b/odd/file\n@@ -1 +1 @@\n-old\n+new\n")
}
func jevResponse(t *testing.T, r *http.Request, w http.ResponseWriter, values map[string]float64) doc {
	t.Helper()
	var request doc
	testutil.OK(t, json.NewDecoder(r.Body).Decode(&request))
	answers := doc{}
	for id := range object(request["questions"]) {
		answers[id] = doc{"type": "noul", "noul": values[id]}
	}
	testutil.OK(t, json.NewEncoder(w).Encode(doc{"model": "jev-reported", "answers": answers, "usage": doc{"input_tokens": 1}}))
	return request
}
func TestJevSelectionThresholdAndFixedQuestions(t *testing.T) {
	a := jevInput(t)
	mapping := must(concerns.Load(filepath.Join(a, "criteria/fixed")))
	seen := make(chan doc, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.Equal(t, r.Method, "POST")
		testutil.Equal(t, r.Header.Get("Authorization"), "Bearer secret")
		testutil.Equal(t, r.Header.Get("Content-Type"), "application/json")
		seen <- jevResponse(t, r, w, map[string]float64{"security": .3, "contract": .299999, "data": 1, "source_or_test": .3})
	}))
	defer s.Close()
	out := routeJev(a, "secret", s.URL, s.Client(), 0)
	testutil.Equal(t, out["available"], true)
	testutil.Equal(t, out["model"], "jev-reported")
	qualities := []string{}
	for _, item := range docs(out["qualities"]) {
		qualities = append(qualities, text(item["quality"]))
		testutil.Equal(t, item["file"], "odd/file")
		testutil.Contains(t, text(item["reason"]), "probability")
	}
	testutil.Equal(t, qualities, []string{"data", "security", "test", "readability"})
	request := <-seen
	testutil.Equal(t, request["model"], "jev-latest")
	questions := object(request["questions"])
	testutil.Equal(t, len(questions), 7)
	for _, id := range []string{"scope", "delivery", "test", "readability"} {
		if questions[id] != nil {
			t.Fatalf("unexpected question %s", id)
		}
	}
	for _, q := range []string{"structure", "data", "contract", "security", "ui", "cost"} {
		question := object(questions[q])
		testutil.Equal(t, question["type"], "noul")
		testutil.Contains(t, text(question["instructions"]), mapping.Qualities[q].Routing)
	}
	testutil.Equal(t, len(docs(out["probabilities"])), 7)
}
func TestJevUsesEachQualitysOwnRoutingThreshold(t *testing.T) {
	a := jevInput(t)
	mapping := must(concerns.Load(filepath.Join(a, "criteria/fixed")))
	contract := mapping.Qualities["contract"].RoutingThreshold
	security := mapping.Qualities["security"].RoutingThreshold
	if contract <= security {
		t.Fatalf("the fixture needs contract above security: %g <= %g", contract, security)
	}
	between := (contract + security) / 2
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jevResponse(t, r, w, map[string]float64{"security": between, "contract": between, "cost": mapping.Qualities["cost"].RoutingThreshold})
	}))
	defer s.Close()
	out := routeJev(a, "secret", s.URL, s.Client(), 0)
	qualities := []string{}
	for _, item := range docs(out["qualities"]) {
		qualities = append(qualities, text(item["quality"]))
	}
	testutil.Equal(t, qualities, []string{"cost", "security"})
}
func TestJevChunksPreserveDiffAndMergeSelection(t *testing.T) {
	patch := "diff --git a/odd/file b/odd/file\n"
	for i := 0; i < jevChunkBytes; i++ {
		patch += fmt.Sprintf("+line %d 日本語\n", i)
	}
	a := jevArtifacts(t, []doc{{"path": "odd/file"}}, patch)
	var mu sync.Mutex
	parts := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body doc
		testutil.OK(t, json.NewDecoder(r.Body).Decode(&body))
		part := text(object(body["state"])["change"])
		if !utf8.ValidString(part) || len(part) > jevChunkBytes {
			t.Errorf("invalid chunk")
		}
		mu.Lock()
		parts = append(parts, part)
		mu.Unlock()
		answers := doc{}
		for id := range object(body["questions"]) {
			answers[id] = doc{"type": "noul", "noul": .1}
		}
		answers["security"] = doc{"type": "noul", "noul": .7}
		if strings.Contains(part, "diff --git") {
			answers["security"] = doc{"type": "noul", "noul": .9}
			answers["cost"] = doc{"type": "noul", "noul": .95}
		}
		testutil.OK(t, json.NewEncoder(w).Encode(doc{"model": "jev-reported", "answers": answers}))
	}))
	defer s.Close()
	out := routeJev(a, "secret", s.URL, s.Client(), 0)
	testutil.Equal(t, out["available"], true)
	probabilities := docs(out["probabilities"])
	testutil.Equal(t, len(probabilities), len(parts)*7)
	testutil.Equal(t, len(docs(out["qualities"])), 2)
	testutil.Equal(t, docs(out["qualities"])[0]["quality"], "cost")
	testutil.Contains(t, text(docs(out["qualities"])[1]["reason"]), "probability 0.9")
	reconstructed := ""
	for len(parts) > 0 {
		found := false
		for i, part := range parts {
			if strings.HasPrefix(patch[len(reconstructed):], part) {
				reconstructed += part
				parts = append(parts[:i], parts[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("chunk bytes lost or reordered")
		}
	}
	testutil.Equal(t, reconstructed, patch)
	for i, p := range probabilities {
		testutil.Equal(t, p["chunk"], i/7)
	}
}
func TestJevConcurrencyLimit(t *testing.T) {
	files := []doc{}
	patch := ""
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("file%d", i)
		files = append(files, doc{"path": name})
		patch += fmt.Sprintf("diff --git a/%s b/%s\n+x\n", name, name)
	}
	a := jevArtifacts(t, files, patch)
	var active, peak atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		if current == jevConcurrency {
			once.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(time.Second):
			t.Error("requests were not concurrent")
		}
		jevResponse(t, r, w, nil)
	}))
	defer s.Close()
	out := routeJev(a, "secret", s.URL, s.Client(), 0)
	testutil.Equal(t, out["available"], true)
	testutil.Equal(t, peak.Load(), int32(jevConcurrency))
	testutil.Equal(t, len(docs(out["probabilities"])), 20*7)
}
func TestJevFailuresAndRetries(t *testing.T) {
	a := jevInput(t)
	for _, tc := range []struct {
		name    string
		status  int
		recover bool
		calls   int
	}{
		{"credit", 402, false, 1}, {"rate limit", 429, false, 2}, {"server", 500, false, 2},
		{"rate limit recovers", 429, true, 2}, {"server recovers", 500, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if !tc.recover || n == 1 {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, "secret")
					return
				}
				jevResponse(t, r, w, nil)
			}))
			defer s.Close()
			out := routeJev(a, "secret", s.URL, s.Client(), time.Millisecond)
			testutil.Equal(t, out["available"], tc.recover)
			testutil.Equal(t, calls.Load(), int32(tc.calls))
			data := must(json.Marshal(out))
			if bytes.Contains(data, []byte("secret")) {
				t.Fatal("key leaked")
			}
			if !tc.recover {
				testutil.Equal(t, out["qualities"], []doc{})
				testutil.Contains(t, text(out["reason"]), fmt.Sprint(tc.status))
			}
		})
	}
}
func TestJevTimeoutAndRetry(t *testing.T) {
	a := jevInput(t)
	for _, recover := range []bool{false, true} {
		t.Run(fmt.Sprint(recover), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if !recover || n == 1 {
					time.Sleep(80 * time.Millisecond)
					return
				}
				jevResponse(t, r, w, nil)
			}))
			defer s.Close()
			client := s.Client()
			client.Timeout = 30 * time.Millisecond
			out := routeJev(a, "secret", s.URL, client, time.Millisecond)
			testutil.Equal(t, calls.Load(), int32(2))
			testutil.Equal(t, out["available"], recover)
			if !recover {
				testutil.Contains(t, text(out["reason"]), "timed out")
				testutil.Equal(t, out["qualities"], []doc{})
			}
		})
	}
}
func TestJevMalformedAnswersAndPartialFailure(t *testing.T) {
	a := jevInput(t)
	for _, bad := range []string{"not JSON secret", `{}`, `{"model":"m","answers":{}}`, `{"model":"m","answers":{"cost":{"type":"noul","noul":null}}}`} {
		var calls atomic.Int32
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); fmt.Fprint(w, bad) }))
		out := routeJev(a, "secret", s.URL, s.Client(), 0)
		s.Close()
		testutil.Equal(t, calls.Load(), int32(1))
		testutil.Equal(t, out["available"], false)
		testutil.Equal(t, out["qualities"], []doc{})
	}
	for _, bad := range []any{nil, "0.3", -0.01, 1.01, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request doc
			testutil.OK(t, json.NewDecoder(r.Body).Decode(&request))
			answers := doc{}
			for id := range object(request["questions"]) {
				answers[id] = doc{"type": "noul", "noul": .5}
			}
			answers["security"] = doc{"type": "noul", "noul": bad}
			json.NewEncoder(w).Encode(doc{"model": "m", "answers": answers})
		}))
		out := routeJev(a, "secret", s.URL, s.Client(), 0)
		s.Close()
		testutil.Equal(t, out["available"], false)
		testutil.Equal(t, out["qualities"], []doc{})
	}
	a = jevArtifacts(t, []doc{{"path": "good"}, {"path": "bad"}}, "diff --git a/good b/good\n+x\ndiff --git a/bad b/bad\n+y\n")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := must(io.ReadAll(r.Body))
		if bytes.Contains(data, []byte(`"file":"bad"`)) {
			w.WriteHeader(500)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		jevResponse(t, r, w, map[string]float64{"security": 1})
	}))
	defer s.Close()
	out := routeJev(a, "secret", s.URL, s.Client(), 0)
	testutil.Equal(t, out["available"], false)
	testutil.Equal(t, out["qualities"], []doc{})
	testutil.Equal(t, len(docs(out["probabilities"])), 7)
}
func TestJevCommandNoKeyAndSecretRedaction(t *testing.T) {
	a := jevInput(t)
	testutil.Put(t, a, "typesafe-apikey.txt", "secret")
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request doc
		json.NewDecoder(r.Body).Decode(&request)
		answers := doc{}
		for id := range object(request["questions"]) {
			answers[id] = doc{"type": "noul", "noul": .25}
		}
		json.NewEncoder(w).Encode(doc{"model": "echo-secret", "answers": answers})
	}))
	defer s.Close()
	for _, key := range []string{"", "secret"} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		env := cli.Env{Cwd: a, Stdout: stdout, Stderr: stderr, LookupEnv: func(k string) (string, bool) {
			v, ok := map[string]string{"ARTIFACTS_DIR": a, "TYPESAFE_API_KEY": key, "TYPESAFE_API_URL": s.URL}[k]
			return v, ok
		}}
		testutil.Equal(t, cli.New("", Commands()).Execute([]string{"judge", "route"}, env), 0)
		var result doc
		testutil.OK(t, json.Unmarshal(stdout.Bytes(), &result))
		testutil.Equal(t, result["source"], map[bool]string{true: "jev", false: "all"}[key != ""])
		testutil.Equal(t, calls.Load(), int32(map[bool]int{true: 0, false: 1}[key == ""]))
		artifact := must(os.ReadFile(filepath.Join(a, "judge/route-jev.json")))
		for _, data := range [][]byte{stdout.Bytes(), stderr.Bytes(), artifact} {
			if bytes.Contains(data, []byte("secret")) {
				t.Fatal("key leaked")
			}
		}
		if key != "" {
			testutil.Contains(t, string(artifact), `"probability": 0.25`)
			if result["probabilities"] != nil {
				t.Fatal("probabilities should be in artifact only")
			}
		}
	}
}
func TestJevFileDiffHandlesGitPathsAndRename(t *testing.T) {
	repo := testutil.Repo(t)
	paths := []string{"plain", "space name", "日本語", "line\nbreak", "quote\"tab\t", "back\\slash", "control\a\b\v\f"}
	for _, p := range paths {
		testutil.Put(t, repo, p, "before\n")
	}
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "before")
	before := testutil.Git(t, repo, "rev-parse", "HEAD")
	for _, p := range paths {
		testutil.Put(t, repo, p, "after\n")
	}
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "after")
	g := must(gitx.New(repo, "main"))
	for _, quote := range []string{"true", "false"} {
		testutil.Git(t, repo, "config", "core.quotePath", quote)
		patch := must(g.Run("diff", "--binary", before, "HEAD")) + "\n"
		for _, p := range paths {
			actual := jevFileDiff(patch, doc{"path": p})
			expected := must(g.Run("diff", "--binary", before, "HEAD", "--", p)) + "\n"
			testutil.Equal(t, actual, expected)
		}
	}
	testutil.Git(t, repo, "mv", "plain", "renamed")
	testutil.Git(t, repo, "commit", "-qm", "rename")
	patch := must(g.Run("diff", "--binary", "HEAD~1", "HEAD")) + "\n"
	testutil.Equal(t, jevFileDiff(patch, doc{"path": "renamed", "oldPath": "plain"}), patch)
}
func TestJevWorkflowFallbackContract(t *testing.T) {
	data := must(os.ReadFile("../../kit/.archon/workflows/dev/judge/judge.yaml"))
	var workflow doc
	testutil.OK(t, yaml.Unmarshal(data, &workflow))
	nodes := map[string]doc{}
	for _, n := range docs(workflow["nodes"]) {
		nodes[text(n["id"])] = n
	}
	testutil.Equal(t, nodes["route-jev"], nil)
	testutil.Equal(t, nodes["route"]["bash"], "devkit judge route")
	testutil.Equal(t, nodes["route"]["when"], "$changes.output.proceed == true")
	testutil.Equal(t, nodes["launch"]["depends_on"], []string{"route"})
	testutil.Contains(t, text(nodes["launch"]["bash"]), "ROUTE=")
}
