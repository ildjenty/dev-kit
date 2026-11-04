package workspace

import (
	"context"
	"reflect"
	"testing"
)

func TestConfiguredNormsExcludeRepositoryOutcomes(t *testing.T) {
	w := newWorkspace(t)
	ctx := context.Background()
	write(t, w.Root(), "devkit.toml", "kit_version = 'v1'\n[paths]\nprinciples = 'norms/principles'\nstages = 'norms/stages'\nchecks = 'norms/checks'\nplans = 'norms/plans'\ntemplates = 'norms/templates'\nrules = 'norms/rules.md'\nrequirements = 'docs/requirements.md'\nstructure = 'docs/structure.json'\n")
	for _, path := range []string{"norms/principles/core.md", "norms/stages/implement.md", "norms/checks/test.md", "norms/plans/default.json", "norms/templates/work.md.tmpl", "norms/rules.md", "AGENTS.md", "CLAUDE.md", ".claude/agents/judge.md", ".codex/agents/judge.toml"} {
		write(t, w.Root(), path, "Original norm\n")
	}
	before, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := w.NormativeIDs(ctx, before.Tree, "norms/principles", "norms/stages", "norms/checks", "norms/plans", "norms/templates", "norms/rules.md", "devkit.toml", "AGENTS.md", "CLAUDE.md", ".claude/agents", ".codex/agents")
	if err != nil || len(ids) != 11 || ids["norms/principles"] == "" || ids["norms/rules.md"] == "" {
		t.Fatalf("configured norms: %v, %v", ids, err)
	}
	for _, path := range []string{"docs/requirements.md", "docs/structure.json", ".devkit/run/generated", ".agents/skills/devkit/SKILL.md", ".claude/settings.json"} {
		write(t, w.Root(), path, "Outcome or host setting\n")
	}
	after, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := w.NormativeIDs(ctx, after.Tree, "norms/principles", "norms/stages", "norms/checks", "norms/plans", "norms/templates", "norms/rules.md", "devkit.toml", "AGENTS.md", "CLAUDE.md", ".claude/agents", ".codex/agents")
	if err != nil || !reflect.DeepEqual(ids, actual) {
		t.Fatalf("outcomes changed norm identities: %v, %v", actual, err)
	}
	write(t, w.Root(), "norms/principles/core.md", "Revised norm\n")
	changed, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	actual, err = w.NormativeIDs(ctx, changed.Tree, "norms/principles", "norms/stages", "norms/checks", "norms/plans", "norms/templates", "norms/rules.md", "devkit.toml", "AGENTS.md", "CLAUDE.md", ".claude/agents", ".codex/agents")
	if err != nil || actual["norms/principles"] == ids["norms/principles"] {
		t.Fatalf("norm edit did not change its identity: %v, %v", actual, err)
	}
}
