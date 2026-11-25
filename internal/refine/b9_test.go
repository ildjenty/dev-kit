package refine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReproposalAcrossRunsFromTechStackRecords(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "withdrawn", true: "kept"}[keep], func(t *testing.T) {
			r := setup(t, nil)
			source := testutil.Must(readJSON("testdata/b9/input.json"))
			for _, d := range items(source["decisions"]) {
				if d["status"] != "proposed" {
					r.decision(str(d["id"]), d)
				}
			}
			r.commit()
			r.begin()
			paths := []string{}
			for _, d := range items(source["decisions"]) {
				if d["status"] != "proposed" {
					continue
				}
				id := str(d["id"])
				r.decision(id, d)
				paths = append(paths, ".devkit/decisions/"+id+".md")
			}
			pending := obj(obj(source["pending_proposal"])["content"])
			input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
			pending["base_commit"] = input["proposal_base"]
			testutil.Must(proposal.Commit(r.ctx, paths, "Propose recorded tech stack", pending))
			r.ctx.RunID = "another-run"
			r.artifacts = filepath.Join(r.root, "another-run")
			testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
			input = testutil.Must(readJSON(file(r.artifacts, "input.json")))
			d := testutil.Must(readJSON("testdata/b9/draft.json"))
			d["bindings"] = []doc{}
			d["parent"] = draft()["parent"]
			if keep {
				for _, old := range items(obj(pending["draft"])["decisions"]) {
					if has([]string{"N1", "N2", "N3", "N6"}, str(old["key"])) {
						old["decided_by"], old["source"] = "provisional", "derived"
						d["decisions"] = append(items(d["decisions"]), old)
					}
				}
			}
			r.writeDraft(d)
			r.artifact("approval.txt", "Reply approve=2 or answers")
			errors := r.errors(d)
			if keep {
				testutil.Equal(t, errors, "")
				content := testutil.Must(materialize(r.ctx, d, input))
				testutil.Equal(t, content["mapping"], pending["mapping"])
				testutil.Equal(t, find(r.data().Decisions, "D-cad658")["depends_on"], []any{"D-29ac70"})
			} else {
				for _, id := range []string{"D-355f35", "D-deddb0", "D-29ac70", "D-8577f7"} {
					testutil.Contains(t, errors, "withdrawn decision "+id)
				}
			}
			testutil.Equal(t, len(items(input["pending_decisions"])), 7)
		})
	}
}

func TestReproposalRestoresWithdrawnBindingEdits(t *testing.T) {
	for _, mode := range []string{"unstaged", "staged", "committed", "new file", "committed new file"} {
		t.Run(mode, func(t *testing.T) {
			r := setup(t, nil)
			r.begin()
			original := readText(t, filepath.Join(r.cwd, ".devkit/config.json"))
			d := draft()
			withdrawn := ".devkit/config.json"
			if strings.Contains(mode, "new file") {
				withdrawn = "AGENTS.md"
				r.put(withdrawn, "Withdraw this rule\n")
			} else {
				r.put(withdrawn, original+"\n")
			}
			r.put("CLAUDE.md", "Keep this rule\n")
			d["bindings"] = []doc{{"path": withdrawn, "decision": "N1", "reason": "Withdraw later"}, {"path": "CLAUDE.md", "decision": "N1", "reason": "Keep"}}
			r.writeDraft(d)
			if strings.HasPrefix(mode, "committed") {
				testutil.Equal(t, r.round(judgment())["valid"], true)
			} else {
				if mode == "staged" {
					r.git("add", withdrawn)
				}
				r.artifact("check.json", doc{"valid": false, "errors": []string{"machine failure"}})
				testutil.Must(round(r.ctx, r.artifacts, doc{}))
			}
			d["bindings"] = items(d["bindings"])[1:]
			r.writeDraft(d)
			input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
			r.artifact("approval.txt", "Reply approve="+fmt.Sprint(input["next_proposal_version"])+" or answers")
			checked := testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))
			if checked["valid"] != true {
				t.Fatal(checked["errors"])
			}
			testutil.Equal(t, readText(t, filepath.Join(r.cwd, ".devkit/config.json")), original)
			testutil.Equal(t, readText(t, filepath.Join(r.cwd, "CLAUDE.md")), "Keep this rule\n")
			if strings.Contains(mode, "new file") {
				_, err := os.Stat(filepath.Join(r.cwd, withdrawn))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
		})
	}
}

func TestMachineFailureSkipsJudgeValidation(t *testing.T) {
	for _, mode := range []string{"draft", "verification", "draft after proposal", "verification after proposal"} {
		t.Run(mode, func(t *testing.T) {
			r := setup(t, nil)
			r.begin()
			if strings.Contains(mode, "after proposal") {
				r.writeDraft()
				testutil.Equal(t, r.round(judgment())["valid"], true)
			}
			d := draft()
			if strings.HasPrefix(mode, "verification") {
				r.put(".devkit/config.json", `{"language":"en","checks":[{"name":"not-ready","command":"exit 77","timeout_seconds":2}]}`)
				d["bindings"] = []doc{{"path": ".devkit/config.json", "decision": "N1", "reason": "Check readiness"}}
			} else {
				items(d["decisions"])[0]["depends_on"] = []string{"D-ffffff"}
			}
			r.writeDraft(d)
			input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
			r.artifact("approval.txt", "Reply approve="+fmt.Sprint(input["next_proposal_version"])+" or answers")
			checked := testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))
			testutil.Equal(t, checked["valid"], false)
			testutil.Must(round(r.ctx, r.artifacts, doc{}))
			entry := testutil.Must(readJSON(file(r.artifacts, "rounds/latest.json")))
			testutil.Equal(t, entry["errors"], checked["errors"])
			testutil.Equal(t, entry["proposal_version"], nil)
			result := r.finish()
			testutil.Equal(t, strings.Join(stringsOf(result["reasons"]), "\n"), strings.Join(stringsOf(checked["errors"]), "\n"))
		})
	}
}
