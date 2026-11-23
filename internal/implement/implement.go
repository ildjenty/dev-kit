package implement

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type unit struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	Criteria     []string `json:"criteria"`
	Done         bool     `json:"done"`
	TestsWritten bool     `json:"tests_written"`
	Passes       int      `json:"passes"`
	Commit       string   `json:"commit,omitempty"`
}

type ownerDecision struct {
	Row      string `json:"row"`
	Question string `json:"question"`
}

type stuckChecks struct {
	Failing     []string `json:"failing"`
	StuckChecks []string `json:"stuck_checks"`
	Results     string   `json:"results"`
}

type state struct {
	Key                 string          `json:"key"`
	RequirementsVersion any             `json:"requirements_version"`
	PlanVersion         any             `json:"plan_version"`
	Round               int             `json:"round"`
	Units               []unit          `json:"units"`
	Stuck               *stuckChecks    `json:"stuck"`
	OwnerDecisions      []ownerDecision `json:"owner_decisions"`
	DecisionErrors      []string        `json:"decision_errors"`
	Exhausted           string          `json:"exhausted,omitempty"`
	BaseFailed          []string        `json:"base_failed,omitempty"`
	FailingLogs         []string        `json:"failing_logs,omitempty"`
	LastFailing         []string        `json:"last_failing,omitempty"`
}

type codeOutput struct {
	OwnerDecisions []ownerDecision `json:"owner_decisions"`
	CommitMessage  any             `json:"commit_message"`
}

type stepBack struct {
	Next        string `json:"next"`
	Premise     string `json:"premise"`
	Observation string `json:"observation"`
}

type mergeResult struct {
	Decisions      []record.Document
	Added, Changed []any
	Owner          []record.Document
}

func documents(value any) []record.Document {
	if docs, ok := value.([]record.Document); ok {
		return docs
	}
	result := []record.Document{}
	items, _ := value.([]any)
	for _, item := range items {
		doc, _ := item.(map[string]any)
		result = append(result, doc)
	}
	return result
}

func text(value any) string { s, _ := value.(string); return s }

func stringsOf(value any) []string {
	if ss, ok := value.([]string); ok {
		return ss
	}
	result := []string{}
	items, _ := value.([]any)
	for _, item := range items {
		result = append(result, text(item))
	}
	return result
}

func clone(doc record.Document) record.Document {
	result := record.Document{}
	for k, v := range doc {
		result[k] = v
	}
	return result
}

func unitsOf(requirements, plan record.Document) []unit {
	result := []unit{}
	if plan != nil {
		for _, i := range documents(plan["increments"]) {
			result = append(result, unit{ID: text(i["id"]), Summary: text(i["summary"]), Criteria: stringsOf(i["criteria"])})
		}
		return result
	}
	return result
}

func inputsKey(requirementsVersion, planVersion any) string {
	plan := "none"
	if planVersion != nil {
		plan = fmt.Sprint(planVersion)
	}
	return fmt.Sprintf("requirements=%v plan=%s", requirementsVersion, plan)
}

func doneUnits(messages []string, key string) map[string]bool {
	done := map[string]bool{}
	trailer := func(message, name string) string {
		match := regexp.MustCompile(`(?m)^` + name + `: (.+)$`).FindStringSubmatch(message)
		if match == nil {
			return ""
		}
		return strings.TrimSpace(match[1])
	}
	for _, message := range messages {
		id := trailer(message, "Archon-Unit")
		if id != "" && trailer(message, "Archon-Inputs") == key {
			done[id] = true
		}
	}
	return done
}

func mergeDecisions(planDecisions, rows []record.Document) *mergeResult {
	result := &mergeResult{Decisions: append([]record.Document{}, planDecisions...), Added: []any{}, Changed: []any{}, Owner: []record.Document{}}
	for _, row := range rows {
		at := slices.IndexFunc(result.Decisions, func(d record.Document) bool { return sameID(d, row) })
		if at >= 0 && equalRows(result.Decisions[at], row) {
			continue
		}
		if row["id"] != nil && row["decided_by"] != nil && (row["decided_by"] != "provisional" || at >= 0 && (result.Decisions[at]["when"] != "later" || row["when"] != "now" || !equalRows(cloneSettlement(result.Decisions[at]), cloneSettlement(row)))) {
			result.Owner = append(result.Owner, row)
			continue
		}
		if at < 0 {
			result.Decisions = append(result.Decisions, row)
			result.Added = append(result.Added, row["id"])
		} else {
			result.Decisions[at] = row
			result.Changed = append(result.Changed, row["id"])
		}
	}

	return result
}

func equalRows(a, b record.Document) bool {
	first, _ := json.Marshal(a)
	second, _ := json.Marshal(b)
	return string(first) == string(second)
}
func cloneSettlement(row record.Document) record.Document {
	out := clone(row)
	for _, k := range []string{"chosen", "evidence", "decided_by", "when", "decide_when"} {
		delete(out, k)
	}
	return out
}
func checkRows(plan record.Document, written json.RawMessage) ([]string, *mergeResult, error) {
	if plan == nil {
		return []string{"Implementation needs a plan"}, nil, nil
	}
	if written == nil {
		return []string{}, nil, nil
	}
	var value any
	if err := json.Unmarshal(written, &value); err != nil {
		return nil, nil, err
	}
	doc, ok := value.(map[string]any)
	rows, array := doc["rows"].([]any)
	if !ok || !array {
		return []string{"decisions.json must be {rows: [...]}"}, nil, nil
	}
	for _, row := range rows {
		if _, ok := row.(map[string]any); !ok {
			return []string{"each decision row must be an object"}, nil, nil
		}
	}
	merged := mergeDecisions(documents(plan["decisions"]), documents(rows))
	errors := []string{}
	seen := []record.Document{}
	twice := []record.Document{}
	for _, row := range documents(rows) {
		id, exists := row["id"]
		matches := func(other record.Document) bool { return sameID(other, row) }
		if slices.ContainsFunc(seen, matches) && !slices.ContainsFunc(twice, matches) {
			label := fmt.Sprint(id)
			if !exists {
				label = "undefined"
			} else if id == nil {
				label = "null"
			}
			errors = append(errors, label+" is written twice")
			twice = append(twice, row)
		}
		seen = append(seen, row)
	}
	for _, row := range merged.Owner {
		id := row["id"]
		if id == nil {
			id = "(no id)"
		}
		errors = append(errors, fmt.Sprint(id)+" needs plan approval; return to plan instead of writing it")
	}
	base := clone(plan)

	base["decisions"] = merged.Decisions
	validation, err := record.Validate("plan", base)
	return append(errors, validation...), merged, err
}

func sameID(a, b record.Document) bool {
	x, xExists := a["id"]
	y, yExists := b["id"]
	return xExists == yExists && (x == nil && y == nil || x != nil && reflect.TypeOf(x).Comparable() && x == y)
}

func readJSON[T any](file string) (T, error) {
	var value T
	data, err := os.ReadFile(file)
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}

func readIf(file string) (json.RawMessage, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value json.RawMessage
	err = json.Unmarshal(data, &value)
	return value, err
}

func writeJSON(file string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0644)
}

func pathAt(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}

func prepare(ctx record.ContextData, artifacts string) (record.Document, error) {
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return nil, err
	}
	if requirements == nil {
		return nil, &record.RecordError{Message: "no requirements are recorded; implement needs them"}
	}
	plan, err := record.Latest(ctx.Dir, "plan")
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("no plan is recorded; implement needs it")
	}
	if plan["requirements_version"] != requirements["version"] {
		return nil, fmt.Errorf("the latest plan (version %v) was made from requirements %v, not the latest %v", plan["version"], plan["requirements_version"], requirements["version"])
	}
	key := inputsKey(requirements["version"], plan["version"])
	repo, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	base, err := repo.MergeBase("")
	if err != nil {
		return nil, err
	}
	if base == "" {
		base = "undefined"
	}
	log, err := repo.Run("log", "--format=%B%x00", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	done := doneUnits(strings.Split(log, "\x00"), key)
	units := unitsOf(requirements, plan)
	remaining := 0
	for i := range units {
		units[i].Done = done[units[i].ID]
		if !units[i].Done {
			remaining++
		}
	}
	dir := filepath.Join(pathAt(ctx.Cwd, artifacts), "implement")
	cfg, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("no request is recorded yet")
	}
	criteriaDir := filepath.Join(artifacts, "criteria")
	if _, err := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: text(requests[0]["criteria_commit"]), Dir: pathAt(ctx.Cwd, criteriaDir)}); err != nil {
		return nil, err
	}
	folder := filepath.Join(criteriaDir, cfg.Standards)
	gitCriteria, err := os.ReadFile(filepath.Join(pathAt(ctx.Cwd, folder), "git.md"))
	if err != nil {
		return nil, err
	}
	prefixes, err := commitPrefixes(string(gitCriteria))
	if err != nil {
		return nil, err
	}
	configuredChecks := []record.Document{}
	allChecks, err := config.ChecksAcrossBase(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return nil, err
	}
	for _, c := range allChecks {
		entry := record.Document{"name": c.Name, "command": c.Command}
		if c.Paths != nil {
			entry["paths"] = c.Paths
		}
		configuredChecks = append(configuredChecks, entry)
	}
	mapping, err := concerns.Load(pathAt(ctx.Cwd, folder))
	if err != nil {
		return nil, err
	}
	returned, err := record.Returned(ctx, "implement")
	if err != nil {
		return nil, err
	}
	input := record.Document{"returned": returned, "language": cfg.Language, "standards": cfg.Standards, "criteria_folder": folder, "checks": configuredChecks, "commit_prefixes": prefixes, "writer_materials": mapping.Materials(folder, "writer:implement", "code"), "test_materials": mapping.Materials(folder, "writer:tests", "code"), "step_back_materials": mapping.Materials(folder, "judge:step-back", "code"), "dispute_materials": mapping.Materials(folder, "judge:dispute", "code")}
	s := state{Key: key, RequirementsVersion: requirements["version"], PlanVersion: plan["version"], Units: units, OwnerDecisions: []ownerDecision{}, DecisionErrors: []string{}}
	for _, item := range []struct {
		name  string
		value any
	}{{"input", input}, {"requirements", requirements}, {"plan", plan}, {"state", s}} {
		if err := writeJSON(filepath.Join(dir, item.name+".json"), item.value); err != nil {
			return nil, err
		}
	}
	return record.Document{"units": len(units), "remaining": remaining, "has_plan": plan != nil}, nil
}

func current(s *state) *unit {
	for i := range s.Units {
		if !s.Units[i].Done {
			return &s.Units[i]
		}
	}
	return nil
}

func next(artifacts string) (record.Document, error) {
	dir := filepath.Join(artifacts, "implement")
	s, err := readJSON[state](filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	u := current(&s)
	if u == nil {
		return nil, fmt.Errorf("every unit is done; the loop should have ended")
	}
	requirements, err := readJSON[record.Document](filepath.Join(dir, "requirements.json"))
	if err != nil {
		return nil, err
	}
	plan, err := readJSON[record.Document](filepath.Join(dir, "plan.json"))
	if err != nil {
		return nil, err
	}
	input, err := readJSON[record.Document](filepath.Join(dir, "input.json"))
	if err != nil {
		return nil, err
	}
	conditions, tests, later := []record.Document{}, []record.Document{}, []string{}
	for _, c := range documents(requirements["completion_conditions"]) {
		if slices.Contains(u.Criteria, text(c["id"])) {
			conditions = append(conditions, c)
		}
	}
	for _, test := range documents(plan["tests"]) {
		if slices.Contains(u.Criteria, text(test["criterion"])) {
			tests = append(tests, test)
		}
	}
	for _, other := range s.Units {
		if !other.Done && other.ID != u.ID {
			later = append(later, other.ID)
		}
	}
	input["id"], input["summary"], input["criteria"], input["tests"], input["later_units"] = u.ID, u.Summary, conditions, tests, later
	if err := writeJSON(filepath.Join(dir, "unit.json"), input); err != nil {
		return nil, err
	}
	return record.Document{"unit": u.ID, "tests_written": u.TestsWritten, "round": s.Round + 1}, nil
}

func testsWritten(artifacts string) (record.Document, error) {
	file := filepath.Join(artifacts, "implement/state.json")
	s, err := readJSON[state](file)
	if err != nil {
		return nil, err
	}
	u := current(&s)
	if u == nil {
		return nil, fmt.Errorf("Cannot set properties of undefined (setting 'tests_written')")
	}
	u.TestsWritten = true
	if err := writeJSON(file, s); err != nil {
		return nil, err
	}
	return record.Document{"unit": u.ID}, nil
}

func commitPrefixes(text string) ([]string, error) {
	matches := regexp.MustCompile("`([a-z][a-z0-9_-]*): ?`").FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		re := regexp.MustCompile(`(?:^|\s)([a-z][a-z0-9_-]*):`)
		for _, at := range re.FindAllStringSubmatchIndex(text, -1) {
			end := at[1]
			if end == len(text) || strings.ContainsRune(" \t\n\r\f\v", rune(text[end])) {
				matches = append(matches, []string{text[at[0]:end], text[at[2]:at[3]]})
			}
		}
	}
	prefixes := []string{}
	for _, match := range matches {
		if !slices.Contains(prefixes, match[1]) {
			prefixes = append(prefixes, match[1])
		}
	}
	if len(prefixes) == 0 {
		return nil, &record.RecordError{Message: "git.md does not name any commit prefixes"}
	}
	return prefixes, nil
}

func firstSentence(text string) string { return strings.TrimSpace(strings.SplitN(text, "\n", 2)[0]) }

func commitSubject(written any, u unit, requirements, plan record.Document, prefixes []string) string {
	line := strings.TrimSpace(text(written))
	match := regexp.MustCompile(`(?i)^([a-z][a-z0-9_-]*): `).FindStringSubmatch(line)
	if match != nil {
		body := line[len(match[1])+2:]
		ids := append([]string{u.ID}, u.Criteria...)
		namesID := false
		for _, word := range regexp.MustCompile(`[\s:]+`).Split(body, -1) {
			if slices.Contains(ids, word) {
				namesID = true
			}
		}
		if slices.Contains(prefixes, match[1]) && !strings.Contains(line, "\n") && strings.TrimSpace(body) != "" && !namesID {
			return line
		}
	}
	goal := firstSentence(text(requirements["goal"]))
	if plan != nil {
		goal += "; " + firstSentence(u.Summary)
	}
	return prefixes[0] + ": " + goal
}

func end(ctx record.ContextData, artifacts string, gate *checks.Gate, code *codeOutput) (record.Document, error) {
	dir := filepath.Join(artifacts, "implement")
	file := filepath.Join(dir, "state.json")
	s, err := readJSON[state](file)
	if err != nil {
		return nil, err
	}
	u := current(&s)
	if gate != nil && gate.OnlyPreexisting() {
		s.BaseFailed = gate.Preexisting
		s.FailingLogs, err = gate.FailingLogs()
		if err != nil {
			return nil, err
		}
		if err := writeJSON(file, s); err != nil {
			return nil, err
		}
		return record.Document{"unit": u.ID, "committed": false, "all_done": false, "finished": true, "needs_owner": 0, "stuck": false, "decision_errors": 0}, nil
	}
	s.Round++
	if u != nil {
		u.Passes++
	}
	plan, err := readJSON[record.Document](filepath.Join(dir, "plan.json"))
	if err != nil {
		return nil, err
	}
	written, err := readIf(filepath.Join(dir, "decisions.json"))
	if err != nil {
		return nil, err
	}
	errors, merged, err := checkRows(plan, written)
	if err != nil {
		return nil, err
	}
	s.DecisionErrors = errors
	if err := writeJSON(filepath.Join(dir, "decision-errors.json"), record.Document{"errors": errors}); err != nil {
		return nil, err
	}
	s.OwnerDecisions = []ownerDecision{}
	if code != nil && code.OwnerDecisions != nil {
		s.OwnerDecisions = code.OwnerDecisions
	}
	if merged != nil {
		for _, row := range merged.Owner {
			s.OwnerDecisions = append(s.OwnerDecisions, ownerDecision{Row: text(row["id"]), Question: fmt.Sprintf("%s: %s", text(row["what"]), text(row["chosen"]))})
		}
	}
	if gate != nil && gate.Stuck {
		s.Stuck = &stuckChecks{gate.Failing, gate.StuckChecks, gate.Results}
	}
	committed := false
	if gate != nil && gate.Passed && len(errors) == 0 && len(s.OwnerDecisions) == 0 {
		if _, err := gitx.Run(ctx.Cwd, "add", "-A"); err != nil {
			return nil, err
		}
		requirements, err := readJSON[record.Document](filepath.Join(dir, "requirements.json"))
		if err != nil {
			return nil, err
		}
		input, err := readJSON[record.Document](filepath.Join(dir, "input.json"))
		if err != nil {
			return nil, err
		}
		var message any
		if code != nil {
			message = code.CommitMessage
		}
		if u == nil {
			return nil, fmt.Errorf("Cannot read properties of undefined (reading 'id')")
		}
		subject := commitSubject(message, *u, requirements, plan, stringsOf(input["commit_prefixes"]))
		message = fmt.Sprintf("%s\n\nArchon-Run: %s\nArchon-Unit: %s\nArchon-Inputs: %s\n", subject, ctx.RunID, u.ID, s.Key)
		if _, err := gitx.Run(ctx.Cwd, "commit", "--allow-empty", "-q", "-m", text(message)); err != nil {
			return nil, err
		}
		u.Done = true
		u.Commit, err = gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		committed = true
	}
	if gate != nil && !gate.Passed {
		s.LastFailing = gate.Failing
	}
	if !committed && u != nil && s.Stuck == nil && len(s.OwnerDecisions) == 0 {
		passes := 3
		if c, err := config.Load(ctx.Cwd); err == nil && c.Limits.Passes > 0 {
			passes = c.Limits.Passes
		}
		if u.Passes >= passes {
			s.Exhausted = u.ID
		}
	}
	if err := writeJSON(file, s); err != nil {
		return nil, err
	}
	allDone := current(&s) == nil
	if u == nil {
		return nil, fmt.Errorf("Cannot read properties of undefined (reading 'id')")
	}
	return record.Document{"unit": u.ID, "committed": committed, "all_done": allDone, "finished": allDone || s.Stuck != nil || len(s.OwnerDecisions) > 0 || s.Exhausted != "", "needs_owner": len(s.OwnerDecisions), "stuck": s.Stuck != nil, "decision_errors": len(errors)}, nil
}

func outcome(s state, plan record.Document, written json.RawMessage, back *stepBack) (record.Document, []record.Document, error) {
	ended := "max_iterations"
	if current(&s) == nil {
		ended = "cleared"
	} else if len(s.BaseFailed) > 0 {
		ended = "base_failed"
	} else if s.Stuck != nil {
		ended = "stuck"
	}
	loops := []record.Document{{"name": "implement", "ended": ended, "rounds": s.Round}}
	stop := func(message string, unresolved any, premise string) record.Document {
		return record.Document{"outcome": "stop", "loops": loops, "stop": record.Document{"state": message, "unresolved": unresolved, "premise": premise}}
	}
	if len(s.BaseFailed) > 0 {
		result := stop(checks.BaseMessage(s.BaseFailed), s.FailingLogs, "")
		result["stop"].(record.Document)["base_failed"] = true
		return result, nil, nil
	}
	if len(s.OwnerDecisions) > 0 {
		rows := []record.Document{}
		for _, d := range s.OwnerDecisions {
			rows = append(rows, record.Document{"id": d.Row, "what": d.Question, "decided_by": "owner"})
		}
		result := record.BackToPlan(rows)
		result["loops"] = loops
		return result, nil, nil
	}
	if s.Stuck != nil {
		if back != nil && (back.Next == "requirements" || back.Next == "plan") {
			return record.Document{"outcome": "back", "loops": loops, "back": record.Document{"to": back.Next, "premise": back.Premise, "observation": back.Observation}}, nil, nil
		}
		message, premise := "The same check failed in consecutive passes", ""
		if back != nil {
			premise = back.Premise + "\n" + back.Observation
			if back.Next == "environment" {
				message = "A check failed because of the environment rather than the change"
			}
		}
		unresolved := s.Stuck.StuckChecks
		if len(unresolved) == 0 {
			unresolved = s.Stuck.Failing
		}
		return stop(message, unresolved, premise), nil, nil
	}
	if s.Exhausted != "" {
		unresolved := []string{}
		for _, u := range s.Units {
			if u.ID == s.Exhausted {
				unresolved = append(unresolved, fmt.Sprintf("%s: %s (passes: %d)", u.ID, u.Summary, u.Passes))
			}
		}
		for _, f := range s.LastFailing {
			unresolved = append(unresolved, "failing check: "+f)
		}
		return stop(fmt.Sprintf("Release step %s did not pass its checks within its pass limit (limits.passes)", s.Exhausted), append(unresolved, s.DecisionErrors...), ""), nil, nil
	}
	remaining := []string{}
	for _, u := range s.Units {
		if !u.Done {
			remaining = append(remaining, u.ID+": "+u.Summary)
		}
	}
	if len(remaining) > 0 {
		return stop(fmt.Sprintf("Units remain after the implementation loop limit (%d passes)", s.Round), append(remaining, s.DecisionErrors...), ""), nil, nil
	}
	errors, merged, err := checkRows(plan, written)
	if err != nil {
		return nil, nil, err
	}
	if merged != nil && len(merged.Owner) > 0 {
		rows := []record.Document{}
		for _, row := range merged.Owner {
			rows = append(rows, record.Document{"id": row["id"], "what": fmt.Sprintf("%s: %s", text(row["what"]), text(row["chosen"])), "decided_by": "owner"})
		}
		result := record.BackToPlan(rows)
		result["loops"] = loops
		return result, nil, nil
	}
	if len(errors) > 0 {
		return stop("The decision rows are invalid", errors, ""), nil, nil
	}
	touched := []any{}
	if merged != nil {
		touched = append(touched, merged.Added...)
		touched = append(touched, merged.Changed...)
	}

	decisions := documents(plan["decisions"])
	if merged != nil {
		decisions = merged.Decisions
	}
	later := []string{}
	for _, d := range decisions {
		if d["when"] == "later" {
			later = append(later, fmt.Sprintf("%v: %v(%v)", d["id"], d["what"], d["decide_when"]))
		}
	}
	if len(later) > 0 {
		return stop("Deferred decisions remain after implementation", later, ""), nil, nil
	}
	if len(touched) == 0 {
		decisions = nil
	}
	return record.Document{"outcome": "pass", "loops": loops}, decisions, nil
}

func finish(ctx record.ContextData, artifacts string, back *stepBack) (record.Document, error) {
	dir := filepath.Join(artifacts, "implement")
	s, err := readJSON[state](filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	plan, err := readJSON[record.Document](filepath.Join(dir, "plan.json"))
	if err != nil {
		return nil, err
	}
	written, err := readIf(filepath.Join(dir, "decisions.json"))
	if err != nil {
		return nil, err
	}
	doc, decisions, err := outcome(s, plan, written, back)
	if err != nil {
		return nil, err
	}
	if decisions != nil {
		body := clone(plan)
		body["decisions"] = decisions
		body["derived_from"] = plan["version"]
		for _, k := range []string{"version", "requirements_version", "run_id", "recorded_at"} {
			delete(body, k)
		}
		if err := writeJSON(filepath.Join(dir, "plan-next.json"), body); err != nil {
			return nil, err
		}
		if _, err := record.Write(ctx, "plan", body, ""); err != nil {
			return nil, err
		}
	}
	if err := writeJSON(filepath.Join(dir, "outcome.json"), doc); err != nil {
		return nil, err
	}
	repo, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	base, err := repo.MergeBase("")
	if err != nil {
		return nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	return record.WriteResult(ctx.Dir, "implement", doc, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head})
}

func parseEnv[T any](value string) (*T, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var out *T
	err := json.Unmarshal([]byte(value), &out)
	return out, err
}

func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"prepare", "next", "tests-written", "end", "state", "finish"} {
		commands = append(commands, cli.Command{Area: "implement", Name: mode, Usage: "devkit implement " + mode, Run: func(args []string, env cli.Env) error {
			artifacts := env.Getenv("ARTIFACTS_DIR")
			if artifacts == "" {
				return fmt.Errorf("ARTIFACTS_DIR must be set")
			}
			var out record.Document
			var err error
			if mode == "next" {
				out, err = next(env.Path(artifacts))
			} else if mode == "tests-written" {
				out, err = testsWritten(env.Path(artifacts))
			} else if mode == "state" {
				var s state
				s, err = readJSON[state](filepath.Join(env.Path(artifacts), "implement/state.json"))
				out = record.Document{"stuck": s.Stuck != nil && len(s.OwnerDecisions) == 0}
			} else {
				ctx, contextErr := record.Context(env)
				if contextErr != nil {
					return contextErr
				}
				ctx.Dir = env.Path(ctx.Dir)
				switch mode {
				case "prepare":
					out, err = prepare(ctx, artifacts)
				case "end":
					gate, parseErr := parseEnv[checks.Gate](env.Getenv("GATE"))
					if parseErr != nil {
						return parseErr
					}
					code, parseErr := parseEnv[codeOutput](env.Getenv("CODE"))
					if parseErr != nil {
						return parseErr
					}
					out, err = end(ctx, env.Path(artifacts), gate, code)
				case "finish":
					back, parseErr := parseEnv[stepBack](env.Getenv("STEP_BACK"))
					if parseErr != nil {
						return parseErr
					}
					out, err = finish(ctx, env.Path(artifacts), back)
				}
			}
			if err != nil {
				return err
			}
			return env.JSON(out)
		}})
	}
	return commands
}
