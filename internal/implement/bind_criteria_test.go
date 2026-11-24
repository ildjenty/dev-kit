package implement

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPrepareUsesFixedMaterialsAfterStandardsRelocation(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	original := testutil.Must(readJSON[record.Document](filepath.Join(artifacts, "implement/input.json")))
	mapping := testutil.Must(concerns.Load(filepath.Join(ctx.Cwd, "standards")))
	source := testutil.Must(os.ReadFile(filepath.Join(artifacts, "criteria/source.json")))
	testutil.Git(t, ctx.Cwd, "mv", "standards", "policy")
	testutil.Put(t, ctx.Cwd, "policy/git.md", "Use `current:`.\n")
	testutil.Put(t, ctx.Cwd, "policy/concerns.json", `{}`)
	putJSON(t, filepath.Join(ctx.Cwd, ".devkit/config.json"), record.Document{"language": "de", "standards": "policy", "checks": []record.Document{{"name": "current", "command": "true", "timeout_seconds": 10}}})
	testutil.Git(t, ctx.Cwd, "add", "-A")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "Relocate standards")
	for _, dir := range []string{artifacts, t.TempDir()} {
		display := testutil.Must(filepath.Rel(ctx.Cwd, dir))
		testutil.Must(prepare(ctx, display))
		testutil.Must(next(dir))
		input := testutil.Must(readJSON[record.Document](filepath.Join(dir, "implement/unit.json")))
		testutil.Equal(t, input["language"], "de")
		testutil.Equal(t, input["standards"], "standards")
		testutil.Equal(t, input["criteria_folder"], filepath.Join(display, "criteria/standards"))
		testutil.Equal(t, input["commit_prefixes"], original["commit_prefixes"])
		testutil.Equal(t, input["checks"], []record.Document{{"name": "unit", "command": "node --test", "paths": []string{"src/**"}}, {"name": "current", "command": "true"}})
		for key, consumer := range map[string]string{"writer_materials": "writer:implement", "test_materials": "writer:tests", "step_back_materials": "judge:step-back", "dispute_materials": "judge:dispute"} {
			testutil.Equal(t, input[key], mapping.Materials(filepath.Join(display, "criteria/standards"), consumer, "code"))
		}
		testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/source.json"))), source)
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/standards/git.md")))), "Use `change:` or `repair:` for commits.\n")
	}
}
