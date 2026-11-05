package install

import (
	"context"
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
)

func TestInitWritesDynamicDefaults(t *testing.T) {
	root := repository(t)
	if _, err := Init(context.Background(), root, nil, InitOptions{NoCommit: true}); err != nil {
		t.Fatal(err)
	}
	c, err := config.Parse(readFile(t, root, "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Planning.MaxRounds != 3 || len(c.Planning.Inputs) != 4 || len(c.PlanCheck.Inputs) != 4 {
		t.Fatal("init omitted planning settings")
	}
	if c.Limits.MaxRoundsCeiling != 3 || c.Limits.MaxNoProgressCeiling != 2 || c.Limits.MaxTotalRounds == nil || *c.Limits.MaxTotalRounds != 40 {
		t.Fatal("init omitted round ceilings")
	}
	if c.Checks["command:test"].Runner != "command:test" || c.Checks["test"].Runner != "judge" || len(c.Checks["behavior"].Criteria) == 0 {
		t.Fatal("init omitted check definitions")
	}
	if len(c.Loop.Stage) != 4 || len(c.Loop.Path) != 2 {
		t.Fatal("init omitted loop rules")
	}
	if !c.Loop.Stage[0].Gate || !slices.Equal(c.Loop.Stage[1].Checks, []string{"command:test", "test", "diff"}) || c.Loop.Stage[3].History != "all" || c.Loop.Path[0].Definition != "implement" || c.Loop.Path[0].Follows != "verification" || c.Loop.Path[1].Last != "retrospective" {
		t.Fatal("init wrote incorrect loop rules")
	}
	if len(readFile(t, root, ".devkit/stages/edit.md")) == 0 {
		t.Fatal("init omitted edit definition")
	}
}
