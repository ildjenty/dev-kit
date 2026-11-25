package status

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func statusFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files[path] = info.Mode().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] += string(data)
		}
		return nil
	}))
	return files
}

func TestStatusDistinguishesUnreadableAnswersFromMissingAndAnswered(t *testing.T) {
	for _, tc := range []struct {
		name, answer, extra, cause string
		questions                  []string
	}{
		{name: "missing", questions: []string{"Q1"}},
		{name: "answered", answer: "valid", questions: []string{}},
		{name: "malformed", answer: "malformed", cause: "unexpected end of JSON input", questions: []string{}},
		{name: "empty object", answer: "{}", cause: "required property answer", questions: []string{}},
		{name: "null", answer: "null", cause: "must be object", questions: []string{}},
		{name: "read failure", answer: "directory", cause: "is a directory", questions: []string{}},
		{name: "malformed with pending", answer: "malformed", extra: "open_point", cause: "unexpected end of JSON input", questions: []string{"Q2"}},
		{name: "malformed with approval", answer: "malformed", extra: "proposal", cause: "unexpected end of JSON input", questions: []string{"Q2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state := testutil.Repo(t), t.TempDir()
			records := filepath.Join(state, "records")
			dir := branch(t, records, "feature/x")
			questions := []any{map[string]any{"id": "Q1", "text": "Which option?", "target": map[string]any{"kind": "open_point", "ref": "O1"}, "next_workflows": []any{"requirements"}}}
			if tc.extra != "" {
				questions = append(questions, map[string]any{"id": "Q2", "text": "Another question", "target": map[string]any{"kind": tc.extra, "ref": "2"}, "next_workflows": []any{"requirements"}})
			}
			run1(t, dir, "requirements", 1, map[string]any{"outcome": "question", "questions": questions})
			answerPath := filepath.Join(dir, "answers/Q1.json")
			switch tc.answer {
			case "valid":
				writeAnswer(t, dir, "Q1")
			case "malformed":
				testutil.Put(t, dir, "answers/Q1.json", "{")
			case "directory":
				testutil.OK(t, os.MkdirAll(answerPath, 0755))
			case "{}", "null":
				testutil.Put(t, dir, "answers/Q1.json", tc.answer)
			}
			healthy := branch(t, records, "feature/healthy")
			run1(t, healthy, "plan", 2, map[string]any{"outcome": "pass"})
			before := statusFiles(t, state)
			check := func(b Branch) {
				t.Helper()
				testutil.Equal(t, b.Branch, "feature/x")
				testutil.Equal(t, b.Workflow, "requirements")
				testutil.Equal(t, b.Outcome, "question")
				testutil.Equal(t, b.RunID, "requirements-1")
				testutil.Equal(t, b.At, "2026-11-24T00:01:30Z")
				ids := []string{}
				for _, q := range b.Questions {
					ids = append(ids, q.ID)
				}
				testutil.Equal(t, ids, tc.questions)
				if tc.cause == "" {
					testutil.Equal(t, b.Errors, []string{})
					if tc.answer == "valid" {
						testutil.Contains(t, b.Next, "Every question is answered; start requirements")
					} else {
						testutil.Contains(t, b.Next, "Relay Q1")
					}
				} else {
					testutil.Equal(t, len(b.Errors), 1)
					testutil.Contains(t, b.Errors[0], "answer Q1")
					testutil.Contains(t, b.Errors[0], answerPath)
					testutil.Contains(t, b.Errors[0], tc.cause)
					testutil.Equal(t, b.Next, "Resolve the unreadable answers first, then check status again.")
				}
			}
			read := testutil.Must(Read(records, "feature/x", 3))
			testutil.Equal(t, len(read), 1)
			check(read[0])
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
			registry := cli.New("", Commands())
			for _, filter := range []string{"", "feature/x"} {
				args := []string{"status", "--json"}
				if filter != "" {
					args = append(args, filter)
				}
				out.Reset()
				testutil.Equal(t, registry.Execute(args, env), 0)
				var parsed struct{ Branches []Branch }
				testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
				count := 2
				if filter != "" {
					count = 1
				}
				testutil.Equal(t, len(parsed.Branches), count)
				for _, b := range parsed.Branches {
					if b.Branch == "feature/x" {
						check(b)
					} else {
						testutil.Equal(t, b.Errors, []string{})
						testutil.Equal(t, b.Next, "Start implement on feature/healthy.")
					}
				}
				out.Reset()
				args = []string{"status"}
				if filter != "" {
					args = append(args, filter)
				}
				testutil.Equal(t, registry.Execute(args, env), 0)
				if tc.cause != "" {
					testutil.Contains(t, out.String(), "  unreadable: "+read[0].Errors[0])
					testutil.Contains(t, out.String(), "  next: "+read[0].Next)
					testutil.Equal(t, strings.Contains(out.String(), "Q1: Which option?"), false)
				} else {
					testutil.Equal(t, strings.Contains(out.String(), "unreadable:"), false)
				}
			}
			testutil.Equal(t, stderr.String(), "")
			testutil.Equal(t, statusFiles(t, state), before)
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
		})
	}
}

func TestStatusPassingJudgeDistinguishesReviewOnlyFromDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, next   string
		requirements any
	}{
		{"review only", "The review is complete. Relay the report to the owner.", nil},
		{"delivery", "Start pr on feature/x.", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state := testutil.Repo(t), t.TempDir()
			records := filepath.Join(state, "records")
			dir := branch(t, records, "feature/x")
			inputs := map[string]any{"requirements_version": tc.requirements, "plan_version": tc.requirements}
			run1(t, dir, "judge", 1, map[string]any{"outcome": "pass", "inputs": inputs})
			before := statusFiles(t, state)
			read := testutil.Must(Read(records, "feature/x", 3))
			testutil.Equal(t, read[0].Next, tc.next)
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
			registry := cli.New("", Commands())
			testutil.Equal(t, registry.Execute([]string{"status", "--json", "feature/x"}, env), 0)
			var parsed struct{ Branches []Branch }
			testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
			testutil.Equal(t, len(parsed.Branches), 1)
			testutil.Equal(t, parsed.Branches[0].Workflow, "judge")
			testutil.Equal(t, parsed.Branches[0].Outcome, "pass")
			testutil.Equal(t, parsed.Branches[0].Errors, []string{})
			testutil.Equal(t, parsed.Branches[0].Next, tc.next)
			out.Reset()
			testutil.Equal(t, registry.Execute([]string{"status", "feature/x"}, env), 0)
			testutil.Contains(t, out.String(), "feature/x: judge pass")
			testutil.Contains(t, out.String(), "  next: "+tc.next)
			testutil.Equal(t, stderr.String(), "")
			testutil.Equal(t, statusFiles(t, state), before)
			testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
		})
	}
}
