package record

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestWriteStampsAndProtectsRecords(t *testing.T) {
	repo := testutil.Repo(t)
	dir := newRecord(t)
	ctx := ContextData{Dir: dir, RunID: "r1", Cwd: repo, BaseBranch: "main"}
	_, err := Write(ctx, "request", Document{"owner_words": "Request", "supplement": ""}, "t")
	testutil.Fails(t, err, "did not pass the start conditions")
	begin(dir, "requirements", "r1")
	first := must(Write(ctx, "request", Document{"owner_words": "Request", "supplement": ""}, "t"))
	testutil.Equal(t, first.Entry["criteria_commit"], testutil.Git(t, repo, "rev-parse", "main"))
	second := must(Write(ctx, "request", Document{"owner_words": "Additional request", "supplement": ""}, "t"))
	testutil.Equal(t, second.Entry["criteria_commit"], first.Entry["criteria_commit"])
	body := requirements(newRecord(t)).Entry
	delete(body, "version")
	delete(body, "request")
	delete(body, "run_id")
	delete(body, "recorded_at")
	req := must(Write(ctx, "requirements", body, "t"))
	testutil.Equal(t, req.Entry["request"], 2)
	testutil.Equal(t, req.Entry["version"], 1)
	testutil.Equal(t, req.Entry["run_id"], "r1")
	body = plan(newRecord(t)).Entry
	for _, key := range []string{"version", "requirements_version", "run_id", "recorded_at"} {
		delete(body, key)
	}
	written := must(Write(ctx, "plan", body, "t"))
	testutil.Equal(t, written.Entry["requirements_version"], 1)
	_, err = Write(ctx, "result", Document{}, "t")
	testutil.Fails(t, err, "write takes one of")
	_, err = Write(ctx, "request", Document{"criteria_commit": sha("a")}, "t")
	testutil.Fails(t, err, "leave them out")
	_, err = Write(ctx, "allowed", Document{"question_id": "Q1"}, "t")
	testutil.Fails(t, err, "has no answer")
}
func TestInvalidOutcomeLeavesNoQuestionsOrResult(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	_, err := WriteResult(dir, "requirements", Document{"outcome": "question", "questions": []Document{ask("Valid", []string{"requirements"}), ask("", []string{"requirements"})}}, ResultOptions{RunID: "r1", Base: sha("b"), Commit: sha("d"), Now: "t"})
	testutil.Fails(t, err, "not a valid result")
	testutil.Equal(t, must(QuestionCount(dir)), 0)
	testutil.Equal(t, must(List(dir, "result")), []Document{})
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("Choose", []string{"requirements"})}})
	testutil.Equal(t, must(QuestionCount(dir)), 1)
	testutil.Equal(t, len(must(ListKeyed(dir, "question"))), 1)
	testutil.Equal(t, must(ListKeyed(dir, "answer")), []Document{})
	_, err = WriteResult(dir, "requirements", Document{"outcome": "pass"}, ResultOptions{RunID: "r1"})
	testutil.Fails(t, err, "already has a result")
}
func TestResultRejectsUnstartedWrongWorkflowAndFilledFields(t *testing.T) {
	dir := newRecord(t)
	o := ResultOptions{RunID: "r1", Base: sha("b"), Commit: sha("d"), Now: "t"}
	_, err := BuildResult(dir, "requirements", Document{"outcome": "pass"}, o)
	testutil.Fails(t, err, "did not pass the start conditions")
	begin(dir, "requirements", "r1")
	request(dir)
	_, err = BuildResult(dir, "plan", Document{"outcome": "pass"}, o)
	testutil.Fails(t, err, "started as requirements, not plan")
	_, err = BuildResult(dir, "requirements", Document{"outcome": "pass", "run_id": "r1"}, o)
	testutil.Fails(t, err, "leave them out of the outcome")
	_, err = BuildResult(dir, "requirements", Document{"outcome": "back", "back": Document{"to": "plan"}}, o)
	testutil.Fails(t, err, "can only go back to an earlier workflow")
	_, err = BuildResult(dir, "requirements", Document{"outcome": "question", "questions": []Document{{"id": "Q1"}}}, o)
	testutil.Fails(t, err, "leave id out")
}
func TestKeyedWritesAreExclusive(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("Choose", []string{"requirements"})}})
	q := must(ReadKeyed(dir, "question", "Q1"))
	testutil.Fails(t, WriteKeyed(dir, "question", "Q1", q), "already recorded")
	testutil.Equal(t, must(ReadKeyed(dir, "question", "Q1")), q)
	testutil.Equal(t, RecordDir("state", "feature/a !'()~"), filepath.Join("state", "records", "feature%2Fa%20!'()~"))
}
func TestRelativeStateAndFilesUseInjectedCwd(t *testing.T) {
	repo := testutil.Repo(t)
	state := ".state"
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1"}
	var out, stderr bytes.Buffer
	env := commandEnv(repo, vars, &out, &stderr)
	r := cli.New("test", Commands())
	testutil.Equal(t, r.Execute([]string{"record", "start", "requirements"}, env), 0)
	var started StartOutput
	testutil.OK(t, json.Unmarshal(out.Bytes(), &started))
	testutil.Equal(t, started.Details, RecordDir(state, "feature/x"))
	dir := filepath.Join(repo, started.Details)
	testutil.Equal(t, runIDs(dir), []string{"r1"})
	testutil.Put(t, repo, "request.json", `{"owner_words":"Request","supplement":""}`)
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"record", "write", "request", "request.json"}, env), 0)
	var written struct {
		Kind   string
		Number int
		Path   string
	}
	testutil.OK(t, json.Unmarshal(out.Bytes(), &written))
	testutil.Equal(t, written.Path, filepath.Join(started.Details, "request", "1.json"))
	_, err := os.Stat(filepath.Join(repo, written.Path))
	testutil.OK(t, err)
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"record", "latest", "request"}, env), 0)
	var doc Document
	testutil.OK(t, json.Unmarshal(out.Bytes(), &doc))
	testutil.Equal(t, doc["owner_words"], "Request")
	testutil.Put(t, repo, "outcome.json", `{"outcome":"pass"}`)
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"record", "result", "requirements", "outcome.json"}, env), 0)
	testutil.Equal(t, len(must(List(dir, "result"))), 1)
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"record", "dir"}, env), 0)
	testutil.Equal(t, out.String(), started.Details+"\n")
}
