package config

import (
	"bytes"
	"fmt"
	"math"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
)

// Config represents validated devkit.toml settings.
type Config struct {
	KitVersion     string                       `toml:"kit_version"`
	Judges         map[string]Judge             `toml:"judges"`
	Commands       map[string]Command           `toml:"commands"`
	External       map[string]ExternalOperation `toml:"external"`
	Deny           Deny                         `toml:"deny"`
	Router         Router                       `toml:"router"`
	PlanCheck      PlanCheck                    `toml:"plan_check"`
	DefaultPlan    string                       `toml:"default_plan"`
	Sources        map[string]Source            `toml:"sources"`
	State          State                        `toml:"state"`
	Approval       Approval                     `toml:"approval"`
	Git            Git                          `toml:"git"`
	Classification Classification               `toml:"classification"`
	Limits         Limits                       `toml:"limits"`
	Lessons        Lessons                      `toml:"lessons"`
	Detection      Detection                    `toml:"detection"`
	Parallel       Parallel                     `toml:"parallel"`
}

// Parallel limits the number of outstanding unit workers.
type Parallel struct {
	MaxWorkers int `toml:"max_workers"`
}

// MaxWorkers returns the effective limit for unit workers.
func (c Config) MaxWorkers() int {
	if c.Parallel.MaxWorkers == 0 {
		return 3
	}
	return c.Parallel.MaxWorkers
}

// Approval controls when plans and edits to the development loop require approval.
type Approval struct {
	Plan         string `toml:"plan"`
	MetaLoopEdit string `toml:"meta_loop_edit"`
}

// PlanCheck selects the name, runner, criteria, and detection specification for plan judgments.
type PlanCheck struct {
	Aspect   string      `toml:"aspect"`
	Runner   string      `toml:"runner"`
	Criteria []Selection `toml:"criteria"`
	Examples *Selection  `toml:"examples"`
}

// PlanCheckSettings returns the effective plan judgment settings.
func (c Config) PlanCheckSettings() PlanCheck {
	settings := c.PlanCheck
	if settings.Aspect == "" {
		settings.Aspect = "plan"
	}
	if settings.Runner == "" {
		settings.Runner = "devkit-judge"
	}
	if settings.Criteria == nil {
		settings.Criteria = []Selection{{Source: "principles", Select: "review/plan.md"}, {Source: "plans", Select: "README.md"}}
	} else {
		settings.Criteria = slices.Clone(settings.Criteria)
	}
	if settings.Examples == nil {
		settings.Examples = &Selection{Source: "examples", Select: "plan/spec.json"}
	} else {
		examples := *settings.Examples
		settings.Examples = &examples
	}
	return settings
}

// Git holds work branch and stage commit settings.
type Git struct {
	Branch           string `toml:"branch"`
	CommitMessage    string `toml:"commit_message"`
	AllowEmptyCommit *bool  `toml:"allow_empty_commit"`
}

// Classification holds dependency file glob patterns.
type Classification struct {
	DependencyFiles []string `toml:"dependency_files"`
}

// Limits holds continuation limits that cannot be disabled.
type Limits struct {
	StopContinuation int `toml:"stop_continuation"`
}

// Lessons holds the priority order of repository lesson destinations.
type Lessons struct {
	Destinations []string `toml:"destinations"`
}

// Detection controls whether required judge checks need passing detection tests.
type Detection struct {
	Required *bool `toml:"required"`
}

// DetectionRequired returns whether required judge checks need detection tests.
func (c Config) DetectionRequired() bool {
	return c.Detection.Required == nil || *c.Detection.Required
}

// Classification names identify changes that may require human confirmation.
const (
	DependencyChanged  = "dependency_changed"
	MetaLoopEdit       = "meta_loop_edit"
	TargetChangedAgain = "target_changed_again"
)

// DefaultPlanName returns the effective plan template name.
func (c Config) DefaultPlanName() string {
	if c.DefaultPlan == "" {
		return "default"
	}
	return c.DefaultPlan
}

// PlanApproval returns the effective plan approval policy.
func (c Config) PlanApproval() string {
	if c.Approval.Plan == "" {
		return "requirements"
	}
	return c.Approval.Plan
}

// MetaLoopEditApproval returns the effective development loop edit policy.
func (c Config) MetaLoopEditApproval() string {
	if c.Approval.MetaLoopEdit == "" {
		return "confirm"
	}
	return c.Approval.MetaLoopEdit
}

func (c Config) branchPattern() string {
	if c.Git.Branch == "" {
		return "devkit/{work_id}"
	}
	return c.Git.Branch
}

// BranchName renders and validates the work branch name.
func (c Config) BranchName(workID string) (string, error) {
	name := strings.ReplaceAll(c.branchPattern(), "{work_id}", workID)
	if !validBranchName(name) {
		return "", fmt.Errorf("git.branch renders an invalid branch name: %q", name)
	}
	return name, nil
}

func (c Config) commitMessagePattern() string {
	if c.Git.CommitMessage == "" {
		return "devkit: {stage} ({work_id})"
	}
	return c.Git.CommitMessage
}

// CommitMessage renders the stage commit message without interpreting replacement values.
func (c Config) CommitMessage(stage, workID, task string) string {
	return strings.NewReplacer("{stage}", stage, "{work_id}", workID, "{task}", task).Replace(c.commitMessagePattern())
}

// AllowEmptyCommit returns whether stages without changes create empty commits.
func (c Config) AllowEmptyCommit() bool {
	return c.Git.AllowEmptyCommit == nil || *c.Git.AllowEmptyCommit
}

// DependencyFiles returns effective dependency globs, preserving an explicitly empty list.
func (c Config) DependencyFiles() []string {
	if c.Classification.DependencyFiles != nil {
		return slices.Clone(c.Classification.DependencyFiles)
	}
	return []string{"go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Cargo.toml", "Cargo.lock", "Gemfile", "Gemfile.lock", "requirements.txt", "pyproject.toml", "poetry.lock", "uv.lock", "composer.json", "composer.lock"}
}

// IsDependencyFile matches dependency globs against a slash path and its base name.
func (c Config) IsDependencyFile(name string) bool {
	for _, pattern := range c.DependencyFiles() {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
		if matched, _ := path.Match(pattern, path.Base(name)); matched {
			return true
		}
	}
	return false
}

// HumanConfirmation returns effective confirmation categories, replacing defaults when set.
func (c Config) HumanConfirmation() []string {
	if c.Router.HumanConfirmation != nil {
		return slices.Clone(c.Router.HumanConfirmation)
	}
	return []string{DependencyChanged, TargetChangedAgain}
}

// StopContinuation returns the effective Stop continuation limit.
func (c Config) StopContinuation() int {
	if c.Limits.StopContinuation == 0 {
		return 3
	}
	return c.Limits.StopContinuation
}

// LessonDestinations returns the effective lesson destination priority order.
func (c Config) LessonDestinations() []string {
	if c.Lessons.Destinations != nil {
		return slices.Clone(c.Lessons.Destinations)
	}
	return []string{"hook_lint_type", "plan_template", "aspect", "principle", "skill"}
}

// Command registers a command to execute without shell expansion.
type Command struct {
	Executable     string   `toml:"executable"`
	Args           []string `toml:"args"`
	TimeoutSeconds int64    `toml:"timeout_seconds"`
}

// Timeout returns the registered time limit.
func (c Command) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Judge registers an evaluation command with an output schema.
type Judge struct {
	Command
	OutputSchema string `toml:"output_schema"`
}

// ExternalOperation registers an external operation and an optional query command.
// occurrence_field specifies the input field identifying an occurrence of an operation repeated with the same input.
type ExternalOperation struct {
	Command
	Query           *Command `toml:"query"`
	OccurrenceField string   `toml:"occurrence_field"`
}

// Deny holds rules for all bound sessions and for devkit judge and planner subagents.
type Deny struct {
	Rules []string `toml:"rules"`
	Judge []string `toml:"judge"`
}

// Router holds a replacement for the default human confirmation categories.
type Router struct {
	HumanConfirmation []string `toml:"human_confirmation"`
}

// Parse parses and validates supplied TOML without reading files.
func Parse(data []byte) (Config, error) {
	c := Config{}
	c.DefaultPlan = c.DefaultPlanName()
	c.State.Detect = c.DetectionPath()
	c.Approval = Approval{Plan: c.PlanApproval(), MetaLoopEdit: c.MetaLoopEditApproval()}
	allowEmpty := c.AllowEmptyCommit()
	c.Git = Git{Branch: c.branchPattern(), CommitMessage: c.commitMessagePattern(), AllowEmptyCommit: &allowEmpty}
	c.Classification.DependencyFiles = c.DependencyFiles()
	c.Router.HumanConfirmation = c.HumanConfirmation()
	c.PlanCheck = c.PlanCheckSettings()
	c.Limits.StopContinuation = c.StopContinuation()
	c.Lessons.Destinations = c.LessonDestinations()
	c.Parallel.MaxWorkers = c.MaxWorkers()
	metadata, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&c)
	if err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if keys := metadata.Undecoded(); len(keys) != 0 {
		return Config{}, fmt.Errorf("unknown configuration key: %s", keys[0])
	}
	c.applySourceDefaults(metadata)
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate checks required settings and formats without checking path existence or kit compatibility.
func (c Config) Validate() error {
	if c.Parallel.MaxWorkers < 1 {
		return fmt.Errorf("parallel.max_workers must be at least 1")
	}
	if strings.TrimSpace(c.KitVersion) == "" {
		return fmt.Errorf("kit_version is required")
	}
	if err := c.validateCommands(); err != nil {
		return err
	}
	if err := c.validateJudges(); err != nil {
		return err
	}
	if err := c.validateExternalOperations(); err != nil {
		return err
	}
	if err := c.validateDeny(); err != nil {
		return err
	}
	if err := c.validateRouter(); err != nil {
		return err
	}
	if err := c.validatePlanSettings(); err != nil {
		return err
	}
	if err := c.validateSources(); err != nil {
		return err
	}
	if err := c.validateApproval(); err != nil {
		return err
	}
	if err := c.validateGit(); err != nil {
		return err
	}
	if err := c.validateDependencyFiles(); err != nil {
		return err
	}
	if err := c.validateContinuationLimit(); err != nil {
		return err
	}
	if err := c.validateLessonDestinations(); err != nil {
		return err
	}
	return nil
}

func (c Config) validateCommands() error {
	for name, command := range c.Commands {
		if err := validateCommand("commands", name, command); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) validateJudges() error {
	for name, judge := range c.Judges {
		if err := validateCommand("judges", name, judge.Command); err != nil {
			return err
		}
		if strings.TrimSpace(judge.OutputSchema) == "" {
			return fmt.Errorf("judges.%s.output_schema is required", name)
		}
	}
	return nil
}

func (c Config) validateExternalOperations() error {
	for name, operation := range c.External {
		if err := validateCommand("external", name, operation.Command); err != nil {
			return err
		}
		if operation.Query != nil {
			if err := validateCommand("external", name+".query", *operation.Query); err != nil {
				return err
			}
		}
		if operation.OccurrenceField != "" && strings.TrimSpace(operation.OccurrenceField) == "" {
			return fmt.Errorf("external.%s.occurrence_field must not contain only whitespace", name)
		}
	}
	return nil
}

func (c Config) validateDeny() error {
	for _, rule := range c.Deny.Rules {
		if !validDenyRule(rule) {
			return fmt.Errorf("invalid deny.rules format: %q", rule)
		}
	}
	for _, tool := range c.Deny.Judge {
		if tool == "" || strings.ContainsAny(tool, "()") || strings.IndexFunc(tool, unicode.IsSpace) >= 0 {
			return fmt.Errorf("invalid deny.judge tool name: %q", tool)
		}
	}
	return nil
}

func (c Config) validateRouter() error {
	for _, item := range c.Router.HumanConfirmation {
		switch item {
		case DependencyChanged, TargetChangedAgain:
		default:
			return fmt.Errorf("unknown router.human_confirmation category: %q", item)
		}
	}
	return nil
}

func (c Config) validatePlanSettings() error {
	if strings.TrimSpace(c.PlanCheck.Aspect) == "" {
		return fmt.Errorf("plan_check.aspect must not be empty")
	}
	if strings.TrimSpace(c.PlanCheck.Runner) == "" {
		return fmt.Errorf("plan_check.runner must not be empty")
	}
	if strings.TrimSpace(c.DefaultPlan) == "" {
		return fmt.Errorf("default_plan must not be empty")
	}
	if err := c.ValidateCriteria(c.PlanCheck.Criteria); err != nil {
		return fmt.Errorf("plan_check.criteria: %w", err)
	}
	if c.PlanCheck.Examples != nil {
		if err := c.ValidateSelection(*c.PlanCheck.Examples); err != nil {
			return fmt.Errorf("plan_check.examples: %w", err)
		}
	}
	return nil
}

func (c Config) validateApproval() error {
	switch c.Approval.Plan {
	case "requirements", "always", "deviation":
	default:
		return fmt.Errorf("approval.plan must be requirements, always, or deviation")
	}
	switch c.Approval.MetaLoopEdit {
	case "confirm", "allow":
	default:
		return fmt.Errorf("approval.meta_loop_edit must be confirm or allow")
	}
	return nil
}

func (c Config) validateGit() error {
	if !strings.Contains(c.Git.Branch, "{work_id}") || !validBranchName(strings.ReplaceAll(c.Git.Branch, "{work_id}", "work")) {
		return fmt.Errorf("git.branch must contain {work_id} and render a valid git branch name")
	}
	if strings.TrimSpace(c.Git.CommitMessage) == "" {
		return fmt.Errorf("git.commit_message must not be empty")
	}
	placeholders := strings.NewReplacer("{stage}", "", "{work_id}", "", "{task}", "").Replace(c.Git.CommitMessage)
	if strings.ContainsAny(placeholders, "{}") {
		return fmt.Errorf("git.commit_message contains an unknown or malformed placeholder")
	}
	return nil
}

func (c Config) validateDependencyFiles() error {
	for _, pattern := range c.Classification.DependencyFiles {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid classification.dependency_files pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func (c Config) validateContinuationLimit() error {
	if c.Limits.StopContinuation < 1 {
		return fmt.Errorf("limits.stop_continuation must be at least 1")
	}
	return nil
}

func (c Config) validateLessonDestinations() error {
	seen := make(map[string]bool)
	for _, destination := range c.Lessons.Destinations {
		switch destination {
		case "hook_lint_type", "plan_template", "aspect", "principle", "skill":
		default:
			return fmt.Errorf("unknown lessons.destinations value: %q", destination)
		}
		if seen[destination] {
			return fmt.Errorf("duplicate lessons.destinations value: %q", destination)
		}
		seen[destination] = true
	}
	return nil
}

func validRepositoryPath(value string) bool {
	if value == "" || value == "." || path.IsAbs(value) || strings.ContainsAny(value, "\\\x00") || path.Clean(value) != value {
		return false
	}
	if len(value) >= 3 && value[1] == ':' && value[2] == '/' {
		return false
	}
	if slices.Contains(strings.Split(value, "/"), "..") {
		return false
	}
	return value != ".devkit/run" && !strings.HasPrefix(value, ".devkit/run/")
}

func validBranchName(name string) bool {
	if name == "" || name == "HEAD" || strings.HasPrefix(name, "-") || strings.HasSuffix(name, ".") || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.ContainsAny(name, " ~^:?*[\\") {
		return false
	}
	if strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return false
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validateCommand(section, name string, command Command) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s registration name must not be empty", section)
	}
	if strings.TrimSpace(command.Executable) == "" {
		return fmt.Errorf("%s.%s.executable is required", section, name)
	}
	if command.TimeoutSeconds <= 0 || command.TimeoutSeconds > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("%s.%s.timeout_seconds must be a positive number of seconds that fits in time.Duration", section, name)
	}
	return nil
}

func validDenyRule(rule string) bool {
	if strings.HasPrefix(rule, "Bash(") {
		return strings.HasSuffix(rule, " *)") && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), " *)")) != ""
	}
	if strings.HasPrefix(rule, "Agent(") {
		attribute := strings.TrimSuffix(strings.TrimPrefix(rule, "Agent("), ")")
		return strings.HasSuffix(rule, ")") && strings.TrimSpace(attribute) != "" && !strings.ContainsAny(attribute, "()")
	}
	return rule != "" && !strings.ContainsAny(rule, "()") && strings.IndexFunc(rule, unicode.IsSpace) < 0
}
