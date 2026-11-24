package refine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReproposalKeepsDependenciesAcrossVersions(t *testing.T) {
	r := setup(t, nil)
	r.git("checkout", "-qb", "refine/shift-management")
	r.ctx.Branch = "refine/shift-management"
	r.begin()
	d := testutil.Must(readJSON("testdata/reproposal/draft.json"))
	mapping := testutil.Must(readJSON("testdata/reproposal/mapping.json"))
	ids, paths := []string{}, []string{}
	previousMapping := doc{}
	for _, item := range items(d["decisions"]) {
		key := str(item["key"])
		if key == "N19" {
			continue
		}
		id := str(mapping[key])
		path := ".devkit/decisions/" + id + ".md"
		r.put(path, readText(t, filepath.Join("testdata/reproposal/decisions", id+".md")))
		ids, paths = append(ids, id), append(paths, path)
		previousMapping[key] = id
	}
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Must(proposal.Commit(r.ctx, paths, "Propose version 1", doc{"mapping": previousMapping, "decision_ids": ids, "base_commit": input["proposal_base"]}))
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	r.writeDraft(d)
	r.artifact("approval.txt", "N2 N3 N5 N7 N8 N9 N10 N12 N19 approve=2 or answers")
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts))["valid"], true)
	testutil.Equal(t, obj(obj(r.latest("proposal")["content"])["mapping"]), mapping)
	testutil.Equal(t, find(r.data().Decisions, str(mapping["N10"]))["depends_on"], []any{"D-3ce450", "D-3b6399", "D-5e6493"})
	expected := testutil.Must(decisions.Parse(readText(t, "testdata/reproposal/decisions/D-f0c65e.md")))
	testutil.Equal(t, find(r.data().Decisions, str(mapping["N19"]))["depends_on"], expected["depends_on"])
	out := testutil.Must(round(r.ctx, r.artifacts, judgment()))
	if out["valid"] != true {
		t.Fatal(testutil.Must(readJSON(file(r.artifacts, "rounds/latest.json")))["errors"])
	}
	testutil.Equal(t, r.finish()["outcome"], "question")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}

func TestReproposalChecksFinalDependencySet(t *testing.T) {
	for _, tc := range []struct {
		name, dependency, want string
	}{
		{"withdrawn key", "N2", "missing dependency N2"},
		{"withdrawn id", "", "missing dependency"},
		{"never existed", "D-ffffff", "missing dependency D-ffffff"},
		{"agreed outside draft", "D-abcdef", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t, nil)
			r.decision("D-abcdef", nil)
			r.commit()
			agreed := readText(t, filepath.Join(r.cwd, ".devkit/decisions/D-abcdef.md"))
			r.begin()
			d := draft()
			second := clone(items(d["decisions"])[0])
			second["key"] = "N2"
			d["decisions"] = append(items(d["decisions"]), second)
			r.writeDraft(d)
			r.artifact("approval.txt", "N1 N2 UN1 approve=1 or answers")
			testutil.Equal(t, r.round(judgment())["valid"], true)
			mapping := obj(obj(r.latest("proposal")["content"])["mapping"])
			dependency := tc.dependency
			if dependency == "" {
				dependency = str(mapping["N2"])
			}
			d = draft()
			items(d["decisions"])[0]["depends_on"] = []string{dependency}
			r.writeDraft(d)
			r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
			head := r.git("rev-parse", "HEAD")
			index := testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".git/index")))
			before := r.data()
			if tc.want != "" {
				testutil.Contains(t, r.errors(d), tc.want)
				input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
				_, err := materialize(r.ctx, d, input)
				testutil.Fails(t, err, "missing dependency")
				testutil.Equal(t, r.data(), before)
				testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
				testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".git/index"))), index)
			} else {
				testutil.Equal(t, r.round(judgment())["valid"], true)
				testutil.Equal(t, r.finish()["outcome"], "question")
				testutil.Equal(t, find(r.data().Decisions, str(mapping["N1"]))["depends_on"], []any{dependency})
				testutil.Equal(t, strings.Contains(r.git("ls-files", ".devkit/decisions"), str(mapping["N2"])), false)
			}
			testutil.Equal(t, readText(t, filepath.Join(r.cwd, ".devkit/decisions/D-abcdef.md")), agreed)
			testutil.Equal(t, r.git("status", "--porcelain"), "")
		})
	}
}
