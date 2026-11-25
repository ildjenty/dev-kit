package record

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestReadRecordsRejectsInvalidShape(t *testing.T) {
	for _, kind := range []string{"request", "requirements", "plan", "run", "result", "judgment", "proposal", "continuation", "allowed"} {
		for _, body := range []string{"null", "{}", "{"} {
			t.Run(kind+body, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(folders[kind], "1.json")
				testutil.Put(t, dir, path, body)
				_, err := List(dir, kind)
				testutil.Fails(t, err, filepath.Join(dir, path))
			})
		}
	}
	for _, kind := range []string{"question", "answer"} {
		for _, body := range []string{"null", "{}", "{"} {
			dir := t.TempDir()
			testutil.Put(t, dir, folders[kind]+"/Q1.json", body)
			_, err := ReadKeyed(dir, kind, "Q1")
			testutil.Fails(t, err, QuestionPath(dir, kind, "Q1"))
		}
	}
}

func TestRequestIssueRejectsInvalidOptionalField(t *testing.T) {
	for _, value := range []any{nil, "42", -1, 1.5} {
		dir := t.TempDir()
		r := request(dir)
		r.Entry["issue"] = value
		testutil.OK(t, os.Remove(r.Path))
		testutil.OK(t, writeNew(r.Path, r.Entry))
		_, err := RequestIssue(dir)
		testutil.Fails(t, err, r.Path)
	}
	dir := t.TempDir()
	testutil.Equal(t, must(RequestIssue(dir)), 0)
	request(dir)
	testutil.Equal(t, must(RequestIssue(dir)), 0)
	r := request(dir)
	r.Entry["issue"] = 0
	testutil.OK(t, os.Remove(r.Path))
	testutil.OK(t, writeNew(r.Path, r.Entry))
	testutil.Equal(t, must(RequestIssue(dir)), 0)
}

func TestKeyedHistoryRejectsMissingQuestionAndKeepsUnansweredState(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("One", []string{"requirements"}), ask("Two", []string{"requirements"}), ask("Three", []string{"requirements"})}})
	testutil.Equal(t, len(must(ListKeyed(dir, "question"))), 3)
	testutil.Equal(t, must(ListKeyed(dir, "answer")), []Document{})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1")), Document(nil))
	testutil.OK(t, os.Remove(QuestionPath(dir, "question", "Q2")))
	for _, kind := range []string{"question", "answer"} {
		_, err := ListKeyed(dir, kind)
		testutil.Fails(t, err, QuestionPath(dir, "question", "Q2"))
	}
}

func TestLatestVersionsRejectsInvalidRequirementsVersion(t *testing.T) {
	for _, version := range []any{nil, "1", 0, 1.5} {
		dir := newRecord(t)
		r := requirements(dir)
		r.Entry["version"] = version
		testutil.OK(t, os.Remove(r.Path))
		testutil.OK(t, writeNew(r.Path, r.Entry))
		_, err := LatestVersions(dir)
		testutil.Fails(t, err, r.Path)
		testutil.Fails(t, err, "/version")
	}
}
