package router

import (
	"slices"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
)

func TestEffectiveConfirmationSettings(t *testing.T) {
	base := defaultTable(t)
	for _, tc := range []struct {
		name     string
		settings config.Config
		want     []string
	}{
		{"defaults", config.Config{}, []string{config.DependencyChanged, config.MetaLoopEdit, config.TargetChangedAgain}},
		{"replacement", config.Config{Router: config.Router{HumanConfirmation: []string{config.MetaLoopEdit}}}, []string{config.MetaLoopEdit}},
		{"empty replacement", config.Config{Router: config.Router{HumanConfirmation: []string{}}}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := base.WithConfig(tc.settings)
			for _, category := range []string{config.DependencyChanged, config.MetaLoopEdit, config.TargetChangedAgain} {
				confirmation := table.ClassifyConfirmation(ConfirmationInput{Categories: []string{category}, NeedsDecision: true})
				if confirmation.Required != slices.Contains(tc.want, category) {
					t.Fatalf("category %s: %+v; effective categories: %v", category, confirmation, tc.want)
				}
			}
		})
	}
	allowed := base.WithConfig(config.Config{Approval: config.Approval{MetaLoopEdit: "allow"}})
	if allowed.ClassifyConfirmation(ConfirmationInput{Categories: []string{config.MetaLoopEdit}, NeedsDecision: true}).Required {
		t.Fatal("ignored the meta-loop edit allow policy")
	}
	if !base.ClassifyConfirmation(ConfirmationInput{Categories: []string{config.MetaLoopEdit}}).Required {
		t.Fatal("modified the original confirmation rules")
	}
}

func TestEffectiveLessonDestinations(t *testing.T) {
	base := defaultTable(t)
	want := []LessonDestination{HookLintType, PlanTemplate, Aspect, Principle, Skill}
	if !slices.Equal(base.Lessons.Destinations, want) {
		t.Fatalf("default lesson destinations: %v; want: %v", base.Lessons.Destinations, want)
	}
	table := base.WithConfig(config.Config{Lessons: config.Lessons{Destinations: []string{"aspect", "hook_lint_type"}}})
	if got, ok := table.PreferredLesson([]LessonDestination{HookLintType, Aspect}); !ok || got != Aspect {
		t.Fatalf("ignored configured lesson priority: %q, %v", got, ok)
	}
	table = base.WithConfig(config.Config{Lessons: config.Lessons{Destinations: []string{}}})
	if _, ok := table.PreferredLesson(want); ok {
		t.Fatal("ignored an explicitly empty lesson destination list")
	}
}
