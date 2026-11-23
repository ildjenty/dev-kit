package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

type step struct {
	id, workflow, outcome string
	run, result           Document
}

func recordOf(t *testing.T, steps []step) HistoryData {
	dir := newRecord(t)
	for _, s := range steps {
		run(dir, s.id, s.workflow, s.run)
		if s.outcome != "" {
			result(dir, s.id, s.workflow, s.outcome, s.result)
		}
	}
	return must(History(dir))
}
func TestClassifyStarts(t *testing.T) {
	stop := Document{"stop": Document{"state": "Stopped", "unresolved": []any{}, "premise": ""}}
	for _, tc := range []struct {
		name, workflow, start string
		steps                 []step
		answered              []string
		counted               bool
	}{{"names the first run", "requirements", "first", nil, nil, false}, {"moving on after a pass", "plan", "next", []step{{id: "r1", workflow: "requirements", outcome: "pass"}}, nil, false}, {"moving on after a carried plan", "implement", "next", []step{{id: "r1", workflow: "requirements", outcome: "pass"}, {id: "p1", workflow: "plan", outcome: "carry", result: Document{"carried": []Document{{"id": "F1", "judge": "structure", "text": "Finding"}}}}}, nil, false}, {"owner answers", "requirements", "answer", []step{{id: "r1", workflow: "requirements", outcome: "stop", result: stop}}, []string{"Q1"}, false}, {"goes back to chosen workflow", "plan", "step_back", []step{{id: "r1", workflow: "requirements", outcome: "pass"}, {id: "p1", workflow: "plan", outcome: "pass"}, {id: "i1", workflow: "implement", outcome: "back", result: Document{"back": Document{"to": "plan", "premise": "Premise", "observation": "Observation"}}}}, nil, true}, {"continues stopped workflow", "requirements", "continue_after_stop", []step{{id: "r1", workflow: "requirements", outcome: "stop", result: stop}}, nil, true}, {"retries unfinished workflow", "plan", "retry_after_error", []step{{id: "r1", workflow: "requirements", outcome: "pass"}, {id: "p1", workflow: "plan"}}, nil, true}, {"new request after pull request", "requirements", "other", []step{{id: "r1", workflow: "requirements", outcome: "pass"}, {id: "p1", workflow: "plan", outcome: "pass"}, {id: "i1", workflow: "implement", outcome: "pass"}, {id: "pr1", workflow: "pr", outcome: "pass"}}, nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			state := must(LimitState(recordOf(t, tc.steps), tc.workflow, tc.answered, 3))
			testutil.Equal(t, state.Start, tc.start)
			testutil.Equal(t, state.Counted, tc.counted)
		})
	}
}
func restarted(t *testing.T, n int) HistoryData {
	steps := []step{{id: "r1", workflow: "requirements", outcome: "pass"}}
	for i := 0; i < n; i++ {
		steps = append(steps, step{id: fmt.Sprintf("p%d", i), workflow: "plan", outcome: "stop", run: Document{"counted": i > 0}, result: Document{"stop": Document{"state": "Stopped", "unresolved": []any{}, "premise": ""}}})
	}
	return recordOf(t, steps)
}
func TestRestartLimit(t *testing.T) {
	for _, tc := range []struct {
		name             string
		n                int
		answered         []string
		restarts         int
		counted, reached bool
	}{{"lets third restart start", 3, nil, 2, true, false}, {"blocks fourth restart", 4, nil, 3, true, true}, {"allows uncounted start at limit", 4, []string{"Q1"}, 3, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			state := must(LimitState(restarted(t, tc.n), "plan", tc.answered, 3))
			testutil.Equal(t, state.Restarts, tc.restarts)
			testutil.Equal(t, state.Counted, tc.counted)
			testutil.Equal(t, state.Reached, tc.reached)
		})
	}
}
func TestContinuationResetsCount(t *testing.T) {
	dir := newRecord(t)
	run(dir, "r1", "requirements", nil)
	for _, id := range []string{"r2", "r3", "r4"} {
		run(dir, id, "requirements", Document{"start": "retry_after_error", "counted": true})
	}
	must(Append(dir, "continuation", Document{"by": "owner", "answer": "Continue", "restarts": 3, "limit": 3, "run_id": "r4", "workflow": "requirements", "start": "retry_after_error", "recorded_at": "t"}))
	testutil.Equal(t, must(LimitState(must(History(dir)), "requirements", nil, 3)).Restarts, 0)
	run(dir, "r5", "requirements", Document{"start": "retry_after_error", "counted": true})
	testutil.Equal(t, must(LimitState(must(History(dir)), "requirements", nil, 3)).Restarts, 1)
}
func TestVersionsKeepRestartCount(t *testing.T) {
	dir := newRecord(t)
	run(dir, "r1", "requirements", nil)
	run(dir, "r2", "requirements", Document{"start": "retry_after_error", "counted": true})
	requirements(dir, "r2")
	requirements(dir, "r2")
	testutil.Equal(t, must(LimitState(must(History(dir)), "plan", nil, 3)).Restarts, 1)
}
func commandEnv(repo string, vars map[string]string, out, stderr *bytes.Buffer) cli.Env {
	return cli.Env{Cwd: repo, Stdout: out, Stderr: stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
}
func TestLimitCLIUsesBranchRecord(t *testing.T) {
	repo := testutil.Repo(t)
	state := t.TempDir()
	dir := RecordDir(state, "feature/x")
	run(dir, "r1", "requirements", nil)
	var out, stderr bytes.Buffer
	r := cli.New("test", Commands())
	testutil.Equal(t, r.Execute([]string{"record", "limit", "requirements"}, commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r2", "INPUTS_ANSWERS": ""}, &out, &stderr)), 0)
	var actual LimitStatus
	testutil.OK(t, json.Unmarshal(out.Bytes(), &actual))
	testutil.Equal(t, actual, LimitStatus{"retry_after_error", true, 0, 3, false})
}
func TestLatestContinuationResetsCount(t *testing.T) {
	h := HistoryData{Continuations: []Document{{"run_id": "p4"}, {"run_id": "p8"}}}
	for _, id := range []string{"p4", "p5", "p6", "p7", "p8", "p9"} {
		h.Runs = append(h.Runs, Document{"run_id": id, "counted": id != "p4"})
	}
	testutil.Equal(t, RestartsOf(h), 1)
}
