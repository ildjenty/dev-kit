package judge

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestLocalityIncludesUnplannedPathsFolderCountsAndAllAncestors(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "src/area/deep/a.ext", "old\n")
	testutil.Put(t, repo, "other/part/b.ext", "old\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "files")
	before := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, "src/area/deep/a.ext", "changed\n")
	testutil.Git(t, repo, "mv", "other/part/b.ext", "src/area/deep/b.ext")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "change")
	g := must(gitx.New(repo, "main"))
	changed := must(g.Changes(before, "HEAD"))
	facts := must(structureFacts(g, before, "HEAD", changed, doc{"decisions": []doc{row("structure", "src/area")}}))
	testutil.Equal(t, facts["outside_plan"], []string{"other/part/b.ext"})
	testutil.Equal(t, facts["top_level_folders"], 2)
	testutil.Equal(t, facts["second_level_folders"], 2)
	folders := map[string]bool{}
	for _, sibling := range docs(facts["siblings"]) {
		folders[text(sibling["folder"])] = true
	}
	for _, folder := range []string{".", "src", "src/area", "src/area/deep", "other", "other/part"} {
		testutil.Equal(t, folders[folder], true)
	}
}

func TestDocumentChangeLaunchesJudgeWithDocumentMaterials(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, "docs/guide.md", "# Export\nOpen Settings.\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, "docs/guide.md", "# Export\nOpen Settings, then choose Export.\n")
	testutil.Git(t, repo, "commit", "-qam", "guide")
	ctx := record.ContextData{Cwd: repo, Dir: t.TempDir(), Branch: "feature/x", RunID: "judge-document", BaseBranch: "main"}
	must(record.Start(ctx.Dir, "judge", record.StartOptions{RunID: ctx.RunID, Commit: testutil.Git(t, repo, "rev-parse", "HEAD"), BaseCommit: base}))
	a := t.TempDir()
	must(prepare(ctx, a, a))
	must(writeChanges(repo, a))
	launch := must(routeChanges(repo, a, doc{"qualities": []doc{}}))
	testutil.Equal(t, launch["launch_document"], true)
	testutil.Equal(t, must(read(filepath.Join(a, "judge/launched.json")))["launched"], []string{"scope", "document"})
	material := must(read(filepath.Join(a, "judge/materials/document.json")))
	testutil.Equal(t, material["criteria"], []string{filepath.Join(a, "criteria/.devkit/standards/review/document.md")})
	testutil.Equal(t, docs(material["files"])[0]["path"], "docs/guide.md")
	testutil.Equal(t, slices.Contains(stringsOf(material["principles"]), filepath.Join(a, "criteria/.devkit/standards/writing.md#2")), true)
	testutil.Equal(t, material["every_quality"], []string{})
}
func TestRouteUnionsPathsPlanRowsAndInferredQualitiesWithFiles(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, "unusual/source", "before\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, "unusual/source", "after\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "change")
	ctx := record.ContextData{Cwd: repo, Dir: record.RecordDir(t.TempDir(), "feature/x"), Branch: "feature/x", RunID: "judge-route", BaseBranch: "main"}
	must(record.Start(ctx.Dir, "judge", record.StartOptions{RunID: ctx.RunID, Commit: testutil.Git(t, repo, "rev-parse", "HEAD"), BaseCommit: base}))
	a := t.TempDir()
	must(prepare(ctx, a, a))
	must(writeChanges(repo, a))
	testutil.OK(t, save(filepath.Join(a, "judge/plan.json"), doc{"decisions": []doc{row("data", "unusual")}}))
	routed := doc{"qualities": []doc{{"quality": "security", "file": "unusual/source", "reason": "Access check changed"}, {"quality": "readability", "file": "unusual/source", "reason": "Source changed"}}}
	result := must(routeChanges(repo, a, routed))
	assertLaunched := func(result doc, want []string) {
		t.Helper()
		manifest := must(read(filepath.Join(a, "judge/launched.json")))
		testutil.Equal(t, len(manifest), 1)
		launched := stringsOf(manifest["launched"])
		testutil.Equal(t, launched, want)
		for _, q := range qualities() {
			testutil.Equal(t, truth(result["launch_"+q]), slices.Contains(launched, q))
		}
	}
	selected := []string{"data", "security", "test", "scope", "readability"}
	assertLaunched(result, selected)
	for _, q := range []string{"security", "test", "readability", "scope", "data"} {
		testutil.Equal(t, result["launch_"+q], true)
	}
	m := must(read(filepath.Join(a, "judge/materials/security.json")))
	testutil.Equal(t, docs(m["files"])[0]["path"], "unusual/source")
	testutil.Equal(t, len(docs(m["routes"])), 1)
	testutil.Equal(t, must(read(filepath.Join(a, "judge/route.json"))), routed)
	for _, source := range []string{"jev", "claude", "codex", "all"} {
		routed["source"] = source
		values := map[string]string{"ARTIFACTS_DIR": a, "ROUTE": string(must(json.Marshal(routed)))}
		var output bytes.Buffer
		env := cli.Env{Cwd: repo, Stdout: &output, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
		testutil.OK(t, cli.New("", Commands()).Run([]string{"judge", "launch"}, env))
		var launch doc
		testutil.OK(t, json.Unmarshal(output.Bytes(), &launch))
		assertLaunched(launch, selected)
		testutil.Equal(t, must(read(filepath.Join(a, "judge/route.json"))), routed)
	}
	var routedOutput bytes.Buffer
	values := map[string]string{"ARTIFACTS_DIR": a, "PACK_DIR": must(filepath.Abs("../../kit/.archon/workflows/dev")), "DEVKIT_CODEX": filepath.Join(a, "not-installed"), "DEVKIT_CLAUDE": filepath.Join(a, "not-installed")}
	env := cli.Env{Cwd: repo, Stdout: &routedOutput, Stderr: &bytes.Buffer{}, LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	testutil.OK(t, cli.New("", Commands()).Run([]string{"judge", "route"}, env))
	var conservative doc
	testutil.OK(t, json.Unmarshal(routedOutput.Bytes(), &conservative))
	testutil.Equal(t, conservative["source"], "all")
	result = must(routeChanges(repo, a, conservative))
	mapping := must(concerns.Load(filepath.Join(a, "criteria/.devkit/standards")))
	assertLaunched(result, mapping.Judged("code"))
	for _, q := range mapping.Judged("code") {
		testutil.Equal(t, result["launch_"+q], true)
	}
	result = must(routeChanges(repo, a, doc{"qualities": []doc{}}))
	assertLaunched(result, []string{"data", "scope"})
	testutil.OK(t, save(filepath.Join(a, "judge/plan.json"), doc{"decisions": []doc{}}))
	result = must(routeChanges(repo, a, doc{"qualities": []doc{}}))
	assertLaunched(result, []string{"scope"})
	testutil.Put(t, repo, ".devkit/config.json", `{"qualities":{"ui":{"paths":["unusual/**"]}}}`)
	result = must(routeChanges(repo, a, doc{"qualities": []doc{}}))
	assertLaunched(result, []string{"ui", "scope"})
	testutil.Put(t, repo, ".devkit/config.json", `{"qualities":{"test":{"paths":["unusual/**"]}}}`)
	result = must(routeChanges(repo, a, doc{"qualities": []doc{}}))
	assertLaunched(result, []string{"test", "scope", "readability"})
	for _, item := range []doc{{"quality": "imaginary", "file": "unusual/source", "reason": "Reason"}, {"quality": "delivery", "file": "unusual/source", "reason": "Plan-only quality"}, {"quality": "security", "file": "unchanged", "reason": "Reason"}, {"quality": "security", "file": "unusual/source", "reason": ""}} {
		_, err := routeChanges(repo, a, doc{"qualities": []doc{item}})
		if err == nil {
			t.Fatal("invalid route accepted", item)
		}
	}
}
func TestTestAndReadabilityPathsLaunchBothForSourceOrTests(t *testing.T) {
	mapping := must(concerns.Load("../../kit/.devkit/standards"))
	c := testConfig()
	c.Qualities["readability"] = config.Quality{Paths: []string{"source/**"}}
	for _, p := range []string{"source/custom", "tests/example.ext"} {
		r := summary(mapping, c, change(p, "M"), nil)
		testutil.Equal(t, r["launch_test"], true)
		testutil.Equal(t, r["launch_readability"], true)
		testutil.Equal(t, r["launch_scope"], true)
	}
}
func TestDeferredCodeFindingsArePreservedWithoutBlockingOrOwnerRouting(t *testing.T) {
	collected := must(gather(doc{"structure": doc{"criteria": []doc{criterion("pass")}, "findings": []doc{finding(doc{"deferred": true, "mechanical": true, "blocking": false, "needs_decision": false})}}}, doc{"launch_structure": true}))
	f := docs(collected[0]["findings"])[0]
	testutil.Equal(t, f["blocking"], false)
	testutil.Equal(t, f["needs_decision"], false)
	judgments, outcome := decide(inputs(doc{"findings": doc{"judges": collected}}))
	testutil.Equal(t, outcome["outcome"], "pass")
	f = docs(judgments[0]["findings"])[0]
	testutil.Equal(t, f["deferred"], true)
	testutil.Equal(t, f["mechanical"], true)
	testutil.Equal(t, f["blocking"], false)
}
