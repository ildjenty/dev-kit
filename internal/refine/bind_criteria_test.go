package refine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPrepareUsesFixedMaterialsAfterStandardsRelocation(t *testing.T) {
	r := setup(t, doc{"standards": "rules"})
	r.begin()
	mapping := testutil.Must(concerns.Load(filepath.Join(r.cwd, "rules")))
	authority := testutil.Must(os.ReadFile(filepath.Join(r.cwd, "rules/authority.md")))
	source := testutil.Must(os.ReadFile(filepath.Join(r.artifacts, "criteria/source.json")))
	r.git("mv", "rules", "policy")
	r.put("policy/authority.md", "Current authority\n")
	r.put("policy/concerns.json", `{}`)
	r.put(".devkit/config.json", `{"language":"de","standards":"policy"}`)
	r.commit()
	for _, dir := range []string{r.artifacts, t.TempDir()} {
		display := testutil.Must(filepath.Rel(r.cwd, dir))
		testutil.Must(prepare(r.ctx, envVars(nil), display, "", "", ""))
		input := testutil.Must(readJSON(file(dir, "input.json")))
		fixed := filepath.Join(display, "criteria/rules")
		testutil.Equal(t, input["language"], "de")
		testutil.Equal(t, input["criteria"], fixed)
		testutil.Equal(t, obj(input["folders"])["standards"], "policy")
		testutil.Equal(t, input["writer_materials"], mapping.Materials(fixed, "writer:refine", "plan"))
		testutil.Equal(t, input["judge_materials"], mapping.Materials(fixed, "judge:refine", "plan"))
		testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/source.json"))), source)
		testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(dir, "criteria/rules/authority.md"))), authority)
	}
}
