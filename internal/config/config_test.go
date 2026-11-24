package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestGlobs(t *testing.T) {
	for _, tc := range []struct {
		glob, path string
		want       bool
	}{{"**/package.json", "package.json", true}, {"**/package.json", "packages/a/package.json", true}, {"**/package.json", ".hidden/package.json", true}, {"src/**/a?.js", "src/a1.js", true}, {"src/**/a?.js", "src/deep/a2.js", true}, {"src/*", "src/deep/a.js", false}, {"src/?", "src/a/b", false}, {"a?.js", "ab.js", true}, {"a?.js", "abb.js", false}, {"file[1].json", "file[1].json", true}, {"file[1].json", "file1.json", false}, {".archon/**", ".archon/config.yaml", true}, {"*.js", "./a.js", true}} {
		t.Run(tc.glob+"_"+tc.path, func(t *testing.T) { testutil.Equal(t, Matches([]string{tc.glob}, tc.path), tc.want) })
	}
	testutil.Equal(t, Matches(nil, "a.js"), false)
}
func TestCheckSelection(t *testing.T) {
	c := Config{Checks: []Check{{Name: "always"}, {Name: "source", Paths: []string{"src/**"}}, {Name: "old", Paths: []string{"old/**"}}, {Name: "never", Paths: []string{}}, {Name: "docs", Paths: []string{"docs/**"}}}}
	names := func(changes gitx.Changes) []string {
		result := []string{}
		for _, check := range ChecksFor(c, changes) {
			result = append(result, check.Name)
		}
		return result
	}
	testutil.Equal(t, names(gitx.Changes{}), []string{"always"})
	testutil.Equal(t, names(gitx.Changes{Files: []gitx.File{{Status: "R100", OldPath: "old/a.js", Path: "src/a.js"}}}), []string{"always", "source", "old"})
	testutil.Equal(t, names(gitx.Changes{Files: []gitx.File{{Path: "docs/a.md"}}}), []string{"always", "docs"})
}
func TestConfigDefaultsAndCheckout(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Equal(t, Defaults("").Language, "en")
	testutil.Equal(t, testutil.Must(Parse([]byte(`{}`))).Language, "en")
	testutil.Equal(t, testutil.Must(Load(repo)), Defaults(""))
	testutil.Put(t, repo, ".devkit/config.json", `{"standards":"policy","language":"en","limits":{"restarts":1},"qualities":{"ui":{"paths":[]}}}`)
	testutil.Git(t, repo, "add", ".devkit")
	testutil.Git(t, repo, "commit", "-qm", "settings")
	testutil.Put(t, repo, ".devkit/config.json", `{"standards":"policy","language":"fr","limits":{"restarts":1},"qualities":{"ui":{"paths":[]}}}`)
	testutil.OK(t, os.Mkdir(filepath.Join(repo, "nested"), 0755))
	c := testutil.Must(Load(filepath.Join(repo, "nested")))
	testutil.Equal(t, c.Language, "fr")
	testutil.Equal(t, c.Standards, "policy")
	testutil.Equal(t, c.Limits, Limits{1, 3, 3})
	testutil.Equal(t, c.Qualities["ui"].Paths, []string{})
	has := func(path string) bool {
		for _, rule := range c.DecisionPaths {
			for _, p := range rule.Paths {
				if p == path {
					return true
				}
			}
		}
		return false
	}
	testutil.Equal(t, has("policy/**"), true)
	testutil.Equal(t, has(".devkit/standards/**"), false)
	var stdout, stderr bytes.Buffer
	r := cli.New("test", Commands())
	testutil.Equal(t, r.Execute([]string{"config", filepath.Join(repo, "nested")}, cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr}), 0)
	var actual Config
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &actual))
	testutil.Equal(t, actual, c)
}
func TestStandardsDefaults(t *testing.T) {
	c := Defaults("")
	testutil.Equal(t, c.Standards, ".devkit/standards")
	testutil.Equal(t, c.Docs, "docs")
	testutil.Equal(t, Matches(c.Qualities["structure"].Paths, ".devkit/standards/authority.md"), true)
	changes := DecisionChanges(c, gitx.Changes{Files: []gitx.File{{Path: ".devkit/standards/authority.md", Status: "M"}}})
	testutil.Equal(t, changes, []DecisionChange{{Path: ".devkit/standards/authority.md", Status: "M", Kind: "file", Quality: "structure", Ask: true}})
}
func TestConfigRejectsRenamedKey(t *testing.T) {
	for _, raw := range []string{`{"norms":"policy"}`, `{"norms":null}`, `{"norms":"policy","standards":"policy"}`} {
		t.Run(raw, func(t *testing.T) {
			_, err := Parse([]byte(raw))
			testutil.Fails(t, err, "norms was renamed to standards")
		})
	}
}
func TestConfigRejectsInvalidShapes(t *testing.T) {
	repo := testutil.Repo(t)
	for _, raw := range []string{`null`, `[]`, `{"language":1}`, `{"standards":"../policy"}`, `{"extra":true}`, `{"checks":{}}`, `{"checks":[{"name":"a","command":"true"}]}`, `{"checks":[{"name":"a","command":"true","timeout_seconds":1,"paths":"src/**"}]}`, `{"checks":[{"name":"a","command":"true","timeout_seconds":1},{"name":"a","command":"true","timeout_seconds":1}]}`, `{"limits":null}`, `{"limits":{"restarts":-1}}`, `{"skeleton_check":["missing"]}`, `{"qualities":{"unknown":{"paths":[]}}}`, `{"qualities":{"ui":{"paths":1}}}`, `{"decision_paths":[{"paths":[],"quality":"unknown","ask":false}]}`} {
		t.Run(raw, func(t *testing.T) {
			testutil.Put(t, repo, ".devkit/config.json", raw)
			_, err := Load(repo)
			testutil.Fails(t, err, "invalid .devkit/config.json")
		})
	}
}
func TestStructuralChanges(t *testing.T) {
	c := Defaults("")
	testutil.Equal(t, QualitiesFor(c, gitx.Changes{Files: []gitx.File{{Path: "readme.md"}}}), []string{})
	for _, status := range []string{"A", "D", "R100"} {
		testutil.Equal(t, QualitiesFor(c, gitx.Changes{Files: []gitx.File{{Status: status, Path: "arbitrary/file.txt"}}}), []string{"structure"})
	}
	changes := gitx.Changes{Files: []gitx.File{{Status: "R100", OldPath: "migrations/001.sql", Path: "archive/001.sql"}}, AddedDirectories: []string{"archive"}, RemovedDirectories: []string{"migrations"}}
	testutil.Equal(t, DecisionChanges(c, changes), []DecisionChange{{"archive/001.sql", "migrations/001.sql", "R100", "file", "data", true}, {Path: "archive", Status: "A", Kind: "directory", Quality: "structure"}, {Path: "migrations", Status: "D", Kind: "directory", Quality: "structure"}})
}

func TestUnicodeGlobLiterals(t *testing.T) {
	testutil.Equal(t, Matches([]string{"docs/設計/**"}, "docs/設計/方針.md"), true)
	testutil.Equal(t, Matches([]string{"docs/設計/**"}, "docs/別/方針.md"), false)
}

func TestConfigIsAnOwnerDecisionAndGeneratedRecordsDoNotInventStructureDecisions(t *testing.T) {
	c := Defaults("")
	changes := gitx.Changes{Files: []gitx.File{{Path: ".devkit/config.json", Status: "M"}}}
	decisions := DecisionChanges(c, changes)
	testutil.Equal(t, len(decisions), 1)
	testutil.Equal(t, decisions[0].Ask, true)
	changes = gitx.Changes{Files: []gitx.File{{Path: ".devkit/decisions/D-abcdef.md", Status: "A"}}, AddedDirectories: []string{".devkit", ".devkit/decisions"}}
	testutil.Equal(t, len(DecisionChanges(c, changes)), 0)
}

func TestF3BacklogPathsRemainDecisionsUnderBroadGlobs(t *testing.T) {
	c := Defaults("")
	c.DecisionPaths = []DecisionRule{{Paths: []string{"**"}, Quality: "structure", Ask: true}}
	changed := gitx.Changes{Files: []gitx.File{{Path: ".devkit/backlog/feature%2Fx.json", Status: "A"}}, AddedDirectories: []string{".devkit", ".devkit/backlog"}}
	testutil.Equal(t, len(DecisionChanges(c, changed)), 3)
	changed.Files = append(changed.Files, gitx.File{Path: ".devkit/config.json", Status: "M"})
	if len(DecisionChanges(c, changed)) == 0 {
		t.Fatal("non-backlog decisions lost")
	}
}

func TestSkillNamesDefaultsMergeAndValidation(t *testing.T) {
	testutil.Equal(t, Defaults("").Skills, map[string]string{"devkit-runs": "devkit-runs", "devkit-discuss": "devkit-discuss", "devkit-optimize": "devkit-optimize"})
	c := testutil.Must(Parse([]byte(`{"skills":{"devkit-runs":"run-42"}}`)))
	testutil.Equal(t, c.Skills, map[string]string{"devkit-runs": "run-42", "devkit-discuss": "devkit-discuss", "devkit-optimize": "devkit-optimize"})
	for _, raw := range []string{`{"skills":null}`, `{"skills":[]}`, `{"skills":{"extra":"name"}}`, `{"skills":{"devkit-runs":null}}`, `{"skills":{"devkit-runs":42}}`, `{"skills":{"devkit-runs":""}}`, `{"skills":{"devkit-runs":"RUN"}}`, `{"skills":{"devkit-runs":"run_name"}}`, `{"skills":{"devkit-runs":"a/b"}}`, `{"skills":{"devkit-runs":"a b"}}`, `{"skills":{"devkit-runs":"名前"}}`, `{"skills":{"devkit-runs":"devkit-discuss"}}`} {
		t.Run(raw, func(t *testing.T) {
			_, err := Parse([]byte(raw))
			testutil.Fails(t, err, "invalid .devkit/config.json")
		})
	}
}

func TestUnitDoneExclusionAlsoAppliesToDecisionDetection(t *testing.T) {
	repo, records := testutil.Repo(t), t.TempDir()
	testutil.Put(t, records, "request/1.json", `{"unit":"U-abcdef"}`)
	g := testutil.Must(gitx.New(repo, "main", records))
	c := Defaults("")
	c.DecisionPaths = []DecisionRule{{Paths: []string{"**"}, Quality: "structure", Ask: true}}
	testutil.Put(t, repo, ".devkit/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"feature/x"}`)
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "done")
	testutil.Equal(t, DecisionChanges(c, testutil.Must(g.Changes("", "HEAD"))), []DecisionChange{})
	testutil.Put(t, repo, ".devkit/units/U-fedcba.md", "another unit")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "another unit")
	changed := DecisionChanges(c, testutil.Must(g.Changes("", "HEAD")))
	if len(changed) == 0 {
		t.Fatal("other unit file was excluded")
	}
	testutil.Put(t, repo, ".devkit/units/U-abcdef.done.json", `{}`)
	testutil.Git(t, repo, "commit", "-qam", "invalid marker")
	changed = DecisionChanges(c, testutil.Must(g.Changes("", "HEAD")))
	found := false
	for _, item := range changed {
		if item.Path == ".devkit/units/U-abcdef.done.json" {
			found = true
		}
	}
	testutil.Equal(t, found, true)
}

func TestRefineRelevanceThreshold(t *testing.T) {
	testutil.Equal(t, testutil.Must(Parse([]byte(`{}`))).Refine.RelevanceThreshold, .2)
	testutil.Equal(t, testutil.Must(Parse([]byte(`{"refine":{}}`))).Refine.RelevanceThreshold, .2)
	for _, value := range []string{"0", "0.7", "1"} {
		c, err := Parse([]byte(`{"refine":{"relevance_threshold":` + value + `}}`))
		testutil.OK(t, err)
		var want float64
		testutil.OK(t, json.Unmarshal([]byte(value), &want))
		testutil.Equal(t, c.Refine.RelevanceThreshold, want)
	}
	for _, raw := range []string{`{"refine":null}`, `{"refine":[]}`, `{"refine":{"extra":1}}`, `{"refine":{"relevance_threshold":null}}`, `{"refine":{"relevance_threshold":"0.2"}}`, `{"refine":{"relevance_threshold":true}}`, `{"refine":{"relevance_threshold":-0.1}}`, `{"refine":{"relevance_threshold":1.1}}`} {
		_, err := Parse([]byte(raw))
		testutil.Fails(t, err, "invalid .devkit/config.json")
	}
}
