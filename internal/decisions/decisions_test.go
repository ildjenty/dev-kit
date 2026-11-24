package decisions

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func decision(t *testing.T, repo, id string, fields Item) {
	doc := Item{"id": id, "title": "Decision " + id, "status": "agreed", "decided_by": "owner", "depends_on": []string{}, "supersedes": []string{}, "request": "refine/topic"}
	for k, v := range fields {
		doc[k] = v
	}
	keys := []string{"id", "title", "status", "decided_by", "depends_on", "supersedes", "request"}
	lines := []string{}
	for _, key := range keys {
		lines = append(lines, key+": "+string(testutil.Must(json.Marshal(doc[key]))))
	}
	testutil.Put(t, repo, ".devkit/decisions/"+id+".md", "---\n"+strings.Join(lines, "\n")+"\n---\nBackground and candidates.\n")
}
func unit(t *testing.T, repo, id string, decisions, dependencies []string) {
	testutil.Put(t, repo, ".devkit/units/"+id+".md", fmt.Sprintf("---\nid: %s\ntitle: Unit\ndecisions: %s\ndepends_on: %s\n---\nA goal.\n\n- A completion condition.\n", id, testutil.Must(json.Marshal(decisions)), testutil.Must(json.Marshal(dependencies))))
}
func TestParseFrontMatter(t *testing.T) {
	doc := testutil.Must(Parse("---\nid: D-a00001\ntitle: \"A, B\"\ndepends_on:\n  - D-a00002\n  - 'D-a00003'\nsupersedes: [D-a00004, \"D-a00005\"]\n---\nBody"))
	testutil.Equal(t, doc, Item{"id": "D-a00001", "title": "A, B", "depends_on": []string{"D-a00002", "D-a00003"}, "supersedes": []string{"D-a00004", "D-a00005"}, "body": "Body"})
	_, err := Parse("no front matter")
	testutil.Fails(t, err, "front matter")
}
func TestListBodiesAndProseCitations(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-a00001", nil)
	unit(t, repo, "U-a00001", []string{"D-a00001"}, []string{})
	testutil.Put(t, repo, "docs/intro.md", "# Intro\nD-a00001 and D-a00001\n")
	testutil.Put(t, repo, ".devkit/standards/designing.md", "# A standard, not a product document\n")
	data := testutil.Must(List(repo))
	testutil.Equal(t, data.Decisions[0]["body"], "Background and candidates.")
	testutil.Equal(t, data.Documents, []Document{{"docs/intro.md", []string{"Intro"}, []string{"D-a00001"}}})
	testutil.Equal(t, Check(data), []string{})
	testutil.Equal(t, testutil.Must(NextID("decision", "feature/a", "choice", data)), "D-"+fmt.Sprintf("%x", sha256.Sum256([]byte("feature/a\x00choice")))[:6])
	testutil.Equal(t, testutil.Must(NextID("unit", "feature/a", "unit", data)), "U-"+fmt.Sprintf("%x", sha256.Sum256([]byte("feature/a\x00unit")))[:6])
}
func TestListExcludesStandardsInsideDocs(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"standards":"docs/standards"}`)
	testutil.Put(t, repo, "docs/intro.md", "# Intro\n")
	testutil.Put(t, repo, "docs/standards/designing.md", "# Standards\nD-ffffff\n")
	testutil.Put(t, repo, "docs/standards/nested/criteria.md", "# Criteria\nD-ffffff\n")
	testutil.Put(t, repo, "docs/standards-product.md", "# Product\n")
	data := testutil.Must(List(repo))
	testutil.Equal(t, data.Documents, []Document{{"docs/intro.md", []string{"Intro"}, []string{}}, {"docs/standards-product.md", []string{"Product"}, []string{}}})
	testutil.Equal(t, Check(data), []string{})
}
func TestMissingReferencesDuplicatesAndCycles(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-a00001", Item{"depends_on": []string{"D-a00002"}})
	decision(t, repo, "D-a00002", Item{"depends_on": []string{"D-a00001"}, "supersedes": []string{"D-a09999"}})
	unit(t, repo, "U-a00001", []string{"D-a09999"}, []string{"U-a00001"})
	testutil.Put(t, repo, "docs/a.md", "D-a08888")
	testutil.Put(t, repo, ".devkit/decisions/nested/D-a00001.md", "---\nid: D-a00001\ntitle: Duplicate\nstatus: agreed\ndecided_by: owner\ndepends_on: []\nsupersedes: []\nrequest: refine/a\n---\nBody")
	errors := strings.Join(Check(testutil.Must(List(repo))), "\n")
	for _, part := range []string{"duplicate id D-a00001", "cycle", "D-a09999", "D-a08888"} {
		testutil.Contains(t, errors, part)
	}
}
func TestSupersededSuccessorsAndAffected(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-a00001", Item{"status": "superseded"})
	testutil.Contains(t, strings.Join(Check(testutil.Must(List(repo))), "\n"), "without a successor")
	decision(t, repo, "D-a00002", Item{"supersedes": []string{"D-a00001"}})
	decision(t, repo, "D-a00003", Item{"depends_on": []string{"D-a00001"}})
	decision(t, repo, "D-a00004", Item{"depends_on": []string{"D-a00003"}})
	unit(t, repo, "U-a00001", []string{"D-a00001"}, []string{})
	unit(t, repo, "U-a00002", []string{"D-a00004"}, []string{"U-a00001"})
	testutil.Put(t, repo, "docs/topic.md", "D-a00001")
	data := testutil.Must(List(repo))
	testutil.Equal(t, Check(data), []string{})
	impact := Affected(data)
	ids := func(items []Item) []string {
		result := []string{}
		for _, item := range items {
			result = append(result, text(item["id"]))
		}
		return result
	}
	testutil.Equal(t, ids(impact.Decisions), []string{"D-a00003", "D-a00004"})
	testutil.Equal(t, impact.Documents[0].Path, "docs/topic.md")
	testutil.Equal(t, ids(impact.Units), []string{"U-a00001", "U-a00002"})
	dependents := Dependents("D-a00001", data)
	slices.Sort(dependents)
	testutil.Equal(t, dependents, []string{"D-a00003", "D-a00004", "U-a00001", "U-a00002"})
}
func TestConfiguredFoldersAndEmptyUnitDecisions(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"docs":"notes","decisions":"choices","units":"work"}`)
	testutil.Put(t, repo, "work/U-a00001.md", "---\nid: U-a00001\ntitle: Unit\ndecisions: []\ndepends_on: []\n---\nGoal\n- Done")
	testutil.Put(t, repo, "notes/a.md", "D-a00001")
	errors := strings.Join(Check(testutil.Must(List(repo))), "\n")
	testutil.Equal(t, errors, "notes/a.md: missing citation D-a00001")
	testutil.Contains(t, errors, "missing citation D-a00001")
}
func TestCheckCLIExitCodeAndJSON(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "docs/a.md", "D-a09999")
	var out, stderr bytes.Buffer
	r := cli.New("test", Commands())
	testutil.Equal(t, r.Execute([]string{"decisions", "check"}, cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr}), 1)
	var errors []string
	testutil.OK(t, json.Unmarshal(out.Bytes(), &errors))
	testutil.Equal(t, errors, []string{"docs/a.md: missing citation D-a09999"})
	testutil.Equal(t, stderr.String(), "")
}

func TestInvalidDecisionModesKeepExitCodeOne(t *testing.T) {
	repo := testutil.Repo(t)
	r := cli.New("test", Commands())
	for _, args := range [][]string{{"decisions"}, {"decisions", "unknown"}, {"decisions", "dependents"}} {
		var out, stderr bytes.Buffer
		testutil.Equal(t, r.Execute(args, cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr}), 1)
		testutil.Contains(t, stderr.String(), "Usage: devkit decisions")
	}
}

func TestIDsUseBranchAndKeyAndExtendCollisionsForEveryKind(t *testing.T) {
	for _, kind := range []string{"decision", "unit", "topic"} {
		empty := Data{}
		id := testutil.Must(NextID(kind, "branch/a", "key", empty))
		testutil.Equal(t, len(id), 8)
		testutil.Equal(t, testutil.Must(NextID(kind, "branch/a", "key", empty)), id)
		if id == testutil.Must(NextID(kind, "branch/b", "key", empty)) || id == testutil.Must(NextID(kind, "branch/a", "other", empty)) {
			t.Fatal("branch or draft key was ignored")
		}
		occupied := []Item{{"id": id}, {"id": id + "0"}}
		data := Data{Decisions: occupied, Units: occupied, Topics: occupied}
		extended := testutil.Must(NextID(kind, "branch/a", "key", data))
		if len(extended) <= len(id) || !strings.HasPrefix(extended, id) {
			t.Fatal("collision did not extend the hash")
		}
	}
	testutil.Equal(t, Citations("D-abcdef D-1234567 D-abcdef"), []string{"D-abcdef", "D-1234567"})
}

func TestTopicsReadCheckAndPickableTopicsForRefine(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-abcdef", nil)
	decision(t, repo, "D-bcdefa", Item{"status": "proposed"})
	for id, fields := range map[string]string{
		"T-aaaaaa": "status: open\ndepends_on: [D-abcdef]",
		"T-bbbbbb": "status: open\ndepends_on: [D-bcdefa]",
		"T-cccccc": "status: settled\ndepends_on: [D-abcdef]",
		"T-dddddd": "status: open\ndepends_on: []",
	} {
		testutil.Put(t, repo, ".devkit/topics"+"/"+id+".md", "---\nid: "+id+"\ntitle: A topic\n"+fields+"\n---\nWhich scope? A settled answer names the scope.\n")
	}
	data := testutil.Must(List(repo))
	testutil.Equal(t, len(data.Topics), 4)
	testutil.Equal(t, Check(data), []string{})
	topics := PickableTopics(data)
	testutil.Equal(t, len(topics), 2)
	testutil.Equal(t, topics[0]["id"], "T-aaaaaa")
	testutil.Equal(t, topics[1]["id"], "T-dddddd")
	testutil.Equal(t, topics[0]["body"], "Which scope? A settled answer names the scope.")
	testutil.Put(t, repo, ".devkit/topics"+"/T-eeeeee.md", "---\nid: T-eeeeee\ntitle: Broken\nstatus: unknown\ndepends_on: [D-ffffff]\n---\nMissing D-ffffff\n")
	errors := strings.Join(Check(testutil.Must(List(repo))), "\n")
	testutil.Contains(t, errors, "invalid status")
	testutil.Contains(t, errors, "missing decision D-ffffff")
}

func TestAffectedPropagatesThroughMultipleDecisionEdgesToDocumentsAndUnits(t *testing.T) {
	data := Data{Decisions: []Item{
		{"id": "D-aaaaaa", "status": "superseded", "depends_on": []string{}},
		{"id": "D-bbbbbb", "status": "agreed", "depends_on": []string{"D-aaaaaa"}},
		{"id": "D-cccccc", "status": "agreed", "depends_on": []string{"D-bbbbbb"}},
	}, Documents: []Document{{Path: "docs/indirect.md", Decisions: []string{"D-cccccc"}}}, Units: []Item{{"id": "U-aaaaaa", "decisions": []string{"D-cccccc"}}}}
	impact := Affected(data)
	testutil.Equal(t, len(impact.Decisions), 2)
	testutil.Equal(t, len(impact.Documents), 1)
	testutil.Equal(t, len(impact.Units), 1)
	testutil.Equal(t, impact.Documents[0].Path, "docs/indirect.md")
	testutil.Equal(t, impact.Units[0]["id"], "U-aaaaaa")
}
