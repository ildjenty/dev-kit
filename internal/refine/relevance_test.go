package refine

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestAlwaysRelevantDependencySupersessionAndOneHopUnitCocitation(t *testing.T) {
	ds := []doc{}
	for i := 1; i <= 14; i++ {
		ds = append(ds, doc{"id": fmt.Sprintf("D-%06x", i), "status": "agreed"})
	}
	ds[0]["depends_on"] = []string{"D-000002"}
	ds[1]["depends_on"] = []string{"D-000003"}
	ds[3]["supersedes"] = []string{"D-000005"}
	ds[4]["supersedes"] = []string{"D-000006"}
	ds[4]["status"], ds[5]["status"] = "superseded", "withdrawn"
	data := decisions.Data{Decisions: ds, Units: []doc{
		{"state": "open", "decisions": []string{"D-000001", "D-000007"}, "body": "Also D-000008"},
		{"state": "done", "decisions": []string{"D-000001", "D-00000d"}},
		{"state": "open", "decisions": []string{"D-00000e"}},
	}, Documents: []decisions.Document{{Decisions: []string{"D-000007", "D-000009"}}, {Decisions: []string{"D-00000e"}}}}
	input := doc{"topic": doc{"depends_on": []string{"D-000001"}, "body": "Cites D-000004"},
		"pending_proposal": doc{"content": doc{"decision_ids": []string{"D-00000a"}, "decisions": []doc{{"supersedes": []string{"D-00000b"}}}}},
		"previous":         doc{"content": doc{"mapping": doc{"N1": "D-00000c"}}}}
	selected := alwaysRelevant(input, data)
	// D-000008 is only mentioned in a unit body and D-000009 only shares a document: neither is included.
	for i := 1; i <= 14; i++ {
		testutil.Equal(t, selected[fmt.Sprintf("D-%06x", i)], i <= 12 && i != 8 && i != 9)
	}
	data.Decisions[2]["depends_on"] = []string{"D-000001"}
	testutil.Equal(t, len(alwaysRelevant(input, data)), 10)
}

func TestRelevanceCLIMaterialAndAlwaysSet(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			r := setup(t, doc{"provider": provider})
			for _, id := range []string{"D-000001", "D-000002", "D-000003", "D-000004"} {
				r.decision(id, nil)
			}
			r.decision("D-000005", doc{"status": "superseded"})
			r.decision("D-000006", doc{"status": "withdrawn"})
			r.decision("D-000007", doc{"status": "proposed"})
			r.commit()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				w.WriteHeader(402)
			}))
			defer server.Close()
			r.put(".devkit/topics/T-abcdef.md", topicText(doc{"id": "T-abcdef", "title": "Topic title", "body": "D-000001", "status": "open", "depends_on": []string{}}))
			r.commit()
			pack := filepath.Join(r.root, "pack")
			testutil.Put(t, pack, "refine/commands/relevance.md", string(testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/refine/commands/relevance.md"))))
			fake := filepath.Join(r.root, "fake-cli")
			body := "#!/bin/sh\ncat > '" + filepath.Join(r.root, "prompt.txt") + "'\nprintf x >> '" + filepath.Join(r.root, "calls.txt") + "'\n"
			if provider == "codex" {
				body += "printf '%s' '{\"relevant_ids\":[\"D-000002\"]}' > \"$9\"\n"
			} else {
				body += "printf '%s' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"structured_output\":{\"relevant_ids\":[\"D-000002\"]}}'\n"
			}
			testutil.OK(t, os.WriteFile(fake, []byte(body), 0700))
			env := envVars(map[string]string{"PACK_DIR": pack, "DEVKIT_CODEX": fake, "DEVKIT_CLAUDE": fake, "TYPESAFE_API_KEY": "secret", "TYPESAFE_API_URL": server.URL})
			env.Stderr = &bytes.Buffer{}
			started := testutil.Must(start(r.ctx, envVars(nil)))
			testutil.Equal(t, started.Stopped, false)
			testutil.Must(prepare(r.ctx, env, r.artifacts, "Owner words", "Session supplement", "Confirmed memo", "T-abcdef"))
			input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
			testutil.Equal(t, len(items(input["decisions"])), 7)
			for _, d := range items(input["decisions"]) {
				testutil.Equal(t, len(d), 6)
				testutil.Equal(t, d["body"], nil)
				testutil.Equal(t, d["path"], ".devkit/decisions/"+str(d["id"])+".md")
			}
			relevant := items(input["relevant_decisions"])
			testutil.Equal(t, len(relevant), 2)
			testutil.Equal(t, relevant[0]["id"], "D-000001")
			testutil.Equal(t, relevant[1]["id"], "D-000002")
			artifact := testutil.Must(readJSON(file(r.artifacts, "relevance.json")))
			testutil.Equal(t, artifact["source"], provider)
			testutil.Equal(t, artifact["reason"], "")
			testutil.Equal(t, artifact["threshold"], nil)
			testutil.Equal(t, find(items(artifact["decisions"]), "D-000002")["source"], provider)
			testutil.Equal(t, find(items(artifact["decisions"]), "D-000003")["included"], false)
			selection := testutil.Must(readJSON(file(r.artifacts, "relevance-input.json")))
			testutil.Equal(t, stringsOf(selection["always_ids"]), []string{"D-000001"})
			testutil.Equal(t, selection["language"], "en")
			testutil.Equal(t, selection["standards"], ".devkit/standards")
			material := obj(selection["material"])
			testutil.Equal(t, obj(material["topic"]), doc{"title": "Topic title", "body": "D-000001"})
			testutil.Equal(t, material["memo"], "Confirmed memo")
			testutil.Equal(t, items(material["requests"])[0]["owner_words"], "Owner words")
			testutil.Equal(t, items(material["requests"])[0]["supplement"], "Session supplement")
			testutil.Equal(t, len(items(selection["decisions"])), 3)
			for i, d := range items(selection["decisions"]) {
				testutil.Equal(t, d, doc{"id": fmt.Sprintf("D-%06x", i+2), "title": "An existing decision", "body": "Existing decision body", "path": fmt.Sprintf(".devkit/decisions/D-%06x.md", i+2)})
			}
			testutil.Equal(t, find(items(artifact["decisions"]), "D-000001")["source"], "always")
			testutil.Equal(t, calls.Load(), int32(0))
			testutil.Equal(t, readText(t, filepath.Join(r.root, "calls.txt")), "x")
			input["decisions"] = append(items(input["decisions"]), doc{"id": "D-ffffff", "status": "agreed"})
			testutil.OK(t, writeJSON(file(r.artifacts, "input.json"), input))
			d := draft()
			items(d["decisions"])[0]["body"] = "Follow D-000004."
			items(d["decisions"])[0]["depends_on"] = []string{"D-000004"}
			r.writeDraft(d)
			testutil.Equal(t, r.errors(d), "")
			items(d["decisions"])[0]["body"] = "Follow D-ffffff."
			testutil.Contains(t, r.errors(d), "missing citation D-ffffff")
			items(d["decisions"])[0]["body"] = "Follow D-000004."
			r.put(".devkit/decisions/D-000004.md", strings.ReplaceAll(readText(t, filepath.Join(r.cwd, ".devkit/decisions/D-000004.md")), "Existing decision body", "Edited body"))
			testutil.Contains(t, r.errors(d), "writer edited decision records directly")
			testutil.Equal(t, calls.Load(), int32(0))
			testutil.Equal(t, readText(t, filepath.Join(r.root, "calls.txt")), "x")
		})
	}
}

func TestRelevanceCLIAndAllAgreed(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, mode := range []string{"no key", "key set", "empty selection", "CLI fails", "CLI malformed", "CLI unknown ID", "CLI non-agreed ID"} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				r := setup(t, doc{"provider": provider})
				r.decision("D-000001", doc{"depends_on": []string{"D-000003"}})
				r.decision("D-000002", nil)
				r.decision("D-000003", doc{"status": "superseded"})
				r.decision("D-000004", doc{"status": "withdrawn"})
				r.put(".devkit/topics/T-abcdef.md", topicText(doc{"id": "T-abcdef", "title": "Topic", "body": "D-000003", "status": "open", "depends_on": []string{}}))
				r.commit()
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					calls.Add(1)
					w.WriteHeader(402)
				}))
				defer server.Close()
				key := "secret"
				if mode == "no key" {
					key = ""
				}
				pack := filepath.Join(r.root, "pack")
				prompt := testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/refine/commands/relevance.md"))
				testutil.Put(t, pack, "refine/commands/relevance.md", string(prompt))
				answer := `{"relevant_ids":["D-000002"]}`
				if mode == "empty selection" {
					answer = `{"relevant_ids":[]}`
				}
				if mode == "CLI malformed" {
					answer = `{"relevant_ids":[],"extra":true}`
				}
				if mode == "CLI non-agreed ID" {
					answer = `{"relevant_ids":["D-000004"]}`
				}
				if mode == "CLI unknown ID" {
					answer = `{"relevant_ids":["D-000002","D-ffffff"]}`
				}
				body := "#!/bin/sh\ncat > '" + filepath.Join(r.root, "prompt.txt") + "'\nprintf x >> '" + filepath.Join(r.root, "calls.txt") + "'\n"
				if provider == "codex" {
					body += "test \"$1 $2 $3 $4\" = 'exec --sandbox read-only --cd' || exit 2\nprintf '%s' '" + answer + "' > \"$9\"\n"
				} else {
					body += "test \"$4 $5 $6 $7 $8 $9\" = '--allowedTools Read,Grep,Glob --tools Read,Grep,Glob --permission-mode dontAsk' || exit 2\nprintf '%s' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"structured_output\":" + answer + "}'\n"
				}
				if mode == "CLI fails" {
					body += "exit 1\n"
				}
				fake := filepath.Join(r.root, "fake-cli")
				testutil.OK(t, os.WriteFile(fake, []byte(body), 0700))
				env := envVars(map[string]string{"PACK_DIR": pack, "DEVKIT_CODEX": fake, "DEVKIT_CLAUDE": fake, "TYPESAFE_API_KEY": key, "TYPESAFE_API_URL": server.URL})
				var stderr bytes.Buffer
				env.Stderr = &stderr
				env.Cwd = r.cwd
				started := testutil.Must(start(r.ctx, envVars(nil)))
				testutil.Equal(t, started.Stopped, false)
				testutil.Must(prepare(r.ctx, env, r.artifacts, "Owner words", "Supplement", "Memo", "T-abcdef"))
				input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
				artifact := testutil.Must(readJSON(file(r.artifacts, "relevance.json")))
				for _, affected := range items(obj(input["affected"])["decisions"]) {
					testutil.Equal(t, len(affected), 6)
					testutil.Equal(t, affected["body"], nil)
				}
				want, source := 2, provider
				if mode == "empty selection" {
					want = 1
				}
				if strings.HasPrefix(mode, "CLI ") {
					want, source = 3, "all"
					testutil.Contains(t, str(artifact["reason"]), provider+" unavailable")
					for _, id := range []string{"D-000001", "D-000002"} {
						testutil.Equal(t, find(items(input["relevant_decisions"]), id)["status"], "agreed")
					}
				}
				testutil.Equal(t, artifact["source"], source)
				testutil.Equal(t, find(items(artifact["decisions"]), "D-000003")["source"], "always")
				testutil.Equal(t, find(items(artifact["decisions"]), "D-000004")["included"], false)
				testutil.Equal(t, len(items(input["relevant_decisions"])), want)
				if want == 2 {
					testutil.Equal(t, items(input["relevant_decisions"])[0]["id"], "D-000002")
				}
				testutil.Equal(t, readText(t, filepath.Join(r.root, "calls.txt")), "x")
				testutil.Contains(t, readText(t, filepath.Join(r.root, "prompt.txt")), file(r.artifacts, "relevance-input.json"))
				selection := testutil.Must(readJSON(file(r.artifacts, "relevance-input.json")))
				testutil.Equal(t, stringsOf(selection["always_ids"]), []string{"D-000003"})
				testutil.Equal(t, len(items(selection["decisions"])), 2)
				for _, d := range items(selection["decisions"]) {
					testutil.Equal(t, len(d), 4)
					testutil.Equal(t, d["body"], "Existing decision body")
				}
				testutil.Equal(t, calls.Load(), int32(0))
				for _, text := range []string{stderr.String(), readText(t, file(r.artifacts, "relevance.json"))} {
					if strings.Contains(text, "secret") {
						t.Fatal("API key leaked")
					}
				}
			})
		}
	}
}

func TestRelevanceWithoutCandidatesSkipsCLI(t *testing.T) {
	for _, ds := range [][]doc{{}, {{"id": "D-000001", "status": "agreed"}, {"id": "D-000002", "status": "withdrawn"}}} {
		artifacts := t.TempDir()
		input := doc{"topic": doc{"body": "D-000001"}}
		relevant := testutil.Must(relevance(envVars(nil), t.TempDir(), artifacts, input, decisions.Data{Decisions: ds}, config.Defaults("")))
		want := []doc{}
		if len(ds) > 0 {
			want = append(want, ds[0])
		}
		testutil.Equal(t, relevant, want)
		artifact := testutil.Must(readJSON(file(artifacts, "relevance.json")))
		testutil.Equal(t, artifact["source"], "always")
		testutil.Equal(t, artifact["reason"], "")
		selection := testutil.Must(readJSON(file(artifacts, "relevance-input.json")))
		testutil.Equal(t, len(items(selection["decisions"])), 0)
	}
}
