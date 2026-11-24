package retro

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	backlogschema "github.com/ildjenty/dev-kit/internal/backlog"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestBacklogEntriesMatchSchemaAndPreserveLegacyMetadata(t *testing.T) {
	for _, kind := range []string{"deferred", "carried", "mechanical", "accepted_objections", "review"} {
		for _, tc := range []struct {
			name, want string
			mutate     func(doc)
		}{
			{"valid", "", func(doc) {}},
			{"missing text", "text", func(item doc) { delete(obj(item["finding"]), "text") }},
			{"empty text", "minLength", func(item doc) { obj(item["finding"])["text"] = "" }},
			{"non-string text", "string", func(item doc) { obj(item["finding"])["text"] = 1 }},
			{"fractional judgment", "integer", func(item doc) { obj(item["source"])["judgment"] = 1.5 }},
			{"non-object target", "object", func(item doc) { obj(item["source"])["target"] = "commit" }},
		} {
			if kind == "review" && tc.name == "fractional judgment" {
				continue
			}
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				b := backlogDocument(kind)
				item := items(b[kind])[0]
				tc.mutate(item)
				data := testutil.Must(json.Marshal(b))
				schemaErr := backlogschema.Validate(data)
				b["branch"], b["commit"], b["legacy"] = "old/topic", "old metadata", doc{"preserved": true}
				r := setup(t, b)
				path := filepath.Join(r.cwd, ".devkit/backlog/topic.json")
				before := testutil.Must(os.ReadFile(path))
				entries, files, err := readBacklog(r.cwd)
				if tc.want == "" {
					testutil.OK(t, schemaErr)
					testutil.OK(t, err)
					testutil.Equal(t, len(entries), 1)
					testutil.Equal(t, files[".devkit/backlog/topic.json"], string(before))
					testutil.Equal(t, obj(entries[0]["content"]), obj(testutil.Must(readJSON(path))[kind].([]any)[0]))
					r.begin()
					return
				}
				testutil.Fails(t, schemaErr, tc.want)
				testutil.Fails(t, err, tc.want)
				testutil.Equal(t, entries, []doc(nil))
				testutil.Equal(t, files, doc(nil))
				testutil.Equal(t, testutil.Must(os.ReadFile(path)), before)
				testutil.Equal(t, r.git("status", "--porcelain"), "")
			})
		}
	}
}

func backlogDocument(kind string) doc {
	b := backlog()
	b["deferred"] = []doc{}
	item := finding()
	if kind == "review" {
		item = doc{"finding": doc{"id": "review-1", "text": "Review text"}, "source": doc{"run_id": "fix1", "kind": "review", "target": doc{"commit": strings.Repeat("a", 40)}}}
	}
	b[kind] = []doc{item}
	return b
}

func TestPrepareCommandRejectsInvalidBacklogWithoutWritingArtifacts(t *testing.T) {
	for _, mutate := range []func(doc){
		func(item doc) { delete(obj(item["finding"]), "text") },
		func(item doc) { obj(item["source"])["judgment"] = 1.5 },
	} {
		b := backlog()
		mutate(items(b["deferred"])[0])
		r := setup(t, b)
		testutil.Equal(t, testutil.Must(start(r.ctx, envVars(nil))).Stopped, false)
		path := filepath.Join(r.cwd, ".devkit/backlog/topic.json")
		before := testutil.Must(os.ReadFile(path))
		head := r.git("rev-parse", "HEAD")
		indexPath := filepath.Join(r.cwd, r.git("rev-parse", "--git-path", "index"))
		index := testutil.Must(os.ReadFile(indexPath))
		var out, errOut bytes.Buffer
		env := envVars(map[string]string{"STATE_DIR": filepath.Dir(filepath.Dir(r.ctx.Dir)), "WORKFLOW_ID": r.ctx.RunID, "ARTIFACTS_DIR": r.artifacts})
		env.Cwd, env.Stdout, env.Stderr = r.cwd, &out, &errOut
		testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"retro", "prepare"}, env), 1)
		testutil.Contains(t, errOut.String(), ".devkit/backlog/topic.json")
		testutil.Equal(t, out.Len(), 0)
		testutil.Equal(t, testutil.Must(os.ReadFile(indexPath)), index)
		testutil.Equal(t, testutil.Must(os.ReadFile(path)), before)
		testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
		testutil.Equal(t, r.git("status", "--porcelain"), "")
		_, err := os.Stat(r.artifacts)
		if !os.IsNotExist(err) {
			t.Fatalf("prepare created artifacts: %v", err)
		}
	}
}
