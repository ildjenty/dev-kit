package retro

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPrepareUsesFixedMaterialsAfterStandardsRelocation(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	mapping := testutil.Must(concerns.Load(filepath.Join(r.cwd, ".devkit/standards")))
	index := testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/README.md")))
	source := testutil.Must(os.ReadFile(filepath.Join(r.artifacts, "criteria/source.json")))
	r.git("mv", ".devkit/standards", "policy")
	r.put("policy/README.md", "Current standards\n")
	r.put("policy/concerns.json", `{}`)
	r.put(".devkit/config.json", `{"language":"de","standards":"policy"}`)
	r.commit()
	for _, dir := range []string{r.artifacts, t.TempDir()} {
		display := testutil.Must(filepath.Rel(r.cwd, dir))
		result := testutil.Must(prepare(r.ctx, display))
		input := testutil.Must(readJSON(file(dir, "input.json")))
		fixed := filepath.Join(display, "criteria/.devkit/standards")
		testutil.Equal(t, input["language"], "de")
		testutil.Equal(t, result["criteria"], fixed)
		testutil.Equal(t, input["criteria"], fixed)
		testutil.Equal(t, obj(input["folders"])["standards"], "policy")
		testutil.Equal(t, input["standards_index"], string(index))
		testutil.Equal(t, input["concerns"], mapping)
		testutil.Equal(t, input["writer_materials"], mapping.Materials(fixed, "writer:retro", "plan"))
		testutil.Equal(t, input["judge_materials"], mapping.Materials(fixed, "judge:retro", "plan"))
		testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/source.json"))), source)
		testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/.devkit/standards/README.md"))), index)
	}
}
