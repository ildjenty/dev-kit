package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// pattern reads and parses a loop pattern, a reference that never constrains task plans.
func (e *Engine) pattern(ctx context.Context, ref, name string) (plan.PlanTemplate, []byte, error) {
	if !localName(name) {
		return plan.PlanTemplate{}, nil, errors.New("invalid pattern name")
	}
	source, err := e.asset(ctx, ref, "plans/"+name+".json")
	if err != nil {
		return plan.PlanTemplate{}, nil, err
	}
	p, err := e.plans.ParseTemplate(source)
	if err == nil && p.Name != name {
		err = errors.New("pattern name does not match its file name")
	}
	return p, source, err
}

// loopRules converts the configured loop rules and ceilings for plan validation.
func loopRules(settings config.Config) (plan.LoopRules, plan.Limits) {
	rules := plan.LoopRules{Stage: []plan.StageRule{}, Path: []plan.PathRule{}}
	for _, rule := range settings.Loop.Stage {
		item := plan.StageRule{Definition: slices.Clone(rule.Definition), Checks: slices.Clone(rule.Checks), Gate: rule.Gate,
			Roles: slices.Clone(rule.Roles), History: plan.History(rule.History), Outputs: rule.Outputs}
		if rule.MaxRounds != nil {
			item.MaxRounds = *rule.MaxRounds
		}
		if rule.MaxNoProgress != nil {
			item.MaxNoProgress = *rule.MaxNoProgress
		}
		if rule.Report != nil {
			item.Report = &plan.Selection{Source: rule.Report.Source, Select: rule.Report.Select}
		}
		rules.Stage = append(rules.Stage, item)
	}
	for _, rule := range settings.Loop.Path {
		rules.Path = append(rules.Path, plan.PathRule{Require: slices.Clone(rule.Require), Definition: rule.Definition, Follows: rule.Follows, First: rule.First, Last: rule.Last})
	}
	rounds, noProgress := settings.StageLimitCeilings()
	return rules, plan.Limits{MaxRounds: rounds, MaxNoProgress: noProgress}
}

// resolver resolves selections at the fixed reference. Products outside the norms, such as
// requirements.md and structure.json, may not exist before the work creates them.
func (e *Engine) resolver(ctx context.Context, ref string) (plan.Resolver, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	return func(selection plan.Selection) ([]string, error) {
		item, ok := sources[selection.Source]
		if !ok {
			return nil, fmt.Errorf("unknown source: %q", selection.Source)
		}
		documents, err := e.selectDocuments(ctx, ref, selection.Source, selection.Select)
		if err != nil {
			return nil, err
		}
		bodies := []string{}
		for _, document := range documents {
			bodies = append(bodies, document.Body)
		}
		if len(bodies) == 0 && !item.Norm {
			bodies = append(bodies, "")
		}
		return bodies, nil
	}, nil
}

// validateProposal applies devkit's structural acceptance checks to a schema-valid plan: roles,
// references, instance IDs, back and return targets, stage rules, and path rules over the work path.
func (e *Engine) validateProposal(ctx context.Context, state *ledger.State, p plan.Plan, ref string) error {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return err
	}
	if err := validateRoles(p.Stages, settings); err != nil {
		return err
	}
	resolve, err := e.resolver(ctx, ref)
	if err != nil {
		return err
	}
	if err := plan.ValidateReferences(p, checkCatalog(settings), settings.DetectionRequired(), resolve); err != nil {
		return err
	}
	if err := e.validateInstanceIDs(state, p); err != nil {
		return err
	}
	violations, err := e.ruleViolations(state, p, settings)
	if err != nil {
		return err
	}
	return violationError(violations)
}

// validateInstanceIDs keeps reused instance IDs bound to the definition they ran with, and
// limits back targets to current instances and instances that ran in this work.
func (e *Engine) validateInstanceIDs(state *ledger.State, p plan.Plan) error {
	ran, err := e.ranDefinitions(state)
	if err != nil {
		return err
	}
	runIDs := map[string]bool{}
	for id := range ran {
		runIDs[id] = true
	}
	if err := plan.ValidateBackTargets(p, runIDs); err != nil {
		return err
	}
	for _, stage := range p.Stages {
		if definition, ok := ran[stage.ID]; ok && definition != stage.DefinitionID() {
			return fmt.Errorf("%s: instance ID ran with definition %q; use a new ID for new work", stage.ID, definition)
		}
	}
	target, err := e.pendingBackTarget(state)
	if err != nil {
		return err
	}
	return plan.ValidateReturnTarget(p, target)
}

// ranDefinitions maps every instance ID that ran in this work to its bound definition.
func (e *Engine) ranDefinitions(state *ledger.State) (map[string]string, error) {
	ran := map[string]string{}
	for _, run := range state.StageRuns() {
		bound, ok, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return nil, err
		}
		definition := run.StageID
		if stage, found := bound.Stage(run.StageID); ok && found {
			definition = stage.DefinitionID()
		}
		ran[run.StageID] = definition
	}
	return ran, nil
}

// pendingBackTarget returns the return instance of a back decision that has not restarted yet.
func (e *Engine) pendingBackTarget(state *ledger.State) (string, error) {
	run, ok := state.LatestStageRun()
	if !ok || run.State != ledger.StageInvalidated || !backDecided(state, run) {
		return "", nil
	}
	bound, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil || !ok {
		return "", err
	}
	target, err := bound.Next(run.StageID, plan.Back)
	if err != nil || target == nil {
		return "", err
	}
	return *target, nil
}

func backDecided(state *ledger.State, run *ledger.StageRun) bool {
	rounds := state.Rounds(run.ID)
	return len(rounds) > 0 && rounds[len(rounds)-1].Decision != nil && rounds[len(rounds)-1].Decision.Action == plan.Back
}

// ruleViolations evaluates the fixed stage rules on the plan and the path rules on the work path:
// the adopted completed instances followed by the plan from where the work continues.
func (e *Engine) ruleViolations(state *ledger.State, p plan.Plan, settings config.Config) ([]plan.RuleViolation, error) {
	completed, err := e.completedStages(state)
	if err != nil {
		return nil, err
	}
	start, err := e.pathStart(state, p)
	if err != nil {
		return nil, err
	}
	rules, ceiling := loopRules(settings)
	path := p
	path.StartStage = start
	violations := plan.ValidateStageRules(p, rules.Stage, ceiling)
	return append(violations, plan.EvaluatePath(completed, path, rules.Path)...), nil
}

// completedStages returns the adopted completed instances in order with their bound stages.
func (e *Engine) completedStages(state *ledger.State) ([]plan.Stage, error) {
	runs, err := e.adoptedRuns(state)
	if err != nil {
		return nil, err
	}
	stages := []plan.Stage{}
	for _, run := range runs {
		bound, ok, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return nil, err
		}
		stage, found := bound.Stage(run.StageID)
		if !ok || !found {
			return nil, fmt.Errorf("bound stage %q is missing from plan %d", run.StageID, run.PlanVersion)
		}
		stages = append(stages, stage)
	}
	return stages, nil
}

// pathStart returns the instance of p where the work continues: the start stage, the return
// instance of a back decision, the instance after a completed one, or the active instance.
func (e *Engine) pathStart(state *ledger.State, p plan.Plan) (string, error) {
	run, ok := state.LatestStageRun()
	if !ok {
		return p.StartStage, nil
	}
	action := plan.Done
	switch {
	case run.State == ledger.StageInvalidated && backDecided(state, run):
		action = plan.Back
	case run.State != ledger.StageCompleted:
		return run.StageID, nil
	}
	bound, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("bound plan is missing")
	}
	target, err := bound.Next(run.StageID, action)
	if err != nil {
		return "", err
	}
	if target == nil || *target == "replan" {
		return p.StartStage, nil
	}
	return *target, nil
}

// approvalCompleted returns the adopted completed instances that ran under plan versions before
// the latest proposal. They are removed from the last approved plan before comparing structure.
func (e *Engine) approvalCompleted(state *ledger.State) (map[string]bool, error) {
	runs, err := e.adoptedRuns(state)
	if err != nil {
		return nil, err
	}
	latest := 0
	for _, version := range state.PlanEvaluations(nil) {
		latest = max(latest, version.Plan.Version)
	}
	completed := map[string]bool{}
	for _, run := range runs {
		if run.PlanVersion < latest {
			completed[run.StageID] = true
		}
	}
	return completed, nil
}

// violationError joins rule violations into one rejection reason.
func violationError(violations []plan.RuleViolation) error {
	if len(violations) == 0 {
		return nil
	}
	return errors.New("plan violates the loop rules:\n" + violationText(violations))
}

func violationText(violations []plan.RuleViolation) string {
	lines := []string{}
	for _, violation := range violations {
		rule := "limit ceilings"
		if violation.Rule >= 0 {
			rule = fmt.Sprintf("rule %d", violation.Rule)
		}
		if violation.StageID != "" {
			rule += ", instance " + violation.StageID
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", rule, violation.Message))
	}
	return strings.Join(lines, "\n")
}

// plannerCatalogs fills the planner lists with fixed stage paths, pattern paths and hashes,
// roles, loop rules, and check names with one-line descriptions.
func (e *Engine) plannerCatalogs(ctx context.Context, ref string, settings config.Config, input *brief.PlannerInput) error {
	var err error
	if input.StageCatalog, input.StageDefinitions, err = e.stageCatalog(ctx, ref); err != nil {
		return err
	}
	if input.Patterns, err = e.sourcePaths(ctx, ref, "plans", func(id string) bool { return strings.HasSuffix(id, ".json") }); err != nil {
		return err
	}
	for _, id := range slices.Sorted(maps.Keys(settings.RoleSettings())) {
		role := settings.RoleSettings()[id]
		input.Roles = append(input.Roles, brief.RoleEntry{ID: id, Kind: role.Kind, Agent: role.Agent})
	}
	rules, ceiling := loopRules(settings)
	input.LoopRules = brief.LoopRules{Stage: rules.Stage, Path: rules.Path, Ceilings: ceiling}
	catalog := checkCatalog(settings)
	for _, aspect := range slices.Sorted(maps.Keys(catalog)) {
		description := settings.Checks[aspect].Description
		if description == "" {
			description, err = e.checkDescription(ctx, ref, aspect, catalog[aspect])
			if err != nil {
				return err
			}
		}
		input.Checks = append(input.Checks, brief.CheckEntry{Aspect: aspect, Description: description})
	}
	return nil
}

// stageCatalog returns the stages/README.md list of definition IDs and names, or the definition
// paths without the README, and the fixed paths of the definition documents.
func (e *Engine) stageCatalog(ctx context.Context, ref string) (string, []brief.NormPath, error) {
	paths, err := e.sourcePaths(ctx, ref, "stages", func(id string) bool { return strings.HasSuffix(id, ".md") && id != "README.md" })
	if err != nil {
		return "", nil, err
	}
	lines := []string{}
	readme, err := e.document(ctx, ref, "stages", "README.md")
	if errors.Is(err, fs.ErrNotExist) {
		for _, path := range paths {
			lines = append(lines, "- `"+path.Path+"`")
		}
		return strings.Join(lines, "\n"), paths, nil
	}
	if err != nil {
		return "", nil, err
	}
	for _, line := range strings.Split(readme.Body, "\n") {
		if strings.HasPrefix(line, "- `stages/") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), paths, nil
}

// sourcePaths lists the selected documents of a source with their fixed snapshot paths.
func (e *Engine) sourcePaths(ctx context.Context, ref, name string, keep func(string) bool) ([]brief.NormPath, error) {
	documents, err := e.selectDocuments(ctx, ref, name, "")
	if err != nil {
		return nil, err
	}
	directory, err := e.snapshotDirectory(ctx, ref, name)
	if err != nil {
		return nil, err
	}
	paths := []brief.NormPath{}
	for _, document := range documents {
		if keep(document.ID) {
			paths = append(paths, brief.NormPath{Path: name + "/" + document.ID, SnapshotPath: filepath.Join(directory, filepath.FromSlash(document.ID)), ContentHash: plan.ContentHash([]byte(document.Body))})
		}
	}
	slices.SortFunc(paths, func(a, b brief.NormPath) int { return strings.Compare(a.Path, b.Path) })
	return paths, nil
}

// snapshotDirectory returns the fixed directory holding a source's documents for ref.
func (e *Engine) snapshotDirectory(ctx context.Context, ref, name string) (string, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return "", err
	}
	if location, inside := e.repositoryLocation(sources[name]); inside {
		expanded, err := e.expanded(ctx, ref)
		return filepath.Join(expanded, filepath.FromSlash(location)), err
	}
	pin, err := e.pin(ctx, ref, false)
	if err != nil {
		return "", err
	}
	return e.fixedDirectory(name, pin[name])
}
