package router

import (
	"maps"
	"slices"

	"github.com/ildjenty/dev-kit/internal/check"
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

const (
	DependencyChanged      = "dependency_changed"
	UndeclaredMetaLoopEdit = "undeclared_meta_loop_edit"
	TargetChangedAgain     = "target_changed_again"
)

type LessonDestination string

const (
	HookLintType LessonDestination = "hook_lint_type"
	PlanTemplate LessonDestination = "plan_template"
	Principle    LessonDestination = "principle"
	Skill        LessonDestination = "skill"
	Kit          LessonDestination = "kit"
)

type Lessons struct {
	Destinations []LessonDestination `toml:"destinations"`
}

// Table は kit の分類表と利用側の追記を、境界で解析済みの値として受け取る。
// ConfirmationRules は名前ごとの上書きであり、保存形式は呼び出し側で解決する。
type Table struct {
	HumanConfirmation []string                                `toml:"human_confirmation"`
	CheckResults      map[check.JudgmentValue]plan.Transition `toml:"check_results"`
	Roles             map[Role]string                         `toml:"roles"`
	Lessons           Lessons                                 `toml:"lessons"`
	ConfirmationRules map[string]ConfirmationMode             `toml:"-"`
}

// MergeTables は同名の分類を利用側の値で上書きする。元の表は変更しない。
// human_confirmation は追記で指定された場合に列全体を置き換える。
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

// PreferredLesson は候補のうち分類表の優先順で最初の反映先を返す。
func (t Table) PreferredLesson(candidates []LessonDestination) (LessonDestination, bool) {
	for _, destination := range t.Lessons.Destinations {
		if slices.Contains(candidates, destination) {
			return destination, true
		}
	}
	return "", false
}
