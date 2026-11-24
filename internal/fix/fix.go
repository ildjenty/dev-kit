package fix

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type prepared struct {
	Ready        bool   `json:"ready"`
	Reason       string `json:"reason"`
	Findings     int    `json:"findings"`
	Rounds       int    `json:"rounds"`
	LimitReached bool   `json:"limit_reached"`
	Limit        int    `json:"limit"`
}

type preparedFile struct {
	prepared
	FindingsList []record.Document `json:"findings_list"`
}

type objections struct {
	Accepted []record.Document `json:"accepted"`
	Rejected []record.Document `json:"rejected"`
}

type disputeRound struct {
	Round     int               `json:"round"`
	Decisions []record.Document `json:"decisions"`
}

type state struct {
	FailingLogs         []string          `json:"failing_logs,omitempty"`
	Review              string            `json:"review,omitempty"`
	OutsideRequirements []record.Document `json:"outside_requirements,omitempty"`
	Start               string            `json:"start"`
	ChangeID            *string           `json:"change_id"`
	RequirementsVersion any               `json:"requirements_version"`
	PlanVersion         any               `json:"plan_version"`
	Round               int               `json:"round"`
	Current             []record.Document `json:"current"`
	Disputes            []disputeRound    `json:"disputes"`
	Objections          objections        `json:"objections"`
	Pending             int               `json:"pending"`
	Gate                *checks.Gate      `json:"gate"`
	OwnerDecisions      []record.Document `json:"owner_decisions"`
	BrokenDecisions     []record.Document `json:"broken_decisions"`
}

type codeOutput struct {
	OutsideRequirements []record.Document `json:"outside_requirements"`
	OwnerDecisions      []record.Document `json:"owner_decisions"`
	BrokenDecisions     []record.Document `json:"broken_decisions"`
	CommitMessage       any               `json:"commit_message"`
}

type stepBack struct {
	Next        string `json:"next"`
	Premise     string `json:"premise"`
	Observation string `json:"observation"`
}

func text(value any) string            { s, _ := value.(string); return s }
func boolean(value any) bool           { b, _ := value.(bool); return b }
func object(value any) record.Document { doc, _ := value.(map[string]any); return doc }
func documents(value any) []record.Document {
	if docs, ok := value.([]record.Document); ok {
		return docs
	}
	result := []record.Document{}
	items, _ := value.([]any)
	for _, item := range items {
		result = append(result, object(item))
	}
	return result
}
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

func findingsOn(judgments []record.Document, commit, runID string) (bool, []record.Document) {
	on := []record.Document{}
	latestRun := runID
	judged := false
	accepted := map[string]bool{}
	for _, j := range judgments {
		if j["kind"] != "code" || object(j["target"])["commit"] != commit {
			continue
		}
		on = append(on, j)
		if j["judge"] != "dispute" {
			judged = true
			if runID == "" {
				latestRun = text(j["run_id"])
			}
		} else {
			for _, f := range documents(j["findings"]) {
				if !boolean(f["blocking"]) {
					accepted[text(f["id"])] = true
				}
			}
		}
	}
	findings := []record.Document{}
	for _, j := range on {
		if j["judge"] == "dispute" || j["run_id"] != latestRun {
			continue
		}
		for _, f := range documents(j["findings"]) {
			if !boolean(f["blocking"]) || boolean(f["needs_decision"]) || boolean(f["deferred"]) {
				continue
			}
			id := text(j["run_id"]) + "/" + text(j["judge"]) + "/" + text(f["id"])
			if !accepted[id] {
				findings = append(findings, record.Document{"id": id, "judge": j["judge"], "text": f["text"], "evidence": f["evidence"]})
			}
		}
	}
	return judged, findings
}

func fixesSinceImplement(results []record.Document) []record.Document {
	last := -1
	for i, r := range results {
		if r["workflow"] == "implement" {
			last = i
		}
	}
	fixes := []record.Document{}
	for _, r := range results[last+1:] {
		if r["workflow"] == "fix" {
			fixes = append(fixes, r)
		}
	}
	return fixes
}

func checkObjections(written record.Document, findings []record.Document, decided map[string]bool) ([]record.Document, []record.Document) {
	valid, invalid := []record.Document{}, []record.Document{}
	for _, o := range documents(written["objections"]) {
		id := text(o["finding"])
		at := slices.IndexFunc(findings, func(f record.Document) bool { return f["id"] == id })
		reason := ""
		switch {
		case at < 0:
			reason = "names no finding of this fix"
		case strings.TrimSpace(text(o["evidence"])) == "":
			reason = "gives no evidence"
		case decided[id]:
			reason = "the dispute judge already decided an objection to this finding"
		case slices.ContainsFunc(valid, func(v record.Document) bool { return v["finding"] == id }):
			reason = "a second objection to the same finding"
		}
		if reason != "" {
			rejected := clone(o)
			rejected["reason"] = reason
			invalid = append(invalid, rejected)
		} else {
			finding := clone(findings[at])
			finding["finding"], finding["finding_evidence"], finding["evidence"] = id, findings[at]["evidence"], o["evidence"]
			valid = append(valid, finding)
		}
	}
	return valid, invalid
}

func checkDecisions(objections, decisions []record.Document) error {
	want, got := []string{}, []string{}
	for _, o := range objections {
		want = append(want, text(o["finding"]))
	}
	for _, d := range decisions {
		got = append(got, text(d["finding"]))
	}
	sort.Strings(want)
	sort.Strings(got)
	if !slices.Equal(want, got) {
		names := strings.Join(got, ", ")
		if names == "" {
			names = "nothing"
		}
		return fmt.Errorf("the dispute judge decided %s, but the objections were %s", names, strings.Join(want, ", "))
	}
	for _, d := range decisions {
		if (d["decision"] != "accepted" && d["decision"] != "rejected") || strings.TrimSpace(text(d["reason"])) == "" {
			return fmt.Errorf("the decision on %v needs accepted or rejected and a reason", d["finding"])
		}
	}
	return nil
}

func disputeJudgment(round disputeRound, target record.Document) record.Document {
	observations, findings := []string{}, []record.Document{}
	verdict := "pass"
	for _, d := range round.Decisions {
		observations = append(observations, stringsOf(d["observations"])...)
		if d["decision"] == "rejected" {
			verdict = "fail"
		}
		findings = append(findings, record.Document{"id": d["finding"], "blocking": d["decision"] == "rejected", "needs_decision": false, "text": d["reason"], "evidence": strings.Join(stringsOf(d["observations"]), "\n")})
	}
	return record.Document{"judge": "dispute", "kind": "code", "target": target, "materials": []string{"findings of the code judgments on the commit", fmt.Sprintf("objections of fix round %d", round.Round)}, "observations": observations, "verdict": verdict, "findings": findings}
}

func outcome(p preparedFile, s state, back *stepBack) record.Document {
	loops := []record.Document{}
	stop := func(message string, unresolved []string, premise string) record.Document {
		return record.Document{"outcome": "stop", "loops": loops, "stop": record.Document{"state": message, "unresolved": unresolved, "premise": premise}}
	}
	if !p.Ready {
		return stop(p.Reason, []string{}, "")
	}
	if p.LimitReached {
		loops = append(loops, record.Document{"name": "fix", "ended": "max_iterations", "rounds": p.Rounds + 1})
		unresolved := []string{}
		for _, f := range p.FindingsList {
			unresolved = append(unresolved, text(f["id"])+": "+text(f["text"]))
		}
		return stop(fmt.Sprintf("Fixes since the last implementation reached the configured limit (%d)", p.Limit), unresolved, "")
	}
	if s.Gate != nil && s.Gate.OnlyPreexisting() {
		loops = append(loops, record.Document{"name": "fix-checks", "ended": "base_failed", "rounds": s.Round})
		result := stop(checks.BaseMessage(s.Gate.Preexisting), s.FailingLogs, "")
		result["stop"].(record.Document)["base_failed"] = true
		return result
	}
	ended := "max_iterations"
	if s.Gate != nil && s.Gate.Passed && s.Pending == 0 {
		ended = "cleared"
	} else if s.Gate != nil && s.Gate.Stuck {
		ended = "stuck"
	}
	loops = append(loops, record.Document{"name": "fix-checks", "ended": ended, "rounds": s.Round})
	if len(s.OutsideRequirements) > 0 {
		items := []string{}
		for _, item := range s.OutsideRequirements {
			items = append(items, text(item["item"])+": "+text(item["reason"]))
		}
		return record.Document{"outcome": "back", "loops": loops, "outside_requirements": s.OutsideRequirements, "back": record.Document{"to": "requirements", "premise": strings.Join(items, "\n"), "observation": s.Review}}
	}
	if len(s.OwnerDecisions) > 0 {
		rows := []record.Document{}
		for _, d := range s.OwnerDecisions {
			rows = append(rows, record.Document{"id": d["ref"], "what": d["question"], "decided_by": "owner"})
		}
		result := record.BackToPlan(rows)
		result["loops"] = loops
		return result
	}
	if (s.Gate != nil && s.Gate.Stuck) || len(s.BrokenDecisions) > 0 {
		if back != nil && slices.Contains([]string{"requirements", "plan", "implement"}, back.Next) {
			return record.Document{"outcome": "back", "loops": loops, "back": record.Document{"to": back.Next, "premise": back.Premise, "observation": back.Observation}}
		}
		message, premise := "A plan decision does not hold during the fix", ""
		if s.Gate != nil && s.Gate.Stuck {
			message = "The same check failed in consecutive passes"
		}
		if back != nil {
			premise = back.Premise + "\n" + back.Observation
			if back.Next == "environment" {
				message = "A check failed because of the environment rather than the change"
			}
		}
		unresolved := []string{}
		if s.Gate != nil {
			unresolved = append(unresolved, s.Gate.StuckChecks...)
		}
		for _, b := range s.BrokenDecisions {
			unresolved = append(unresolved, fmt.Sprintf("%v: %v", b["row"], b["observation"]))
		}
		return stop(message, unresolved, premise)
	}
	if s.Gate == nil || !s.Gate.Passed || s.Pending > 0 {
		unresolved := []string{}
		if s.Gate != nil {
			unresolved = append(unresolved, s.Gate.Failing...)
		}
		for _, r := range s.Objections.Rejected {
			unresolved = append(unresolved, text(r["finding"]))
		}
		return stop("Checks or findings with rejected objections remain after the fix loop limit", unresolved, "")
	}
	return record.Document{"outcome": "pass", "loops": loops}
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

func prepare(ctx record.ContextData, artifacts string) (prepared, error) {
	dir := filepath.Join(pathAt(ctx.Cwd, artifacts), "fix")
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return prepared{}, err
	}
	var plan record.Document
	if requirements != nil {
		plan, err = record.Latest(ctx.Dir, "plan")
		if err != nil {
			return prepared{}, err
		}
	}
	if plan != nil && plan["requirements_version"] != requirements["version"] {
		return prepared{}, fmt.Errorf("the latest plan (version %v) was made from requirements %v, not the latest %v", plan["version"], plan["requirements_version"], requirements["version"])
	}
	cfg, err := config.Load(ctx.Cwd)
	if err != nil {
		return prepared{}, err
	}
	repo, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return prepared{}, err
	}
	head, err := repo.Head()
	if err != nil {
		return prepared{}, err
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return prepared{}, err
	}
	judged, findings := findingsOn(judgments, head, "")
	results, err := record.List(ctx.Dir, "result")
	if err != nil {
		return prepared{}, err
	}
	runs, err := record.List(ctx.Dir, "run")
	if err != nil {
		return prepared{}, err
	}
	review := ""
	for _, run := range runs {
		if run["run_id"] == ctx.RunID {
			review = text(object(run["review"])["text"])
		}
	}
	if review != "" {
		judged = true
		findings = []record.Document{{"id": ctx.RunID + "/review", "source": "review", "text": review, "evidence": review}}
	}
	fixes := fixesSinceImplement(results)
	reason := ""
	if !judged {
		reason = fmt.Sprintf("No code judgment is bound to HEAD (%s)", head)
	} else if len(findings) == 0 {
		reason = fmt.Sprintf("The judgments on HEAD (%s) have no findings to fix", head)
	}
	p := prepared{Ready: reason == "", Reason: reason, Findings: len(findings), Rounds: len(fixes), LimitReached: len(fixes) >= cfg.Limits.Fixes, Limit: cfg.Limits.Fixes}
	for _, item := range []struct {
		name  string
		value any
	}{{"requirements", requirements}, {"plan", plan}} {
		if err := writeJSON(filepath.Join(dir, item.name+".json"), item.value); err != nil {
			return prepared{}, err
		}
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return prepared{}, err
	}
	if len(requests) == 0 {
		return prepared{}, fmt.Errorf("no request is recorded yet")
	}
	criteriaDir := filepath.Join(artifacts, "criteria")
	if _, err := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: text(requests[0]["criteria_commit"]), Dir: pathAt(ctx.Cwd, criteriaDir)}); err != nil {
		return prepared{}, err
	}
	folder := filepath.Join(criteriaDir, cfg.Standards)
	gitCriteria, err := os.ReadFile(filepath.Join(pathAt(ctx.Cwd, folder), "git.md"))
	if err != nil {
		return prepared{}, err
	}
	prefixes, err := commitPrefixes(string(gitCriteria))
	if err != nil {
		return prepared{}, err
	}
	configuredChecks := []record.Document{}
	allChecks, err := config.ChecksAcrossBase(ctx.Cwd, ctx.BaseBranch)
	if err != nil {
		return prepared{}, err
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
		return prepared{}, err
	}
	input := record.Document{"language": cfg.Language, "standards": cfg.Standards, "criteria_folder": folder, "checks": configuredChecks, "commit_prefixes": prefixes, "writer_materials": mapping.Materials(folder, "writer:fix", "code"), "test_materials": mapping.Materials(folder, "writer:tests", "code"), "step_back_materials": mapping.Materials(folder, "judge:step-back", "code"), "dispute_materials": mapping.Materials(folder, "judge:dispute", "code")}
	for _, item := range []struct {
		name  string
		value any
	}{{"input", input}, {"findings", record.Document{"commit": head, "findings": findings}}, {"prepared", preparedFile{p, findings}}} {
		if err := writeJSON(filepath.Join(dir, item.name+".json"), item.value); err != nil {
			return prepared{}, err
		}
	}
	changeID, err := repo.ChangeID(head)
	if err != nil {
		return prepared{}, err
	}
	s := state{Review: review, Start: head, ChangeID: changeID, RequirementsVersion: requirements["version"], PlanVersion: plan["version"], Current: []record.Document{}, Disputes: []disputeRound{}, Objections: objections{[]record.Document{}, []record.Document{}}, OwnerDecisions: []record.Document{}, BrokenDecisions: []record.Document{}}
	if err := writeJSON(filepath.Join(dir, "state.json"), s); err != nil {
		return prepared{}, err
	}
	return p, nil
}

func proceed(artifacts string) (record.Document, error) {
	p, err := readJSON[prepared](filepath.Join(artifacts, "fix/prepared.json"))
	if err != nil {
		return nil, err
	}
	return record.Document{"proceed": p.Ready && !p.LimitReached}, nil
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

func commitSubject(written any, prefixes []string) string {
	line := strings.TrimSpace(text(written))
	match := regexp.MustCompile(`(?i)^([a-z][a-z0-9_-]*): `).FindStringSubmatch(line)
	if match != nil && slices.Contains(prefixes, match[1]) && !strings.Contains(line, "\n") && strings.TrimSpace(line[len(match[1])+2:]) != "" {
		return line
	}
	prefix := prefixes[0]
	if slices.Contains(prefixes, "fix") {
		prefix = "fix"
	}
	return prefix + ": address blocking findings"
}

func takeObjections(artifacts string) (record.Document, error) {
	dir := filepath.Join(artifacts, "fix")
	s, err := readJSON[state](filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	s.Round++
	source := filepath.Join(artifacts, "objections.json")
	var written record.Document
	if _, err := os.Stat(source); err == nil {
		written, err = readJSON[record.Document](source)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(dir, "objections"), 0755); err != nil {
			return nil, err
		}
		if err := os.Rename(source, filepath.Join(dir, "objections", strconv.Itoa(s.Round)+".written.json")); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	decided := map[string]bool{}
	for _, group := range [][]record.Document{s.Objections.Accepted, s.Objections.Rejected} {
		for _, o := range group {
			decided[text(o["finding"])] = true
		}
	}
	findings, err := readJSON[record.Document](filepath.Join(dir, "findings.json"))
	if err != nil {
		return nil, err
	}
	valid, invalid := checkObjections(written, documents(findings["findings"]), decided)
	if err := writeJSON(filepath.Join(dir, "objections", strconv.Itoa(s.Round)+".json"), record.Document{"objections": valid, "invalid": invalid}); err != nil {
		return nil, err
	}
	if s.Review != "" {
		for _, objection := range valid {
			s.OutsideRequirements = append(s.OutsideRequirements, record.Document{"item": objection["text"], "reason": objection["evidence"]})
		}
		valid = []record.Document{}
	}
	s.Current = valid
	if err := writeJSON(filepath.Join(dir, "state.json"), s); err != nil {
		return nil, err
	}
	return record.Document{"round": s.Round, "count": len(valid)}, nil
}

func disputeEnd(artifacts string, dispute record.Document) (record.Document, error) {
	file := filepath.Join(artifacts, "fix/state.json")
	s, err := readJSON[state](file)
	if err != nil {
		return nil, err
	}
	decisions := []record.Document{}
	if len(s.Current) > 0 {
		decisions = documents(dispute["decisions"])
	}
	if err := checkDecisions(s.Current, decisions); err != nil {
		return nil, err
	}
	pending := 0
	for _, d := range decisions {
		item := clone(d)
		item["round"] = s.Round
		if d["decision"] == "accepted" {
			s.Objections.Accepted = append(s.Objections.Accepted, item)
		} else {
			s.Objections.Rejected = append(s.Objections.Rejected, item)
			pending++
		}
	}
	if len(decisions) > 0 {
		s.Disputes = append(s.Disputes, disputeRound{s.Round, decisions})
	}
	s.Pending = pending
	s.Current = []record.Document{}
	if err := writeJSON(file, s); err != nil {
		return nil, err
	}
	return record.Document{"pending": s.Pending}, nil
}

func roundEnd(ctx record.ContextData, artifacts string, gate *checks.Gate, code *codeOutput) (record.Document, error) {
	dir := filepath.Join(artifacts, "fix")
	file := filepath.Join(dir, "state.json")
	s, err := readJSON[state](file)
	if err != nil {
		return nil, err
	}
	s.Gate = gate
	if gate != nil && gate.OnlyPreexisting() {
		s.Round--
		s.FailingLogs, err = gate.FailingLogs()
		if err != nil {
			return nil, err
		}
		if err := writeJSON(file, s); err != nil {
			return nil, err
		}
		return record.Document{"round": s.Round, "finished": true, "passed": false, "stuck": false, "pending": s.Pending, "needs_owner": 0, "broken": 0}, nil
	}
	s.OwnerDecisions = []record.Document{}
	s.BrokenDecisions = []record.Document{}
	if code != nil {
		s.OutsideRequirements = append(s.OutsideRequirements, code.OutsideRequirements...)
		if code.OwnerDecisions != nil {
			s.OwnerDecisions = code.OwnerDecisions
		}
		if code.BrokenDecisions != nil {
			s.BrokenDecisions = code.BrokenDecisions
		}
	}
	if gate != nil && gate.Passed && len(s.OutsideRequirements) == 0 {
		input, err := readJSON[record.Document](filepath.Join(dir, "input.json"))
		if err != nil {
			return nil, err
		}
		var written any
		if code != nil {
			written = code.CommitMessage
		}
		subject := commitSubject(written, stringsOf(input["commit_prefixes"]))
		if _, err := gitx.Run(ctx.Cwd, "add", "-A"); err != nil {
			return nil, err
		}
		message := fmt.Sprintf("%s\n\nArchon-Run: %s\nArchon-Fixed-From: %s\n", subject, ctx.RunID, s.Start)
		if _, err := gitx.Run(ctx.Cwd, "commit", "--allow-empty", "-q", "-m", message); err != nil {
			return nil, err
		}
	}
	if err := writeJSON(file, s); err != nil {
		return nil, err
	}
	passed, stuck := gate != nil && gate.Passed, gate != nil && gate.Stuck
	return record.Document{"round": s.Round, "finished": len(s.OutsideRequirements) > 0 || stuck || len(s.OwnerDecisions) > 0 || len(s.BrokenDecisions) > 0 || (passed && s.Pending == 0), "passed": passed, "stuck": stuck, "pending": s.Pending, "needs_owner": len(s.OwnerDecisions), "broken": len(s.BrokenDecisions)}, nil
}

func judgeBack(artifacts string) (record.Document, error) {
	s, err := readJSON[state](filepath.Join(artifacts, "fix/state.json"))
	if err != nil {
		return nil, err
	}
	return record.Document{"judge_back": (s.Gate == nil || !s.Gate.OnlyPreexisting()) && len(s.OutsideRequirements) == 0 && len(s.OwnerDecisions) == 0 && ((s.Gate != nil && s.Gate.Stuck) || len(s.BrokenDecisions) > 0)}, nil
}

func disputeTarget(s state, base string) record.Document {
	target := record.Document{"base": base, "commit": s.Start}
	if s.RequirementsVersion != nil {
		target["requirements_version"] = s.RequirementsVersion
	}
	if s.PlanVersion != nil {
		target["plan_version"] = s.PlanVersion
	}
	if s.ChangeID != nil {
		target["change_id"] = *s.ChangeID
	}
	return target
}

func finish(ctx record.ContextData, artifacts string, back *stepBack) (record.Document, error) {
	dir := filepath.Join(artifacts, "fix")
	p, err := readJSON[preparedFile](filepath.Join(dir, "prepared.json"))
	if err != nil {
		return nil, err
	}
	s, err := readJSON[state](filepath.Join(dir, "state.json"))
	if err != nil {
		return nil, err
	}
	doc := outcome(p, s, back)
	repo, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	base, err := repo.MergeBase(s.Start)
	if err != nil {
		return nil, err
	}
	target := disputeTarget(s, base)
	for _, round := range s.Disputes {
		judgment := disputeJudgment(round, target)
		if err := writeJSON(filepath.Join(dir, "judgments", fmt.Sprintf("dispute-%d.json", round.Round)), judgment); err != nil {
			return nil, err
		}
		if _, err := record.Write(ctx, "judgment", judgment, ""); err != nil {
			return nil, err
		}
	}
	if err := writeJSON(filepath.Join(dir, "outcome.json"), doc); err != nil {
		return nil, err
	}
	repo, err = gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	base, err = repo.MergeBase("")
	if err != nil {
		return nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	return record.WriteResult(ctx.Dir, "fix", doc, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head})
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
	for _, mode := range []string{"prepare", "proceed", "objections", "dispute-end", "round-end", "state", "finish"} {
		commands = append(commands, cli.Command{Area: "fix", Name: mode, Usage: "devkit fix " + mode, Run: func(args []string, env cli.Env) error {
			artifacts := env.Getenv("ARTIFACTS_DIR")
			if artifacts == "" {
				return fmt.Errorf("ARTIFACTS_DIR must be set")
			}
			var out any
			var err error
			switch mode {
			case "proceed":
				out, err = proceed(env.Path(artifacts))
			case "objections":
				out, err = takeObjections(env.Path(artifacts))
			case "dispute-end":
				dispute, parseErr := parseEnv[record.Document](env.Getenv("DISPUTE"))
				if parseErr != nil {
					return parseErr
				}
				var doc record.Document
				if dispute != nil {
					doc = *dispute
				}
				out, err = disputeEnd(env.Path(artifacts), doc)
			case "state":
				out, err = judgeBack(env.Path(artifacts))
			default:
				ctx, contextErr := record.Context(env)
				if contextErr != nil {
					return contextErr
				}
				ctx.Dir = env.Path(ctx.Dir)
				switch mode {
				case "prepare":
					out, err = prepare(ctx, artifacts)
				case "round-end":
					gate, parseErr := parseEnv[checks.Gate](env.Getenv("GATE"))
					if parseErr != nil {
						return parseErr
					}
					code, parseErr := parseEnv[codeOutput](env.Getenv("CODE"))
					if parseErr != nil {
						return parseErr
					}
					out, err = roundEnd(ctx, env.Path(artifacts), gate, code)
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
