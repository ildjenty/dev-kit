package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"
[commands]
test = { executable = "./scripts/test", args = ["--all"], timeout_seconds = 120 }
[judges]
review = { executable = "./scripts/judge", args = [], timeout_seconds = 60, output_schema = ".devkit/kit/v1/schemas/judgment.json" }
[deny]
rules = ["Bash(git push *)", "Agent(fork)", "mcp__service__write"]
judge = ["apply_patch", "mcp__service__write"]
[router]
human_confirmation = ["dependency_changed"]
[external.comment]
executable = "./scripts/comment"
args = ["--json"]
timeout_seconds = 30
occurrence_field = "request_id"
query = { executable = "./scripts/find-comment", args = [], timeout_seconds = 10 }
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.KitVersion != "v1" || c.Commands["test"].Timeout() != 120*time.Second || c.Commands["test"].Args[0] != "--all" {
		t.Fatalf("configuration does not match: %+v", c)
	}
	if c.Judges["review"].Executable != "./scripts/judge" || c.Judges["review"].OutputSchema == "" {
		t.Fatalf("judge command does not match: %+v", c.Judges)
	}
	op := c.External["comment"]
	if op.Query == nil || op.Query.Timeout() != 10*time.Second || op.OccurrenceField != "request_id" {
		t.Fatalf("external operation does not match: %+v", op)
	}
	if len(c.Deny.Rules) != 3 || len(c.Deny.Judge) != 2 || c.Deny.Judge[0] != "apply_patch" || c.Router.HumanConfirmation[0] != DependencyChanged {
		t.Fatalf("rules do not match: %+v", c)
	}
}

func TestDefaults(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"`))
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := Paths{
		Principles: ".devkit/principles", Stages: ".devkit/stages", Checks: ".devkit/checks",
		Plans: ".devkit/plans", Templates: ".devkit/templates", Rules: ".devkit/rules.md",
		Requirements: ".devkit/requirements.md", Structure: ".devkit/structure.json", Detect: ".devkit/detect.json",
	}
	wantDependencies := []string{"go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Cargo.toml", "Cargo.lock", "Gemfile", "Gemfile.lock", "requirements.txt", "pyproject.toml", "poetry.lock", "uv.lock", "composer.json", "composer.lock"}
	wantConfirmation := []string{DependencyChanged, TargetChangedAgain}
	wantDestinations := []string{"hook_lint_type", "plan_template", "aspect", "principle", "skill"}
	if c.DefaultPlan != "default" || c.Paths != wantPaths || c.Approval != (Approval{Plan: "requirements", MetaLoopEdit: "confirm"}) || c.Git.Branch != "devkit/{work_id}" || c.Git.CommitMessage != "devkit: {stage} ({work_id})" || c.Git.AllowEmptyCommit == nil || !*c.Git.AllowEmptyCommit || c.Limits.StopContinuation != 3 {
		t.Fatalf("parsed defaults do not match: %+v", c)
	}
	if !slices.Equal(c.Classification.DependencyFiles, wantDependencies) || !slices.Equal(c.Router.HumanConfirmation, wantConfirmation) || !slices.Equal(c.Lessons.Destinations, wantDestinations) {
		t.Fatalf("parsed list defaults do not match: %+v", c)
	}
	for _, settings := range []Config{{}, c} {
		if settings.DefaultPlanName() != "default" || settings.RepositoryPaths() != wantPaths || settings.PlanApproval() != "requirements" || settings.MetaLoopEditApproval() != "confirm" || !settings.AllowEmptyCommit() || settings.StopContinuation() != 3 {
			t.Fatalf("effective defaults do not match: %+v", settings)
		}
		if !slices.Equal(settings.DependencyFiles(), wantDependencies) || !slices.Equal(settings.HumanConfirmation(), wantConfirmation) || !slices.Equal(settings.LessonDestinations(), wantDestinations) {
			t.Fatalf("effective list defaults do not match: %+v", settings)
		}
		branch, err := settings.BranchName("work-123")
		if err != nil || branch != "devkit/work-123" {
			t.Fatalf("branch = %q, error = %v", branch, err)
		}
		if message := settings.CommitMessage("implement", "work-123", "Fix a defect"); message != "devkit: implement (work-123)" {
			t.Fatalf("commit message = %q", message)
		}
		for _, dependency := range wantDependencies {
			if !settings.IsDependencyFile(dependency) || !settings.IsDependencyFile("nested/"+dependency) {
				t.Errorf("default dependency was not matched: %s", dependency)
			}
		}
	}
}

func TestOverrides(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"
default_plan = "fix"
[paths]
principles = "norms/principles"
stages = "norms/stages"
checks = "norms/checks"
plans = "norms/plans"
templates = "norms/templates"
rules = "norms/rules.md"
requirements = "docs/requirements.md"
structure = "docs/structure.json"
detect = "norms/detect.json"
[approval]
plan = "always"
meta_loop_edit = "allow"
[git]
branch = "work/{work_id}/topic"
commit_message = "{task}: {stage} [{work_id}]"
allow_empty_commit = false
[classification]
dependency_files = ["deps/*.lock", "manifest-?.json"]
[router]
human_confirmation = ["target_changed_again"]
[limits]
stop_continuation = 1
[lessons]
destinations = ["skill", "aspect"]
`))
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := Paths{
		Principles: "norms/principles", Stages: "norms/stages", Checks: "norms/checks",
		Plans: "norms/plans", Templates: "norms/templates", Rules: "norms/rules.md",
		Requirements: "docs/requirements.md", Structure: "docs/structure.json", Detect: "norms/detect.json",
	}
	if c.DefaultPlanName() != "fix" || c.RepositoryPaths() != wantPaths || c.PlanApproval() != "always" || c.MetaLoopEditApproval() != "allow" || c.AllowEmptyCommit() || c.StopContinuation() != 1 {
		t.Fatalf("effective settings do not match overrides: %+v", c)
	}
	if !slices.Equal(c.HumanConfirmation(), []string{TargetChangedAgain}) || !slices.Equal(c.LessonDestinations(), []string{"skill", "aspect"}) || !slices.Equal(c.DependencyFiles(), []string{"deps/*.lock", "manifest-?.json"}) {
		t.Fatalf("effective lists do not match overrides: %+v", c)
	}
	branch, err := c.BranchName("abc")
	if err != nil || branch != "work/abc/topic" {
		t.Fatalf("branch = %q, error = %v", branch, err)
	}
	if message := c.CommitMessage("test", "abc", "Describe {stage}"); message != "Describe {stage}: test [abc]" {
		t.Fatalf("commit message = %q", message)
	}
	for _, name := range []string{"deps/build.lock", "nested/manifest-a.json"} {
		if !c.IsDependencyFile(name) {
			t.Errorf("dependency was not matched: %s", name)
		}
	}
	for _, name := range []string{"go.mod", "deps/nested/build.lock", "nested/manifest-ab.json"} {
		if c.IsDependencyFile(name) {
			t.Errorf("unexpected dependency match: %s", name)
		}
	}
	for _, values := range [][]string{c.DependencyFiles(), c.HumanConfirmation(), c.LessonDestinations()} {
		values[0] = "changed"
	}
	if c.Classification.DependencyFiles[0] != "deps/*.lock" || c.Router.HumanConfirmation[0] != TargetChangedAgain || c.Lessons.Destinations[0] != "skill" {
		t.Fatal("effective list accessors changed the configuration")
	}
}

func TestEmptyListsReplaceDefaults(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"
[classification]
dependency_files = []
[router]
human_confirmation = []
[lessons]
destinations = []
`))
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]string{c.DependencyFiles(), c.HumanConfirmation(), c.LessonDestinations()} {
		if values == nil || len(values) != 0 {
			t.Fatalf("expected an explicitly empty list, got %v", values)
		}
	}
	if c.IsDependencyFile("go.mod") {
		t.Fatal("an empty dependency list used the default patterns")
	}
}

func TestPartialSettings(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"
[paths]
rules = "rules.md"
[approval]
plan = "deviation"
[git]
allow_empty_commit = true
`))
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := (Config{}).RepositoryPaths()
	wantPaths.Rules = "rules.md"
	if c.RepositoryPaths() != wantPaths || c.PlanApproval() != "deviation" || c.MetaLoopEditApproval() != "confirm" || !c.AllowEmptyCommit() {
		t.Fatalf("partial settings lost defaults or overrides: %+v", c)
	}
	if got := (Config{Paths: Paths{Rules: "rules.md"}}).RepositoryPaths(); got != wantPaths {
		t.Fatalf("effective partial paths = %+v", got)
	}
}

func TestInvalidRepositoryPaths(t *testing.T) {
	for _, field := range []string{"principles", "stages", "checks", "plans", "templates", "rules", "requirements", "structure", "detect"} {
		for _, value := range []string{"", ".", "/tmp/norms", "C:/norms", `norms\rules`, "../norms", "norms/../rules", "norms//rules", "./norms", "norms/", ".devkit/run", ".devkit/run/norms"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				_, err := Parse([]byte(fmt.Sprintf("kit_version = 'v1'\n[paths]\n%s = '%s'", field, value)))
				if err == nil || !strings.Contains(err.Error(), "paths."+field) {
					t.Fatalf("path %q: error = %v", value, err)
				}
			})
		}
	}
	for _, value := range []string{"rules.md", ".devkit/runtime/rules", "norms/.devkit/run/rules", "norms with spaces/rules"} {
		if _, err := Parse([]byte(fmt.Sprintf("kit_version = 'v1'\n[paths]\nrules = '%s'", value))); err != nil {
			t.Errorf("valid path %q: %v", value, err)
		}
	}
}

func TestInvalidNewSettings(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"default_plan = ''", "default_plan"},
		{"default_plan = ' '", "default_plan"},
		{"[approval]\nplan = 'sometimes'", "approval.plan"},
		{"[approval]\nplan = ''", "approval.plan"},
		{"[approval]\nmeta_loop_edit = 'deny'", "approval.meta_loop_edit"},
		{"[approval]\nmeta_loop_edit = ''", "approval.meta_loop_edit"},
		{"[plan_check]\naspect = ''", "plan_check.aspect"},
		{"[plan_check]\naspect = ' '", "plan_check.aspect"},
		{"[plan_check]\nrunner = ''", "plan_check.runner"},
		{"[plan_check]\nrunner = ' '", "plan_check.runner"},
		{"[git]\nbranch = 'topic'", "git.branch"},
		{"[git]\nbranch = ''", "git.branch"},
		{"[git]\ncommit_message = ''", "git.commit_message"},
		{"[git]\ncommit_message = '{unknown}'", "git.commit_message"},
		{"[git]\ncommit_message = '{stage'", "git.commit_message"},
		{"[git]\ncommit_message = 'stage}'", "git.commit_message"},
		{"[git]\ncommit_message = '{{stage}}'", "git.commit_message"},
		{"[classification]\ndependency_files = ['[']", "classification.dependency_files"},
		{"[router]\nhuman_confirmation = ['billing_changed']", "router.human_confirmation"},
		{"[router]\nhuman_confirmation = ['meta_loop_edit']", "router.human_confirmation"},
		{"[router]\nhuman_confirmation = ['undeclared_meta_loop_edit']", "router.human_confirmation"},
		{"[limits]\nstop_continuation = 0", "limits.stop_continuation"},
		{"[limits]\nstop_continuation = -1", "limits.stop_continuation"},
		{"[lessons]\ndestinations = ['kit']", "lessons.destinations"},
		{"[lessons]\ndestinations = ['']", "lessons.destinations"},
		{"[lessons]\ndestinations = ['aspect', 'aspect']", "lessons.destinations"},
		{"[paths]\nunknown = 'norms'", "unknown configuration key"},
		{"[timeouts]\ncommand_seconds = 10", "unknown configuration key"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			_, err := Parse([]byte("kit_version = 'v1'\n" + tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, expected field = %s", err, tt.want)
			}
		})
	}
}

func TestInvalidBranchNames(t *testing.T) {
	for _, name := range []string{"-work", "with space", "a..b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", `a\b`, "a\tb", "a\nb", "a\x7fb", ".hidden", "a/.hidden", "a.lock", "a.lock/b", "a@{b", "a.", "a/", "/a", "a//b", "HEAD", ""} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			c := Config{Git: Git{Branch: "{work_id}"}}
			if _, err := c.BranchName(name); err == nil {
				t.Fatalf("accepted invalid rendered branch name %q", name)
			}
		})
	}
	for _, pattern := range []string{"-{work_id}", "with space/{work_id}", "a..b/{work_id}", "a~{work_id}", "a^{work_id}", "a:{work_id}", "a?{work_id}", "a*{work_id}", "a[{work_id}", `a\{work_id}`, "a\t{work_id}", ".{work_id}", "a/.{work_id}", "{work_id}.lock", "a.lock/{work_id}", "a@{b/{work_id}", "{work_id}.", "{work_id}/", "/{work_id}", "a//{work_id}"} {
		_, err := Parse([]byte("kit_version = 'v1'\n[git]\nbranch = " + fmt.Sprintf("%q", pattern)))
		if err == nil || !strings.Contains(err.Error(), "git.branch") {
			t.Errorf("pattern %q: error = %v", pattern, err)
		}
	}
}

func TestValidateRejectsChangedSettings(t *testing.T) {
	c, err := Parse([]byte(`kit_version = "v1"`))
	if err != nil {
		t.Fatal(err)
	}
	before := c
	if err := c.Validate(); err != nil || !reflect.DeepEqual(c, before) {
		t.Fatalf("validation changed settings or failed: %v", err)
	}
	c.Limits.StopContinuation = 0
	if err := c.Validate(); err == nil {
		t.Fatal("validation accepted a disabled continuation limit")
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"kit_missing", "", "kit_version"},
		{"syntax", "kit_version = [", "parse"},
		{"unknown", "kit_version = 'v1'\nmodel = 'x'", "unknown"},
		{"executable", "kit_version = 'v1'\n[commands.test]\ntimeout_seconds = 1", "executable"},
		{"timeout", "kit_version = 'v1'\n[commands.test]\nexecutable = 'test'\ntimeout_seconds = 0", "timeout_seconds"},
		{"timeout_overflow", "kit_version = 'v1'\n[commands.test]\nexecutable = 'test'\ntimeout_seconds = 9223372036854775807", "timeout_seconds"},
		{"schema", "kit_version = 'v1'\n[judges.review]\nexecutable = 'judge'\ntimeout_seconds = 1", "output_schema"},
		{"query", "kit_version = 'v1'\n[external.post]\nexecutable = 'post'\ntimeout_seconds = 1\nquery = { executable = 'query' }", "query.timeout_seconds"},
		{"deny", "kit_version = 'v1'\n[deny]\nrules = ['Bash(git push)']", "deny.rules"},
		{"agent_extra_close", "kit_version = 'v1'\n[deny]\nrules = ['Agent(fork))']", "deny.rules"},
		{"agent_extra_open", "kit_version = 'v1'\n[deny]\nrules = ['Agent((fork)']", "deny.rules"},
		{"agent_nested", "kit_version = 'v1'\n[deny]\nrules = ['Agent((fork))']", "deny.rules"},
		{"judge_empty", "kit_version = 'v1'\n[deny]\njudge = ['']", "deny.judge"},
		{"judge_command", "kit_version = 'v1'\n[deny]\njudge = ['Bash(git push *)']", "deny.judge"},
		{"judge_whitespace", "kit_version = 'v1'\n[deny]\njudge = ['apply patch']", "deny.judge"},
		{"router", "kit_version = 'v1'\n[router]\nhuman_confirmation = ['']", "human_confirmation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, expected field = %s", err, tt.want)
			}
		})
	}
}
