package router

import "testing"

func TestResolvedConfirmationSettings(t *testing.T) {
	base := defaultTable(t)
	table := MergeTables(base, Table{HumanConfirmation: []string{}, ConfirmationRules: map[string]ConfirmationMode{"meta_loop_edit": ConfirmNow}})
	if !table.ClassifyConfirmation(ConfirmationInput{Categories: []string{"meta_loop_edit"}}).Required {
		t.Fatal("loop edit policy was removed by an empty category list")
	}
	if table.ClassifyConfirmation(ConfirmationInput{Categories: []string{"dependency_changed"}}).Required {
		t.Fatal("empty replacement retained a dependency confirmation")
	}
	allowed := MergeTables(table, Table{ConfirmationRules: map[string]ConfirmationMode{"meta_loop_edit": NoConfirm}})
	if allowed.ClassifyConfirmation(ConfirmationInput{Categories: []string{"meta_loop_edit"}}).Required {
		t.Fatal("ignored the resolved loop edit policy")
	}
	if !base.ClassifyConfirmation(ConfirmationInput{Categories: []string{"meta_loop_edit"}}).Required {
		t.Fatal("modified the original table")
	}
}

func TestResolvedLessonDestinations(t *testing.T) {
	base := defaultTable(t)
	table := MergeTables(base, Table{Lessons: Lessons{Destinations: []LessonDestination{Aspect, HookLintType}}})
	if got, ok := table.PreferredLesson([]LessonDestination{HookLintType, Aspect}); !ok || got != Aspect {
		t.Fatalf("lesson priority: %q, %v", got, ok)
	}
	table = MergeTables(base, Table{Lessons: Lessons{Destinations: []LessonDestination{}}})
	if _, ok := table.PreferredLesson([]LessonDestination{HookLintType}); ok {
		t.Fatal("ignored an empty lesson destination list")
	}
}
