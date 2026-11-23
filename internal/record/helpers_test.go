package record

import (
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func sha(c string) string              { return strings.Repeat(c, 40) }
func newRecord(t *testing.T) string    { return RecordDir(t.TempDir(), "feature/x") }
func must[T any](value T, err error) T { return testutil.Must(value, err) }
func request(dir string) Appended {
	return must(Append(dir, "request", Document{"owner_words": "Request", "supplement": "", "criteria_commit": sha("c"), "run_id": "r0", "recorded_at": "t"}))
}
func run(dir, id, workflow string, extra Document) Appended {
	doc := Document{"run_id": id, "workflow": workflow, "started_at": "t", "start": "other", "counted": false, "answers": []any{}}
	for k, v := range extra {
		doc[k] = v
	}
	return must(Append(dir, "run", doc))
}
func result(dir, id, workflow, outcome string, extra Document) Appended {
	doc := Document{"run_id": id, "workflow": workflow, "recorded_at": "t", "outcome": outcome, "inputs": Document{"requirements_version": nil, "plan_version": nil}, "base": sha("b"), "commit": sha("d"), "criteria_commit": sha("c"), "loops": []any{}}
	for k, v := range extra {
		doc[k] = v
	}
	return must(Append(dir, "result", doc))
}
func requirements(dir string, ids ...string) Appended {
	id := "r1"
	if len(ids) > 0 {
		id = ids[0]
	}
	return must(Append(dir, "requirements", Document{"request": 1, "run_id": id, "recorded_at": "t", "goal": "Goal", "completion_conditions": []Document{{"id": "C1", "text": "Completion condition"}}, "constraints": []any{}, "open_points": []any{}}))
}
func judgment(dir, id, judge string, extra Document) Appended {
	kind := "code"
	target := Document{"requirements_version": 1, "plan_version": 1, "base": sha("b"), "commit": sha("d"), "change_id": strings.Repeat("a", 64)}
	if judge == "requirements" {
		kind = "requirements"
		target = Document{"requirements_version": 1}
	}
	doc := Document{"run_id": id, "recorded_at": "t", "judge": judge, "kind": kind, "target": target, "criteria_commit": sha("c"), "materials": []any{}, "observations": []any{}, "verdict": "pass", "findings": []any{}}
	for k, v := range extra {
		doc[k] = v
	}
	return must(Append(dir, "judgment", doc))
}
func plan(dir string, ids ...string) Appended {
	id := "p1"
	if len(ids) > 0 {
		id = ids[0]
	}
	return must(Append(dir, "plan", Document{"requirements_version": 1, "run_id": id, "recorded_at": "t", "skeleton_commit": sha("e"), "decisions": []any{}, "increments": []Document{{"id": "U1", "summary": "First increment", "criteria": []string{"C1"}}}, "tests": []Document{{"criterion": "C1", "method": "test", "file": "a.test.ts", "name": "does it"}}, "kept_behaviors": []any{}}))
}
func begin(dir, workflow, id string, answers ...[]Answer) StartOutput {
	o := StartOptions{RunID: id, Commit: sha("d"), ChangeID: func() (*string, error) { s := strings.Repeat("a", 64); return &s, nil }, BaseCommit: sha("c"), Now: "t"}
	if len(answers) > 0 {
		o.Answers = answers[0]
	}
	return must(Start(dir, workflow, o))
}
func beginAt(dir, workflow, id, commit, patch string) StartOutput {
	return must(Start(dir, workflow, StartOptions{RunID: id, Commit: commit, ChangeID: func() (*string, error) { return &patch, nil }, BaseCommit: sha("c"), Now: "t"}))
}
func end(dir, workflow, id string, doc Document) Document {
	return must(WriteResult(dir, workflow, doc, ResultOptions{RunID: id, Base: sha("b"), Commit: sha("d"), Now: "t"}))
}
func ask(text string, next []string, kinds ...string) Document {
	kind := "open_point"
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	return Document{"text": text, "target": Document{"kind": kind, "ref": "O1"}, "next_workflows": next, "invalidates": []any{}}
}
func afterRequirements(t *testing.T) string {
	dir := newRecord(t)
	testutil.Equal(t, begin(dir, "requirements", "r1").Stopped, false)
	request(dir)
	requirements(dir)
	end(dir, "requirements", "r1", Document{"outcome": "pass"})
	return dir
}
func afterPlan(t *testing.T) string {
	dir := afterRequirements(t)
	testutil.Equal(t, begin(dir, "plan", "p1").Stopped, false)
	plan(dir)
	end(dir, "plan", "p1", Document{"outcome": "pass"})
	return dir
}
func runIDs(dir string) []string {
	ids := []string{}
	for _, r := range must(History(dir)).Runs {
		ids = append(ids, str(r["run_id"]))
	}
	return ids
}
func atLimit(t *testing.T) string {
	dir := afterRequirements(t)
	for _, id := range []string{"p1", "p2", "p3"} {
		run(dir, id, "plan", Document{"start": "retry_after_error", "counted": true})
	}
	return dir
}
func goOn(dir, workflow, id, continuation string, answers []Answer) StartOutput {
	return must(Start(dir, workflow, StartOptions{RunID: id, Commit: sha("d"), BaseCommit: sha("c"), Answers: answers, Continuation: continuation, Now: "t"}))
}
func finding(blocking bool) Document {
	return Document{"id": "F1", "blocking": blocking, "needs_decision": false, "text": "Finding", "evidence": "a.ts:1"}
}
func stopped() Document {
	return Document{"outcome": "stop", "stop": Document{"state": "s", "unresolved": []any{}, "premise": ""}}
}
func judged(t *testing.T, doc Document, findings []Document) string {
	dir := newRecord(t)
	begin(dir, "judge", "j1")
	request(dir)
	verdict := "pass"
	if len(findings) > 0 {
		verdict = "fail"
	}
	judgment(dir, "j1", "scope", Document{"verdict": verdict, "findings": findings})
	end(dir, "judge", "j1", doc)
	return dir
}
