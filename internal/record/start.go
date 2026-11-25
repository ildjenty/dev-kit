package record

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

func AnswerErrors(dir, workflow string, answers []Answer) ([]string, error) {
	h, err := History(dir)
	if err != nil {
		return nil, err
	}
	valid, err := ValidResults(h, "")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	problems := []string{}
	for _, answer := range answers {
		id := answer.QuestionID
		q, err := ReadKeyed(dir, "question", id)
		if err != nil {
			return nil, err
		}
		a, err := ReadKeyed(dir, "answer", id)
		if err != nil {
			return nil, err
		}
		active := false
		for _, r := range valid {
			if r["run_id"] == q["run_id"] {
				active = true
			}
		}
		switch {
		case seen[id]:
			problems = append(problems, id+" is answered twice in the input")
		case q == nil:
			problems = append(problems, id+" is not a question in the record")
		case a != nil:
			problems = append(problems, id+" is already answered")
		case !slices.Contains(stringsOf(q["next_workflows"]), workflow):
			problems = append(problems, fmt.Sprintf("%s is to be followed by %s, not %s", id, strings.Join(stringsOf(q["next_workflows"]), " or "), workflow))
		case !active:
			problems = append(problems, id+" was asked by a result that a later start of an earlier workflow made invalid")
		}
		seen[id] = true
	}
	return problems, nil
}
func RecordAnswers(dir, workflow string, answers []Answer, runID, now string) error {
	for _, a := range answers {
		q, err := ReadKeyed(dir, "question", a.QuestionID)
		if err != nil {
			return err
		}
		if err := WriteKeyed(dir, "answer", a.QuestionID, Document{"question_id": a.QuestionID, "answer": a.Answer, "run_id": runID, "workflow": workflow, "answered_at": now, "question": q["text"], "target": q["target"], "versions": q["versions"], "commit": q["commit"]}); err != nil {
			return err
		}
	}
	return nil
}

type inputState struct {
	present bool
	errors  []string
	passed  []Document
}

func requirementsInput(dir string, valid []Document, needed bool) (inputState, error) {
	versions, err := LatestVersions(dir)
	if err != nil {
		return inputState{}, err
	}
	v := versions["requirements_version"]
	state := inputState{errors: []string{}, passed: []Document{}}
	if v == nil {
		if needed {
			state.errors = append(state.errors, "no requirements are recorded yet")
		}
		return state, nil
	}
	state.present = true
	for _, r := range valid {
		if r["workflow"] == "requirements" && r["outcome"] == "pass" && reflect.DeepEqual(object(r["inputs"])["requirements_version"], v) {
			state.passed = append(state.passed, r)
		}
	}
	if len(state.passed) == 0 {
		state.errors = append(state.errors, fmt.Sprintf("requirements version %v is the latest, and no valid result of requirements passed on it", v))
	}
	return state, nil
}
func planInput(dir string, valid []Document) (inputState, error) {
	now, err := LatestVersions(dir)
	if err != nil {
		return inputState{}, err
	}
	plan, err := Latest(dir, "plan")
	if err != nil {
		return inputState{}, err
	}
	state := inputState{errors: []string{}}
	if plan == nil || now["requirements_version"] == nil || !reflect.DeepEqual(plan["requirements_version"], now["requirements_version"]) {
		return state, nil
	}
	state.present = true
	passed, err := PlanAccepted(dir, plan, valid)
	if err != nil {
		return state, err
	}
	if passed {
		return state, nil
	}

	state.errors = append(state.errors, fmt.Sprintf("plan version %v is the latest, and no valid result of plan passed on it", plan["version"]))
	return state, nil
}
func PlanAccepted(dir string, plan Document, valid []Document) (bool, error) {
	plans, err := List(dir, "plan")
	if err != nil {
		return false, err
	}
	for plan != nil {
		for _, r := range valid {
			inputs := object(r["inputs"])
			if r["workflow"] == "plan" && (r["outcome"] == "pass" || r["outcome"] == "carry") && reflect.DeepEqual(inputs["plan_version"], plan["version"]) && reflect.DeepEqual(inputs["requirements_version"], plan["requirements_version"]) {
				return true, nil
			}
		}
		derived := plan["derived_from"]
		if derived == nil {
			return false, nil
		}
		implemented := false
		for _, r := range valid {
			if r["workflow"] == "implement" && r["run_id"] == plan["run_id"] && r["outcome"] == "pass" && reflect.DeepEqual(object(r["inputs"])["plan_version"], plan["version"]) {
				implemented = true
			}
		}
		if !implemented {
			return false, nil
		}
		var parent Document
		for _, p := range plans {
			if reflect.DeepEqual(p["version"], derived) && p["version"].(float64) < plan["version"].(float64) && reflect.DeepEqual(p["requirements_version"], plan["requirements_version"]) {
				parent = p
			}
		}
		plan = parent
	}
	return false, nil
}

type JudgedChange struct {
	Commit    string     `json:"commit"`
	RunID     string     `json:"run_id"`
	Judgments []Document `json:"judgments"`
}

func JudgedStep(judgments []Document, changeID *string, commit string) *JudgedChange {
	same := []Document{}
	for _, j := range judgments {
		if j["kind"] == "code" && (commit != "" && object(j["target"])["commit"] == commit || changeID != nil && object(j["target"])["change_id"] == *changeID) {
			same = append(same, j)
		}
	}
	if len(same) == 0 {
		return nil
	}
	last := same[len(same)-1]
	result := &JudgedChange{str(object(last["target"])["commit"]), str(last["run_id"]), []Document{}}
	for _, j := range same {
		if j["run_id"] == result.RunID && object(j["target"])["commit"] == result.Commit {
			result.Judgments = append(result.Judgments, j)
		}
	}
	return result
}

type ConditionOptions struct {
	Review     string
	Cwd        string
	BaseBranch string
	Commit     string
	ChangeID   func() (*string, error)
	Answers    []Answer
}

func StartConditions(dir, workflow string, options ConditionOptions) ([]string, error) {
	if _, err := StageOf(workflow); err != nil {
		return nil, err
	}
	h, err := History(dir)
	if err != nil {
		return nil, err
	}
	valid, err := ValidResults(h, workflow)
	if err != nil {
		return nil, err
	}
	reasons := []string{}
	if workflow != "optimize" && workflow != "refine" && workflow != "retro" && workflow != "requirements" && workflow != "sync" {
		req, err := requirementsInput(dir, valid, workflow == "plan" || workflow == "implement" || workflow == "fix" && options.Review != "")
		if err != nil {
			return nil, err
		}
		reasons = append(reasons, req.errors...)
		if workflow != "plan" && (workflow != "implement" || len(req.errors) == 0) {
			plan, err := planInput(dir, valid)
			if err != nil {
				return nil, err
			}
			reasons = append(reasons, plan.errors...)
			if !plan.present && req.present && len(req.errors) == 0 {
				reasons = append(reasons, "the latest requirements have no plan")
			}

		}
		if workflow == "fix" || workflow == "pr" {
			var judge Document
			for _, r := range valid {
				if r["workflow"] == "judge" {
					judge = r
				}
			}
			if judge == nil {
				if workflow == "fix" {
					reasons = append(reasons, "judge has no valid result, so there are no findings to fix")
				} else {
					reasons = append(reasons, "judge has no valid result, so no commit passed the judgment")
				}
			} else {
				if workflow == "pr" {
					index, _ := judge["run_index"].(int)
					for _, run := range h.Runs[index+1:] {
						if run["workflow"] == "fix" && run["review"] != nil {
							reasons = append(reasons, "a review fix started after the latest judge result; run judge again before pr")
							break
						}
					}
				}
				judgments, err := List(dir, "judgment")
				if err != nil {
					return nil, err
				}
				if workflow == "fix" && options.Review == "" {
					if judge["commit"] != options.Commit {
						reasons = append(reasons, fmt.Sprintf("the latest result of judge (run %v) judged %v, but HEAD is %s", judge["run_id"], judge["commit"], options.Commit))
					}
					blocking := false
					for _, j := range judgments {
						if j["run_id"] == judge["run_id"] {
							for _, f := range array(j["findings"]) {
								blocking = blocking || object(f)["blocking"] == true
							}
						}
					}
					if !blocking {
						reasons = append(reasons, fmt.Sprintf("the latest result of judge (run %v) has no blocking finding", judge["run_id"]))
					}
				} else {
					if workflow == "pr" && judge["outcome"] != "pass" {
						reasons = append(reasons, fmt.Sprintf("the latest result of judge (run %v) is %v", judge["run_id"], judge["outcome"]))
					}
					var id *string
					if options.ChangeID != nil {
						id, err = options.ChangeID()
						if err != nil {
							return nil, err
						}
					}
					step := JudgedStep(judgments, id, options.Commit)
					if step == nil || step.RunID != judge["run_id"] {
						patch := "none"
						if id != nil {
							patch = *id
						}
						reasons = append(reasons, fmt.Sprintf("the latest result of judge (run %v) judged the change of %v, and the change of HEAD %s (change-id %s) is not the one it judged", judge["run_id"], judge["commit"], options.Commit, patch))
					}
					now, err := LatestVersions(dir)
					if err != nil {
						return nil, err
					}
					for _, key := range []string{"requirements_version", "plan_version"} {
						if !reflect.DeepEqual(object(judge["inputs"])[key], now[key]) {
							reasons = append(reasons, fmt.Sprintf("the latest result of judge (run %v) was written on %s %s, but the latest is %s", judge["run_id"], key, display(object(judge["inputs"])[key]), display(now[key])))
						}
					}
				}
			}
		}
	}
	for _, r := range valid {
		for _, q := range array(r["questions"]) {
			id := str(object(q)["id"])
			answer, err := ReadKeyed(dir, "answer", id)
			if err != nil {
				return nil, err
			}
			if answer == nil && !slices.ContainsFunc(options.Answers, func(a Answer) bool { return a.QuestionID == id }) {
				reasons = append(reasons, fmt.Sprintf("%s (asked by %v, run %v) has no answer", id, r["workflow"], r["run_id"]))
			}
		}
	}
	return reasons, nil
}

type StartOptions struct {
	BaseBranchSource string
	Review           string
	Cwd              string
	BaseBranch       string
	IssueReasons     []string
	RunID            string
	Commit           string
	ChangeID         func() (*string, error)
	BaseCommit       string
	Answers          []Answer
	Continuation     string
	Limit            *int
	Now              string
	WorkflowsChanged []string
	WorktreeChanged  []string
}
type StartOutput struct {
	Stopped   bool     `json:"stopped"`
	Stage     string   `json:"stage"`
	Reason    string   `json:"reason"`
	Details   string   `json:"details"`
	Remaining []string `json:"remaining"`
	Start     string   `json:"start"`
	Counted   bool     `json:"counted"`
	Restarts  int      `json:"restarts"`
}

func Start(dir, workflow string, o StartOptions) (StartOutput, error) {
	output := func(stopped bool, reason string, remaining []string, state LimitStatus) StartOutput {
		if remaining == nil {
			remaining = []string{}
		}
		return StartOutput{stopped, "start", reason, dir, remaining, state.Start, state.Counted, state.Restarts}
	}
	if _, err := StageOf(workflow); err != nil {
		return StartOutput{}, err
	}
	h, err := History(dir)
	if err != nil {
		return StartOutput{}, err
	}
	for _, r := range h.Runs {
		if r["run_id"] == o.RunID {
			if r["workflow"] != workflow {
				return StartOutput{}, &RecordError{Message: fmt.Sprintf("run %s is registered as %v", o.RunID, r["workflow"])}
			}
			return output(false, "", nil, LimitStatus{Start: str(r["start"]), Counted: r["counted"] == true, Restarts: RestartsOf(h)}), nil
		}
	}
	if len(o.WorkflowsChanged) > 0 {
		return output(true, fmt.Sprintf("the workflows of the main checkout have changes that are not committed, so %s did not start", workflow), o.WorkflowsChanged, LimitStatus{}), nil
	}
	if len(o.WorktreeChanged) > 0 {
		return output(true, fmt.Sprintf("the worktree of the branch has changes that are not committed, so %s did not start", workflow), o.WorktreeChanged, LimitStatus{}), nil
	}
	problems, err := AnswerErrors(dir, workflow, o.Answers)
	if err != nil {
		return StartOutput{}, err
	}
	if len(problems) > 0 {
		return output(true, fmt.Sprintf("the answers were not recorded, so %s did not start", workflow), problems, LimitStatus{}), nil
	}
	answered := []string{}
	for _, a := range o.Answers {
		answered = append(answered, a.QuestionID)
	}
	limit := Limit
	if o.Limit != nil {
		limit = *o.Limit
	}
	state, err := LimitState(h, workflow, answered, limit)
	if err != nil {
		return StartOutput{}, err
	}
	if o.Continuation != "" && !state.Reached {
		counted := "is not"
		if state.Counted {
			counted = "is"
		}
		return output(true, fmt.Sprintf("continue was passed, and this start does not reach the limit of restarts, so %s did not start and nothing was recorded", workflow), []string{fmt.Sprintf("this start (%s) %s counted, and the branch restarted %d times (limit %d); start it without continue", state.Start, counted, state.Restarts, limit)}, state), nil
	}
	now := timestamp(o.Now)
	reasons, err := StartConditions(dir, workflow, ConditionOptions{Cwd: o.Cwd, BaseBranch: o.BaseBranch, Review: o.Review, Commit: o.Commit, ChangeID: o.ChangeID, Answers: o.Answers})
	if err != nil {
		return StartOutput{}, err
	}

	reasons = append(reasons, o.IssueReasons...)
	if state.Reached && o.Continuation == "" {
		reasons = append(reasons, fmt.Sprintf("the branch already restarted %d times (limit %d) and this start (%s) would be another; it starts when the owner's words that let it go on are passed as continue", state.Restarts, limit, state.Start))
	}
	if len(reasons) > 0 {
		return output(true, fmt.Sprintf("the start conditions of %s do not hold, so it did not start", workflow), reasons, state), nil
	}
	requests, err := List(dir, "request")
	if err != nil {
		return StartOutput{}, err
	}
	needsRequest := workflow != "refine" && workflow != "requirements" && len(requests) == 0
	if needsRequest && o.BaseCommit == "" {
		return StartOutput{}, &RecordError{Message: "the branch has no request yet, and the base branch's commit was not given"}
	}
	if err := RecordAnswers(dir, workflow, o.Answers, o.RunID, now); err != nil {
		return StartOutput{}, err
	}

	if o.Continuation != "" {
		if _, err := Append(dir, "continuation", Document{"by": "owner", "answer": o.Continuation, "restarts": state.Restarts, "limit": limit, "run_id": o.RunID, "workflow": workflow, "start": state.Start, "recorded_at": now}); err != nil {
			return StartOutput{}, err
		}
	}
	run := Document{"run_id": o.RunID, "workflow": workflow, "started_at": now, "start": state.Start, "counted": state.Counted, "answers": answered}
	if o.BaseBranchSource != "" {
		run["base_branch"], run["base_branch_source"] = o.BaseBranch, o.BaseBranchSource
	}
	if workflow == "fix" && o.Review != "" {
		run["review"] = Document{"text": o.Review, "commit": o.Commit}
	}
	if _, err := Append(dir, "run", run); err != nil {
		return StartOutput{}, err
	}
	if needsRequest {
		if _, err := Append(dir, "request", Document{"owner_words": "", "supplement": "", "criteria_commit": o.BaseCommit, "run_id": o.RunID, "recorded_at": now}); err != nil {
			return StartOutput{}, err
		}
	}
	h, err = History(dir)
	if err != nil {
		return StartOutput{}, err
	}
	state.Restarts = RestartsOf(h)
	return output(false, "", nil, state), nil
}

func display(value any) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprint(value)
}
