package record

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
)

type Document = map[string]any

var Workflows = []string{"refine", "requirements", "plan", "implement", "judge", "fix", "pr", "sync", "retro", "optimize"}

func StageOf(workflow string) (int, error) {
	stage := slices.Index(Workflows, workflow)
	if stage < 0 {
		return -1, fmt.Errorf("unknown workflow %s; one of %s", workflow, strings.Join(Workflows, ", "))
	}
	return stage, nil
}

type RecordError struct {
	Message string
	Errors  []string
}

func (e *RecordError) Error() string {
	if len(e.Errors) > 0 {
		return e.Message + ":\n" + strings.Join(e.Errors, "\n")
	}
	return e.Message
}

var folders = map[string]string{"request": "request", "proposal": "proposals", "requirements": "requirements", "plan": "plans", "judgment": "judgments", "allowed": "allowed", "run": "runs", "result": "results", "question": "questions", "answer": "answers", "continuation": "continuations"}

func EncodeURIComponent(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.!~*'()", rune(c)) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
func RecordDir(stateDir, branch string) string {
	return filepath.Join(stateDir, "records", EncodeURIComponent(branch))
}
func numbers(folder string) ([]int, error) {
	entries, err := os.ReadDir(folder)
	if os.IsNotExist(err) {
		return []int{}, nil
	}
	if err != nil {
		return nil, err
	}
	ns := []int{}
	for _, entry := range entries {
		if regexp.MustCompile(`^\d+\.json$`).MatchString(entry.Name()) {
			n, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
			if err != nil {
				return nil, err
			}
			ns = append(ns, n)
		}
	}
	sort.Ints(ns)
	return ns, nil
}
func readJSON(file string) (Document, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc Document
	err = json.Unmarshal(data, &doc)
	return doc, err
}
func readBody(file string) (Document, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return object(value), nil
}

func readRecord(path, kind string) (Document, error) {
	doc, err := readJSON(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	problems, err := Validate(kind, doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(problems) > 0 {
		return nil, &RecordError{Message: path + ": not a valid " + kind, Errors: problems}
	}
	return doc, nil
}

func List(dir, kind string) ([]Document, error) {
	folder, ok := folders[kind]
	if !ok {
		return nil, fmt.Errorf("unknown record kind %s", kind)
	}
	ns, err := numbers(filepath.Join(dir, folder))
	if err != nil {
		return nil, err
	}
	docs := []Document{}
	for _, n := range ns {
		doc, err := readRecord(filepath.Join(dir, folder, strconv.Itoa(n)+".json"), kind)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
func Latest(dir, kind string) (Document, error) {
	docs, err := List(dir, kind)
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	return docs[len(docs)-1], nil
}
func LatestVersions(dir string) (Document, error) {
	result := Document{"requirements_version": nil, "plan_version": nil}
	for _, kind := range []string{"requirements", "plan"} {
		doc, err := Latest(dir, kind)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			result[kind+"_version"] = doc["version"]
		}
	}
	return result, nil
}
func writeNew(file string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func clone(doc Document) Document {
	result := Document{}
	for k, v := range doc {
		result[k] = v
	}
	return result
}

type Appended struct {
	Kind   string   `json:"kind"`
	Number int      `json:"number"`
	Path   string   `json:"path"`
	Entry  Document `json:"entry"`
}

func Append(dir, kind string, value Document) (Appended, error) {
	folder, ok := folders[kind]
	if !ok {
		return Appended{}, fmt.Errorf("unknown record kind %s", kind)
	}
	folder = filepath.Join(dir, folder)
	ns, err := numbers(folder)
	if err != nil {
		return Appended{}, err
	}
	n := 1
	if len(ns) > 0 {
		n = ns[len(ns)-1] + 1
	}
	for ; ; n++ {
		entry := clone(value)
		if kind == "proposal" || kind == "requirements" || kind == "plan" {
			entry["version"] = n
		}
		problems, err := Validate(kind, entry)
		if err != nil {
			return Appended{}, err
		}
		if len(problems) > 0 {
			return Appended{}, &RecordError{"not a valid " + kind, problems}
		}
		path := filepath.Join(folder, strconv.Itoa(n)+".json")
		if err := writeNew(path, entry); err != nil {
			if os.IsExist(err) {
				continue
			}
			return Appended{}, err
		}
		return Appended{kind, n, path, entry}, nil
	}
}
func QuestionPath(dir, kind, id string) string { return filepath.Join(dir, folders[kind], id+".json") }
func ReadKeyed(dir, kind, id string) (Document, error) {
	doc, err := readRecord(QuestionPath(dir, kind, id), kind)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return doc, err
}
func WriteKeyed(dir, kind, id string, value Document) error {
	problems, err := Validate(kind, value)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return &RecordError{"not a valid " + kind, problems}
	}
	if err := writeNew(QuestionPath(dir, kind, id), value); err != nil {
		if os.IsExist(err) {
			return &RecordError{Message: kind + " " + id + " is already recorded"}
		}
		return err
	}
	return nil
}
func QuestionCount(dir string) (int, error) {
	entries, err := os.ReadDir(filepath.Join(dir, folders["question"]))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	numbers := []int{}
	for _, entry := range entries {
		if regexp.MustCompile(`^Q\d+\.json$`).MatchString(entry.Name()) {
			n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "Q"), ".json"))
			if err != nil || n < 1 || entry.Name() != fmt.Sprintf("Q%d.json", n) {
				return 0, fmt.Errorf("%s: invalid question ID", filepath.Join(dir, folders["question"], entry.Name()))
			}
			numbers = append(numbers, n)
		}
	}
	sort.Ints(numbers)
	for i, n := range numbers {
		if n != i+1 {
			return 0, fmt.Errorf("%s: missing question record", QuestionPath(dir, "question", fmt.Sprintf("Q%d", i+1)))
		}
	}
	return len(numbers), nil
}
func ListKeyed(dir, kind string) ([]Document, error) {
	count, err := QuestionCount(dir)
	if err != nil {
		return nil, err
	}
	docs := []Document{}
	for n := 1; n <= count; n++ {
		if kind != "question" {
			if _, err := ReadKeyed(dir, "question", fmt.Sprintf("Q%d", n)); err != nil {
				return nil, err
			}
		}
		doc, err := ReadKeyed(dir, kind, fmt.Sprintf("Q%d", n))
		if err != nil {
			return nil, err
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

type HistoryData struct {
	Runs          []Document
	Results       []Document
	Continuations []Document
}

func History(dir string) (HistoryData, error) {
	h := HistoryData{}
	var err error
	h.Runs, err = List(dir, "run")
	if err != nil {
		return h, err
	}
	h.Results, err = List(dir, "result")
	if err != nil {
		return h, err
	}
	h.Continuations, err = List(dir, "continuation")
	if err != nil {
		return h, err
	}
	for _, r := range h.Results {
		for i, run := range h.Runs {
			if r["run_id"] == run["run_id"] {
				r["run_index"] = i
			}
		}
	}
	return h, nil
}
func ValidResults(h HistoryData, starting string) ([]Document, error) {
	result := []Document{}
	start := len(Workflows)
	if starting != "" {
		var err error
		start, err = StageOf(starting)
		if err != nil {
			return nil, err
		}
	}
	for _, r := range h.Results {
		stage, err := StageOf(str(r["workflow"]))
		if err != nil {
			return nil, err
		}
		if start < stage {
			continue
		}
		index, ok := r["run_index"].(int)
		if !ok {
			index = -1
		}
		valid := true
		for _, run := range h.Runs[index+1:] {
			s, err := StageOf(str(run["workflow"]))
			if err != nil {
				return nil, err
			}
			if s < stage {
				valid = false
				break
			}
		}
		if valid {
			result = append(result, r)
		}
	}
	return result, nil
}

type ContextData struct {
	Dir              string
	Branch           string
	RunID            string
	Cwd              string
	BaseBranch       string
	BaseBranchSource string
}

func Context(env cli.Env) (ContextData, error) {
	if env.Getenv("STATE_DIR") == "" || env.Getenv("WORKFLOW_ID") == "" {
		return ContextData{}, &RecordError{Message: "STATE_DIR and WORKFLOW_ID must be set"}
	}
	branch, err := gitx.Run(env.Cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ContextData{}, err
	}
	if branch == "HEAD" {
		return ContextData{}, &RecordError{Message: "HEAD is detached; the record is kept per branch"}
	}
	base, source, err := gitx.BaseBranch(env.Cwd, env.Getenv("BASE_BRANCH"))
	if err != nil {
		return ContextData{}, err
	}
	return ContextData{Dir: RecordDir(env.Getenv("STATE_DIR"), branch), Branch: branch, RunID: env.Getenv("WORKFLOW_ID"), Cwd: env.Cwd, BaseBranch: base, BaseBranchSource: source}, nil
}

type Answer struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

func ParseAnswers(text string) ([]Answer, error) {
	if strings.TrimSpace(text) == "" {
		return []Answer{}, nil
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, &RecordError{Message: "the answers are not JSON: " + err.Error()}
	}
	entries, ok := value.([]any)
	if ok {
		answers := []Answer{}
		for _, entry := range entries {
			object, ok := entry.(map[string]any)
			if !ok || len(object) != 2 {
				break
			}
			id, a := object["question_id"].(string)
			answer, b := object["answer"].(string)
			if !a || !b || strings.TrimSpace(answer) == "" {
				break
			}
			answers = append(answers, Answer{id, answer})
		}
		if len(answers) == len(entries) {
			return answers, nil
		}
	}
	return nil, &RecordError{Message: "the answers must be a JSON array of {question_id, answer} with a non-empty answer"}
}

var filled = map[string][]string{"request": {"criteria_commit", "run_id", "recorded_at"}, "proposal": {"version", "request", "run_id", "recorded_at"}, "requirements": {"version", "request", "run_id", "recorded_at"}, "plan": {"version", "requirements_version", "run_id", "recorded_at"}, "judgment": {"criteria_commit", "run_id", "recorded_at"}, "allowed": {"run_id", "recorded_at"}}

func timestamp(now string) string {
	if now != "" {
		return now
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
func Write(ctx ContextData, kind string, body Document, now string) (Appended, error) {
	fields, ok := filled[kind]
	if !ok {
		return Appended{}, &RecordError{Message: "write takes one of request, proposal, requirements, plan, judgment, allowed"}
	}
	if body == nil {
		return Appended{}, &RecordError{Message: "the " + kind + " must be an object"}
	}
	supplied := []string{}
	for _, key := range fields {
		if _, ok := body[key]; ok {
			supplied = append(supplied, key)
		}
	}
	if len(supplied) > 0 {
		return Appended{}, &RecordError{Message: "the record fills " + strings.Join(supplied, ", ") + "; leave them out of the " + kind}
	}
	h, err := History(ctx.Dir)
	if err != nil {
		return Appended{}, err
	}
	started := false
	for _, run := range h.Runs {
		if run["run_id"] == ctx.RunID {
			started = true
		}
	}
	if !started {
		return Appended{}, &RecordError{Message: fmt.Sprintf("run %s did not pass the start conditions, so it does not write the record", ctx.RunID)}
	}
	entry := clone(body)
	entry["run_id"] = ctx.RunID
	entry["recorded_at"] = timestamp(now)
	switch kind {
	case "request", "requirements", "proposal", "judgment":
		requests, err := List(ctx.Dir, "request")
		if err != nil {
			return Appended{}, err
		}
		if kind == "request" {
			var commit any
			if len(requests) > 0 {
				commit = requests[0]["criteria_commit"]
			} else {
				commit, err = gitx.Run(ctx.Cwd, "rev-parse", ctx.BaseBranch)
				if err != nil {
					return Appended{}, err
				}
			}
			entry["criteria_commit"] = commit
		} else {
			if len(requests) == 0 {
				return Appended{}, &RecordError{Message: "no request is recorded yet"}
			}
			if kind == "judgment" {
				entry["criteria_commit"] = requests[0]["criteria_commit"]
			} else {
				entry["request"] = len(requests)
			}
		}
	case "plan":
		versions, err := LatestVersions(ctx.Dir)
		if err != nil {
			return Appended{}, err
		}
		if versions["requirements_version"] == nil {
			return Appended{}, &RecordError{Message: "no requirements are recorded yet"}
		}
		entry["requirements_version"] = versions["requirements_version"]
	case "allowed":
		answer, err := ReadKeyed(ctx.Dir, "answer", str(body["question_id"]))
		if err != nil {
			return Appended{}, err
		}
		if answer == nil {
			return Appended{}, &RecordError{Message: fmt.Sprintf("question %v has no answer, so nothing was allowed by it", body["question_id"])}
		}
	}
	return Append(ctx.Dir, kind, entry)
}
func str(value any) string      { s, _ := value.(string); return s }
func object(value any) Document { m, _ := value.(map[string]any); return m }
func array(value any) []any {
	a, _ := value.([]any)
	if a == nil {
		if docs, ok := value.([]Document); ok {
			a = make([]any, len(docs))
			for i, doc := range docs {
				a[i] = doc
			}
		}
	}
	return a
}
func stringsOf(value any) []string {
	if ss, ok := value.([]string); ok {
		return ss
	}
	result := []string{}
	for _, item := range array(value) {
		result = append(result, str(item))
	}
	return result
}
