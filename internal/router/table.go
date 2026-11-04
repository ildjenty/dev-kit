package router

import (
	"maps"
	"slices"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/plan"
)

type Role string

const (
	Worker  Role = "worker"
	Human   Role = "human"
	Judge   Role = "judge"
	Planner Role = "planner"
)

type ConfirmationMode string

const (
	ConfirmNow  ConfirmationMode = "ask"
	Provisional ConfirmationMode = "provisional"
	NoConfirm   ConfirmationMode = "none"
)

type LessonDestination string

const (
	HookLintType LessonDestination = "hook_lint_type"
	PlanTemplate LessonDestination = "plan_template"
	Aspect       LessonDestination = "aspect"
	Principle    LessonDestination = "principle"
	Skill        LessonDestination = "skill"
)

type Lessons struct {
	Destinations []LessonDestination `toml:"destinations"`
}

// Table receives kit classifications and repository additions parsed at the boundary.
// ConfirmationRules provides per-name overrides; the caller resolves the storage format.
type Table struct {
	HumanConfirmation []string                                `toml:"human_confirmation"`
	CheckResults      map[check.JudgmentValue]plan.Transition `toml:"check_results"`
	Roles             map[Role]string                         `toml:"roles"`
	Lessons           Lessons                                 `toml:"lessons"`
	ConfirmationRules map[string]ConfirmationMode             `toml:"-"`
}

// MergeTables overrides classifications by name with repository values without modifying the originals.
// A non-nil HumanConfirmation list replaces the existing categories.
func MergeTables(base, additions Table) Table {
	result := Table{
		HumanConfirmation: slices.Clone(base.HumanConfirmation),
		CheckResults:      maps.Clone(base.CheckResults), Roles: maps.Clone(base.Roles),
		Lessons:           Lessons{Destinations: slices.Clone(base.Lessons.Destinations)},
		ConfirmationRules: maps.Clone(base.ConfirmationRules),
	}
	if additions.HumanConfirmation != nil {
		result.HumanConfirmation = slices.Clone(additions.HumanConfirmation)
	}
	if result.CheckResults == nil {
		result.CheckResults = make(map[check.JudgmentValue]plan.Transition)
	}
	maps.Copy(result.CheckResults, additions.CheckResults)
	if result.Roles == nil {
		result.Roles = make(map[Role]string)
	}
	maps.Copy(result.Roles, additions.Roles)
	if result.ConfirmationRules == nil {
		result.ConfirmationRules = make(map[string]ConfirmationMode)
	}
	maps.Copy(result.ConfirmationRules, additions.ConfirmationRules)
	if additions.Lessons.Destinations != nil {
		result.Lessons.Destinations = slices.Clone(additions.Lessons.Destinations)
	}
	return result
}

// WithConfig applies effective repository confirmation and lesson settings.
func (t Table) WithConfig(settings config.Config) Table {
	destinations := []LessonDestination{}
	for _, destination := range settings.LessonDestinations() {
		destinations = append(destinations, LessonDestination(destination))
	}
	result := MergeTables(t, Table{
		HumanConfirmation: settings.HumanConfirmation(),
		Lessons:           Lessons{Destinations: destinations},
	})
	if settings.MetaLoopEditApproval() == "allow" {
		result.ConfirmationRules[config.MetaLoopEdit] = NoConfirm
	}
	return result
}

// PreferredLesson returns the first candidate destination in table priority order.
func (t Table) PreferredLesson(candidates []LessonDestination) (LessonDestination, bool) {
	for _, destination := range t.Lessons.Destinations {
		if slices.Contains(candidates, destination) {
			return destination, true
		}
	}
	return "", false
}
