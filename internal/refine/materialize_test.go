package refine

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestFinishReplacesDocumentKeysWithoutChangingReservedFiles(t *testing.T) {
	r := setup(t, doc{"standards": "docs/standards", "decisions": "docs/decisions", "units": "docs/units", "topics": "docs/topics"})
	protected := map[string]string{
		"docs/standards/example.md":  "N1 is a neutral example token.\r\n",
		"docs/standards-extra.md":    "N1\n",
		"docs/decisions/D-abcdef.md": decisionText(doc{"id": "D-abcdef", "title": "Existing", "status": "agreed", "decided_by": "owner", "depends_on": []string{}, "supersedes": []string{}, "request": "previous", "body": "N1 is an example."}),
		"docs/units/U-abcdef.md":     "---\nid: U-abcdef\ntitle: Finished\ndecisions: []\ndepends_on: []\n---\nCompleted N1 goal.\n",
		"docs/topics/T-abcdef.md":    topicText(doc{"id": "T-abcdef", "title": "Existing topic", "status": "settled", "depends_on": []string{}, "body": "N1 is an example."}),
	}
	for path, text := range protected {
		r.put(path, text)
	}
	r.commit()
	r.begin()
	r.writeDraft()
	testutil.Equal(t, r.round(judgment())["valid"], true)
	out := r.finish()
	id := stringsOf(out["decision_ids"])[0]
	for path, text := range protected {
		if path == "docs/standards-extra.md" {
			testutil.Equal(t, readText(t, filepath.Join(r.cwd, path)), id+"\n")
			continue
		}
		testutil.Equal(t, readText(t, filepath.Join(r.cwd, path)), text)
	}
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, "docs/refined.md")), "# Refined decisions\n"+id+"\n")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}

func TestReproposalRejectsInvalidFinalSetBeforeChangingFilesOrIndex(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(*fixture, doc, doc)
	}{
		{"reused decision cycle", "depends_on cycle", func(r *fixture, d, mapping doc) {
			items(d["decisions"])[0]["depends_on"] = []string{str(mapping["N1"])}
		}},
		{"reused unit cycle", "depends_on cycle", func(r *fixture, d, mapping doc) {
			items(d["units"])[0]["depends_on"] = []string{str(mapping["UN1"])}
		}},
		{"withdrawn dependency", "missing dependency", func(r *fixture, d, mapping doc) {
			items(d["decisions"])[0]["depends_on"] = []string{str(mapping["N2"])}
		}},
		{"withdrawn supersedes", "invalid supersedes", func(r *fixture, d, mapping doc) {
			items(d["decisions"])[0]["supersedes"] = []string{str(mapping["N2"])}
		}},
		{"reused supersedes", "invalid supersedes", func(r *fixture, d, mapping doc) {
			items(d["decisions"])[0]["supersedes"] = []string{str(mapping["N1"])}
			d["kept_units"] = []doc{{"id": mapping["UN1"], "reason": "Retain the unit with the successor citation"}}
		}},
		{"withdrawn unit citation", "missing decision", func(r *fixture, d, mapping doc) {
			items(d["units"])[0]["decisions"] = []string{str(mapping["N2"])}
		}},
		{"withdrawn body citation", "missing citation", func(r *fixture, d, mapping doc) {
			items(d["decisions"])[0]["body"] = "Cites " + str(mapping["N2"])
		}},
		{"withdrawn document citation", "missing citation", func(r *fixture, d, mapping doc) {
			r.put("docs/topic.md", "Cites "+str(mapping["N2"])+"\n")
		}},
		{"withdrawn topic dependency", "missing decision", func(r *fixture, d, mapping doc) {
			d["topics"] = []doc{{"key": "TN2", "title": "Next topic", "body": "Which channel? Name the channel.", "depends_on": []string{str(mapping["N2"])}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, entry := range []string{"finish", "command", "materialize"} {
				t.Run(entry, func(t *testing.T) {
					r := setup(t, nil)
					r.begin()
					d := draft()
					second := clone(items(d["decisions"])[0])
					second["key"] = "N2"
					d["decisions"] = append(items(d["decisions"]), second)
					r.writeDraft(d)
					r.artifact("approval.txt", "N1 N2 UN1 approve=1 or answers")
					testutil.Equal(t, r.round(judgment())["valid"], true)
					first := r.finish()
					mapping := obj(obj(r.latest("proposal")["content"])["mapping"])
					r.restart(first)
					d = draft()
					tc.mutate(r, d, mapping)
					if entry != "materialize" {
						r.writeDraft(d)
						tc.mutate(r, d, mapping)
						r.artifact("draft.json", d)
						r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
						if len(items(d["topics"])) > 0 {
							r.artifact("approval.txt", "N1 UN1 TN2 approve=2 or answers")
						}
						if r.round(judgment())["valid"] != true {
							t.Fatal(r.errors(d))
						}
					}
					r.put("docs/staged.md", "N1 staged document\n")
					r.git("add", "docs/staged.md")
					before := map[string]string{}
					for _, group := range [][]doc{r.data().Decisions, r.data().Units, r.data().Topics} {
						for _, item := range group {
							p := str(item["path"])
							before[p] = readText(t, filepath.Join(r.cwd, p))
						}
					}
					docs := testutil.Must(docsSnapshot(r.cwd, "docs"))
					head := r.git("rev-parse", "HEAD")
					indexPath := filepath.Join(r.cwd, r.git("rev-parse", "--git-path", "index"))
					index := testutil.Must(os.ReadFile(indexPath))
					var err error
					if entry == "command" {
						var out, errOut bytes.Buffer
						env := envVars(map[string]string{"STATE_DIR": filepath.Join(r.root, "state"), "WORKFLOW_ID": r.ctx.RunID, "ARTIFACTS_DIR": r.artifacts})
						env.Cwd, env.Stdout, env.Stderr = r.cwd, &out, &errOut
						testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"refine", "finish"}, env), 1)
						testutil.Contains(t, errOut.String(), tc.want)
						testutil.Equal(t, out.Len(), 0)
					} else if entry == "finish" {
						_, err = finish(r.ctx, r.artifacts)
					} else {
						input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
						_, err = materialize(r.ctx, r.artifacts, d, input)
					}
					if entry != "command" {
						testutil.Fails(t, err, tc.want)
					}
					testutil.Equal(t, testutil.Must(os.ReadFile(indexPath)), index)
					for p, text := range before {
						testutil.Equal(t, readText(t, filepath.Join(r.cwd, p)), text)
					}
					testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
					testutil.Equal(t, r.git("diff", "--name-only", "--", ".devkit"), "")
					testutil.Equal(t, r.git("ls-files", "--others", "--exclude-standard", "--", ".devkit"), "")
					testutil.Equal(t, testutil.Must(docsSnapshot(r.cwd, "docs")), docs)
				})
			}
		})
	}
}
