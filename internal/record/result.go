package record

import (
	"fmt"
	"reflect"
	"strings"
)

type ResultOptions struct {
	RunID  string
	Base   string
	Commit string
	Now    string
}
type BuiltResult struct {
	Result    Document
	Questions []Document
}

func BuildResult(dir, workflow string, doc Document, o ResultOptions) (BuiltResult, error) {
	if _, err := StageOf(workflow); err != nil {
		return BuiltResult{}, err
	}
	if doc == nil {
		return BuiltResult{}, &RecordError{Message: "the outcome must be an object"}
	}
	supplied := []string{}
	for _, key := range []string{"run_id", "workflow", "recorded_at", "inputs", "base", "commit", "criteria_commit"} {
		if _, ok := doc[key]; ok {
			supplied = append(supplied, key)
		}
	}
	if len(supplied) > 0 {
		return BuiltResult{}, &RecordError{Message: "the record fills " + strings.Join(supplied, ", ") + "; leave them out of the outcome"}
	}
	for _, q := range array(doc["questions"]) {
		if _, ok := object(q)["id"]; ok {
			return BuiltResult{}, &RecordError{Message: "the record gives the questions their ids; leave id out"}
		}
	}
	h, err := History(dir)
	if err != nil {
		return BuiltResult{}, err
	}
	var run Document
	for _, r := range h.Runs {
		if r["run_id"] == o.RunID {
			run = r
		}
	}
	if run == nil {
		return BuiltResult{}, &RecordError{Message: fmt.Sprintf("run %s did not pass the start conditions, so it has no result to write", o.RunID)}
	}
	if run["workflow"] != workflow {
		return BuiltResult{}, &RecordError{Message: fmt.Sprintf("run %s started as %v, not %s", o.RunID, run["workflow"], workflow)}
	}
	for _, r := range h.Results {
		if r["run_id"] == o.RunID {
			return BuiltResult{}, &RecordError{Message: fmt.Sprintf("run %s already has a result", o.RunID)}
		}
	}
	if doc["outcome"] == "back" && doc["back"] != nil {
		back := str(object(doc["back"])["to"])
		to, err := StageOf(back)
		if err != nil {
			return BuiltResult{}, err
		}
		stage, _ := StageOf(workflow)
		if to >= stage {
			return BuiltResult{}, &RecordError{Message: fmt.Sprintf("%s can only go back to an earlier workflow, not %s", workflow, back)}
		}
	}
	requests, err := List(dir, "request")
	if err != nil {
		return BuiltResult{}, err
	}
	if len(requests) == 0 {
		return BuiltResult{}, &RecordError{Message: "no request is recorded yet, so the result has no criteria commit"}
	}
	inputs, err := LatestVersions(dir)
	if err != nil {
		return BuiltResult{}, err
	}
	count, err := QuestionCount(dir)
	if err != nil {
		return BuiltResult{}, err
	}
	now := timestamp(o.Now)
	result := clone(doc)
	result["run_id"] = o.RunID
	result["workflow"] = workflow
	result["recorded_at"] = now
	result["inputs"] = inputs
	result["base"] = o.Base
	result["commit"] = o.Commit
	result["criteria_commit"] = requests[0]["criteria_commit"]
	if result["loops"] == nil {
		result["loops"] = []any{}
	}
	questions := []Document{}
	if raw, exists := doc["questions"]; exists {
		if _, ok := raw.([]any); !ok {
			if _, ok := raw.([]Document); !ok {
				return BuiltResult{}, &RecordError{Message: "questions must be an array"}
			}
		}
		qs := []Document{}
		for i, q := range array(raw) {
			question := clone(object(q))
			question["id"] = fmt.Sprintf("Q%d", count+i+1)
			qs = append(qs, question)
			record := clone(question)
			record["run_id"] = o.RunID
			record["workflow"] = workflow
			record["asked_at"] = now
			record["versions"] = inputs
			record["commit"] = o.Commit
			questions = append(questions, record)
		}
		result["questions"] = qs
	}
	problems, err := Validate("result", result)
	if err != nil {
		return BuiltResult{}, err
	}
	if len(problems) > 0 {
		return BuiltResult{}, &RecordError{"not a valid result", problems}
	}
	for _, q := range questions {
		problems, err := Validate("question", q)
		if err != nil {
			return BuiltResult{}, err
		}
		if len(problems) > 0 {
			return BuiltResult{}, &RecordError{fmt.Sprintf("not a valid question %v", q["id"]), problems}
		}
	}
	return BuiltResult{result, questions}, nil
}
func WriteResult(dir, workflow string, doc Document, o ResultOptions) (Document, error) {
	built, err := BuildResult(dir, workflow, doc, o)
	if err != nil {
		return nil, err
	}
	for _, q := range built.Questions {
		if err := WriteKeyed(dir, "question", str(q["id"]), q); err != nil {
			return nil, err
		}
	}
	if _, err := Append(dir, "result", built.Result); err != nil {
		return nil, err
	}
	return built.Result, nil
}

func BackToPlan(rows []Document) Document {
	names := []string{}
	for _, row := range rows {
		names = append(names, str(row["id"])+": "+str(row["what"]))
	}
	return Document{"outcome": "back", "back": Document{"to": "plan", "premise": "An owner decision must be settled in plan", "observation": strings.Join(names, "\n"), "pending_owner_rows": rows}}
}
func PendingOwnerRows(dir string) ([]Document, error) {
	results, err := List(dir, "result")
	if err != nil {
		return nil, err
	}
	versions, err := LatestVersions(dir)
	if err != nil {
		return nil, err
	}
	rows := []Document{}
	for _, r := range results {
		if !reflect.DeepEqual(object(r["inputs"])["requirements_version"], versions["requirements_version"]) {
			continue
		}
		if r["workflow"] == "plan" && (r["outcome"] == "pass" || r["outcome"] == "carry") {
			rows = []Document{}
		}
		for _, value := range array(object(r["back"])["pending_owner_rows"]) {
			row := object(value)
			found := false
			for index, old := range rows {
				if old["id"] == row["id"] {
					rows[index] = row
					found = true
				}
			}
			if !found {
				rows = append(rows, row)
			}
		}
	}
	return rows, nil
}

func Returned(ctx ContextData, workflow string) ([]Document, error) {
	h, err := History(ctx.Dir)
	if err != nil {
		return nil, err
	}
	target, err := StageOf(workflow)
	if err != nil {
		return nil, err
	}
	pending := []Document{}
	for _, run := range h.Runs {
		stage, err := StageOf(str(run["workflow"]))
		if err != nil {
			return nil, err
		}
		kept := []Document{}
		for _, back := range pending {
			source, err := StageOf(str(back["workflow"]))
			if err != nil {
				return nil, err
			}
			if stage >= target && (stage >= source || run["workflow"] == workflow) {
				kept = append(kept, back)
			}
		}
		pending = kept
		for _, result := range h.Results {
			if result["run_id"] != run["run_id"] {
				continue
			}
			if result["workflow"] == workflow && (result["outcome"] == "pass" || result["outcome"] == "carry") {
				pending = []Document{}
			}
			back := object(result["back"])
			if result["outcome"] == "back" && back["to"] == workflow {
				returned := clone(back)
				returned["run_id"] = result["run_id"]
				returned["workflow"] = result["workflow"]
				pending = append(pending, returned)
			}
		}
		if run["run_id"] == ctx.RunID {
			break
		}
	}
	return pending, nil
}
