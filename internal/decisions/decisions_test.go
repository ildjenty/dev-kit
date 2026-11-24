package decisions

import (
	"bytes"
	"encoding/json"
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
func TestParseFrontMatter(t *testing.T) {
	doc := testutil.Must(Parse("---\nid: D-a00001\ntitle: \"A, B\"\ndepends_on:\n  - D-a00002\n  - 'D-a00003'\nsupersedes: [D-a00004, \"D-a00005\"]\n---\nBody"))
	testutil.Equal(t, doc, Item{"id": "D-a00001", "title": "A, B", "depends_on": []string{"D-a00002", "D-a00003"}, "supersedes": []string{"D-a00004", "D-a00005"}, "body": "Body"})
	_, err := Parse("no front matter")
	testutil.Fails(t, err, "front matter")
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

func TestListReadsOnlyDecisionLogAndIgnoresObsoleteFolders(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-abcdef", nil)
	for _, path := range []string{".devkit/units/U-abcdef.md", ".devkit/topics/T-abcdef.md", "docs/a.md"} {
		testutil.Put(t, repo, path, "Malformed old record D-ffffff")
	}
	data := testutil.Must(List(repo))
	testutil.Equal(t, len(data.Decisions), 1)
	testutil.Equal(t, data.Decisions[0]["body"], "Background and candidates.")
	testutil.Equal(t, Check(data), []string{})
}
func TestDecisionDependenciesSupersessionAndCollision(t *testing.T) {
	repo := testutil.Repo(t)
	decision(t, repo, "D-aaaaaa", Item{"status": "superseded"})
	decision(t, repo, "D-bbbbbb", Item{"supersedes": []string{"D-aaaaaa"}})
	decision(t, repo, "D-cccccc", Item{"depends_on": []string{"D-aaaaaa"}})
	decision(t, repo, "D-dddddd", Item{"depends_on": []string{"D-cccccc"}})
	data := testutil.Must(List(repo))
	testutil.Equal(t, Check(data), []string{})
	testutil.Equal(t, len(Affected(data).Decisions), 2)
	testutil.Equal(t, Dependents("D-aaaaaa", data), []string{"D-cccccc", "D-dddddd"})
	first := testutil.Must(NextID("decision", "refine/topic", "N1", data))
	data.Decisions = append(data.Decisions, Item{"id": first})
	second := testutil.Must(NextID("decision", "refine/topic", "N1", data))
	testutil.Equal(t, strings.HasPrefix(second, first), true)
	testutil.Equal(t, len(second), len(first)+1)
	decision(t, repo, "D-bbbbbb", Item{"depends_on": []string{"D-dddddd"}})
	decision(t, repo, "D-cccccc", Item{"depends_on": []string{"D-bbbbbb"}})
	testutil.Contains(t, strings.Join(Check(testutil.Must(List(repo))), "\n"), "cycle")
}
