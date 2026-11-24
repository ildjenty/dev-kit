package refine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

type doc = map[string]any

func obj(v any) doc    { m, _ := v.(map[string]any); return m }
func str(v any) string { s, _ := v.(string); return s }
func num(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
func array(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []doc:
		r := []any{}
		for _, d := range a {
			r = append(r, d)
		}
		return r
	case []string:
		r := []any{}
		for _, s := range a {
			r = append(r, s)
		}
		return r
	}
	return []any{}
}
func items(v any) []doc {
	r := []doc{}
	for _, v := range array(v) {
		r = append(r, obj(v))
	}
	return r
}
func stringsOf(v any) []string {
	r := []string{}
	for _, s := range array(v) {
		r = append(r, str(s))
	}
	return r
}
func clone(d doc) doc {
	r := doc{}
	for k, v := range d {
		r[k] = v
	}
	return r
}
func find(ds []doc, id string) doc {
	for _, d := range ds {
		if d["id"] == id {
			return d
		}
	}
	return nil
}
func has(v any, s string) bool           { return slices.Contains(stringsOf(v), s) }
func file(artifacts, name string) string { return filepath.Join(artifacts, "refine", name) }
func readJSON(path string) (doc, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var v doc
	e = json.Unmarshal(b, &v)
	return v, e
}
func readDraft(artifacts string) (any, error) {
	b, e := os.ReadFile(file(artifacts, "draft.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var d any
	if json.Unmarshal(b, &d) != nil {
		return nil, nil
	}
	return d, nil
}
func put(path, text string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, []byte(text), 0644)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return put(path, string(b)+"\n")
}
func resolve(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}
func inside(path, dir string) bool { return strings.HasPrefix(path, dir+"/") }
func changes(cwd string) ([]string, error) {
	r := []string{}
	for _, args := range [][]string{{"diff", "--cached", "--name-only", "--no-renames", "-z"}, {"diff", "HEAD", "--name-only", "--no-renames", "-z"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		s, e := gitx.Run(cwd, args...)
		if e != nil {
			return nil, e
		}
		for _, p := range strings.Split(s, "\x00") {
			if p != "" && !slices.Contains(r, p) {
				r = append(r, p)
			}
		}
	}
	return r, nil
}
func start(ctx record.ContextData, env cli.Env) (record.StartOutput, error) {
	stopped := func(reason string) record.StartOutput {
		return record.StartOutput{Stopped: true, Stage: "start", Reason: reason, Details: ctx.Dir, Remaining: []string{}}
	}
	reason, err := record.KitStartReason(env, "refine")
	if err != nil {
		return record.StartOutput{}, err
	}
	if reason != "" {
		return stopped(reason), nil
	}
	if !strings.HasPrefix(ctx.Branch, "refine/") {
		return stopped("refine requires a refine/<name> branch"), nil
	}
	if env.Getenv("INPUTS_APPROVE") == "" {
		if _, e := selectedTopic(ctx, env, env.Getenv("INPUTS_TOPIC")); e != nil {
			return stopped(e.Error()), nil
		}
	}
	latest, e := record.Latest(ctx.Dir, "result")
	if e != nil {
		return record.StartOutput{}, e
	}
	if latest["outcome"] == "pass" && env.Getenv("INPUTS_TOPIC") == "" && env.Getenv("INPUTS_APPROVE") == "" && strings.TrimSpace(env.Getenv("INPUTS_OWNER_WORDS")) == "" && strings.TrimSpace(env.Getenv("INPUTS_MEMO")) == "" {
		return stopped("select a topic, or provide a new request"), nil
	}
	approve := env.Getenv("INPUTS_APPROVE")
	answers, e := record.ParseAnswers(env.Getenv("INPUTS_ANSWERS"))
	if e != nil {
		return record.StartOutput{}, e
	}
	if approve != "" {
		problems, e := approvalErrors(ctx, approve)
		if e != nil {
			return record.StartOutput{}, e
		}
		if len(problems) > 0 {
			return stopped(strings.Join(problems, "; ")), nil
		}
		if len(answers) > 0 || env.Getenv("INPUTS_OWNER_WORDS") != "" || env.Getenv("INPUTS_SUPPLEMENT") != "" || env.Getenv("INPUTS_MEMO") != "" || env.Getenv("INPUTS_TOPIC") != "" {
			return stopped("approve cannot be combined with a topic, request, memo or answers"), nil
		}
		r, e := record.Latest(ctx.Dir, "result")
		if e != nil {
			return record.StartOutput{}, e
		}
		qid := str(items(r["questions"])[0]["id"])
		recorded, e := record.ReadKeyed(ctx.Dir, "answer", qid)
		if e != nil {
			return record.StartOutput{}, e
		}
		if recorded == nil {
			answers = append(answers, record.Answer{QuestionID: qid, Answer: "approve=" + approve})
		}

	}
	common, e := gitx.Run(ctx.Cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return record.StartOutput{}, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return record.StartOutput{}, e
	}
	base, e := gitx.Run(ctx.Cwd, "rev-parse", ctx.BaseBranch)
	if e != nil {
		return record.StartOutput{}, e
	}
	c, e := config.Load(ctx.Cwd)
	if e != nil {
		return record.StartOutput{}, e
	}
	workflows, e := gitx.Run(filepath.Dir(common), "status", "--porcelain", "--", ".archon/workflows")
	if e != nil {
		return record.StartOutput{}, e
	}
	worktree, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return record.StartOutput{}, e
	}
	lines := func(s string) []string {
		if s == "" {
			return []string{}
		}
		return strings.Split(s, "\n")
	}
	return record.Start(ctx.Dir, "refine", record.StartOptions{RunID: ctx.RunID, Commit: head, BaseCommit: base, ChangeID: func() (*string, error) {
		g, e := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
		if e != nil {
			return nil, e
		}
		return g.ChangeID("HEAD")
	}, Answers: answers, Continuation: env.Getenv("INPUTS_CONTINUE"), Limit: &c.Limits.Restarts, WorkflowsChanged: lines(workflows), WorktreeChanged: lines(worktree)})
}
func prepare(ctx record.ContextData, env cli.Env, artifacts, ownerWords, supplement, memo string, topicID ...string) (doc, error) {
	displayArtifacts := artifacts
	artifacts = resolve(ctx.Cwd, artifacts)
	c, e := config.Load(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	previous, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	proposals, e := record.List(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	results, e := record.List(ctx.Dir, "result")
	if e != nil {
		return nil, e
	}
	approvedVersion := 0
	for _, result := range results {
		if result["workflow"] == "refine" && result["outcome"] == "pass" {
			approvedVersion = max(approvedVersion, num(result["proposal_version"]))
		}
	}
	var pending doc
	for i := len(proposals) - 1; i >= 0; i-- {
		if num(proposals[i]["version"]) <= approvedVersion {
			break
		}
		if obj(proposals[i]["content"])["commit"] != nil {
			pending = proposals[i]
			break
		}
	}

	selected := ""
	if len(topicID) > 0 {
		selected = topicID[0]
	}
	topic, e := selectedTopic(ctx, env, selected)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(ownerWords) != "" || strings.TrimSpace(supplement) != "" || strings.TrimSpace(memo) != "" || topic != nil && previous == nil {
		if _, e = record.Write(ctx, "request", doc{"owner_words": ownerWords, "supplement": supplement}, ""); e != nil {
			return nil, e
		}
	}
	requests, e := record.List(ctx.Dir, "request")
	if e != nil {
		return nil, e
	}
	if memo == "" {
		memo = str(obj(previous["content"])["memo"])
	}
	owner := false
	for _, r := range requests {
		owner = owner || strings.TrimSpace(str(r["owner_words"])) != ""
	}
	if len(requests) == 0 || (!owner && strings.TrimSpace(memo) == "" && topic == nil) {
		return nil, fmt.Errorf("refine needs owner_words or a discussion memo")
	}
	source, e := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: str(requests[0]["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
	if e != nil {
		return nil, e
	}
	data, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	rs := []doc{}
	for i, r := range requests {
		rs = append(rs, doc{"number": i + 1, "owner_words": r["owner_words"], "supplement": r["supplement"]})
	}
	answers, e := record.ListKeyed(ctx.Dir, "answer")
	if e != nil {
		return nil, e
	}
	as := []doc{}
	for _, a := range answers {
		as = append(as, doc{"question_id": a["question_id"], "question": a["question"], "target": a["target"], "answer": a["answer"]})
	}
	fixed := filepath.Join(displayArtifacts, "criteria", source.Paths[0])
	mapping, e := concerns.Load(filepath.Join(source.Dir, source.Paths[0]))
	if e != nil {
		return nil, e
	}
	if topic == nil {
		text := ownerWords
		if text == "" && len(rs) > 0 {
			text = str(rs[0]["owner_words"])
		}
		if text == "" {
			text = memo
		}
		topic = doc{"number": 0, "title": text, "body": text}
	}
	input := doc{"pending_proposal": pending, "previous_unapproved": pending != nil, "topic": topic, "requests": rs, "memo": memo, "answers": as, "previous": previous, "decisions": data.Decisions, "language": c.Language, "config": c, "criteria": fixed, "folders": doc{"decisions": c.Decisions, "standards": c.Standards}, "next_proposal_version": num(previous["version"]) + 1, "judge_materials": mapping.Materials(fixed, "judge:refine", "plan"), "writer_materials": mapping.Materials(fixed, "writer:refine", "plan")}
	base, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if pending != nil {
		base = str(obj(pending["content"])["base_commit"])
	}
	input["proposal_base"] = base
	for _, name := range []string{"draft", "judgment"} {
		b, err := schemas.ReadFile("schemas/" + name + ".schema.json")
		if err != nil {
			return nil, err
		}
		if err = put(file(artifacts, name+".schema.json"), string(b)); err != nil {
			return nil, err
		}
	}
	if e = writeJSON(file(artifacts, "input.json"), input); e != nil {
		return nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "baseline.json"), doc{"commit": head, "decisions_hash": decisionHash(data.Decisions)}); e != nil {
		return nil, e
	}
	return doc{"input": file(displayArtifacts, "input.json"), "criteria": input["criteria"]}, nil
}
func approvalStart(ctx record.ContextData, version string) error {
	h, err := record.History(ctx.Dir)
	if err != nil {
		return err
	}
	result, err := record.Latest(ctx.Dir, "result")
	if err != nil {
		return err
	}
	q := items(result["questions"])[0]
	answer, err := record.ReadKeyed(ctx.Dir, "answer", str(q["id"]))
	if err != nil {
		return err
	}
	progress, err := issues.ReadCreation(creationPath(ctx, version))
	if err != nil {
		return err
	}
	authorizationRun := ctx.RunID
	if progress.RunID != "" {
		authorizationRun = progress.RunID
	}
	valid := false
	for _, run := range h.Runs {
		if run["run_id"] == authorizationRun && run["workflow"] == "refine" && has(run["answers"], str(q["id"])) {
			valid = true
		}
	}
	if !valid || answer["run_id"] != authorizationRun || answer["workflow"] != "refine" || answer["answer"] != "approve="+version || obj(answer["target"])["kind"] != "proposal" || obj(answer["target"])["ref"] != version || answer["commit"] != result["commit"] {
		return fmt.Errorf("approval requires a successful refine start with the recorded approval answer")
	}
	problems, err := record.StartConditions(ctx.Dir, "refine", record.ConditionOptions{Commit: str(result["commit"])})
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return &record.RecordError{Message: "approval start conditions do not hold", Errors: problems}
	}
	options, err := resultOptions(ctx)
	if err != nil {
		return err
	}
	_, err = record.BuildResult(ctx.Dir, "refine", doc{"outcome": "pass"}, options)
	return err
}
func decisionHash(ds []doc) string {
	data, _ := json.Marshal(ds)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func selectedTopic(ctx record.ContextData, env cli.Env, value string) (doc, error) {
	if value == "" {
		previous, err := record.Latest(ctx.Dir, "proposal")
		if err != nil {
			return nil, err
		}
		result, err := record.Latest(ctx.Dir, "result")
		if err != nil {
			return nil, err
		}
		if result["outcome"] != "pass" {
			return obj(obj(previous["content"])["topic"]), nil
		}
		return nil, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return nil, fmt.Errorf("topic must be a positive issue number")
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	env.Cwd = ctx.Cwd
	topic, err := (issues.Client{Env: env}).Read(number)
	if err != nil {
		return nil, err
	}
	if topic.State != "open" || !slices.Contains(topic.Labels, c.Issues.Labels.Topic) {
		return nil, fmt.Errorf("topic #%d must be open and carry topic label %q", number, c.Issues.Labels.Topic)
	}
	data, err := json.Marshal(topic)
	if err != nil {
		return nil, err
	}
	var out doc
	err = json.Unmarshal(data, &out)
	return out, err
}
func resultOptions(ctx record.ContextData) (record.ResultOptions, error) {
	base, err := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ctx.BaseBranch)
	if err != nil {
		return record.ResultOptions{}, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	return record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}, err
}
func creationPath(ctx record.ContextData, version string) string {
	return filepath.Join(ctx.Dir, "issue-creation", version+".json")
}
func approvalErrors(ctx record.ContextData, version string) ([]string, error) {
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	r, err := record.Latest(ctx.Dir, "result")
	if err != nil {
		return nil, err
	}
	if !regexp.MustCompile(`^[1-9]\d*$`).MatchString(version) || strconv.Itoa(num(p["version"])) != version {
		return []string{"approve must name the latest proposal version"}, nil
	}
	qs := items(r["questions"])
	content := obj(p["content"])
	if content["ready"] != true || r["workflow"] != "refine" || r["run_id"] != p["run_id"] || r["outcome"] != "question" || len(qs) != 1 || obj(qs[0]["target"])["kind"] != "proposal" || obj(qs[0]["target"])["ref"] != version {
		return []string{"the latest proposal result did not ask for approval"}, nil
	}
	progress, err := issues.ReadCreation(creationPath(ctx, version))
	if err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	expected := str(content["commit"])
	if progress.Commit != "" {
		expected = progress.Commit
	}
	if expected != head {
		return []string{"HEAD changed after the proposal asked for approval"}, nil
	}
	status, err := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if status != "" {
		return []string{"approval requires a clean worktree"}, nil
	}
	return []string{}, nil
}
func bindingPath(path string, c config.Config) bool {
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "\\") {
		return false
	}
	if inside(path, c.Decisions) {
		return false
	}
	if inside(path, c.Standards) || path == ".devkit/config.json" || filepath.Base(path) == "CLAUDE.md" || filepath.Base(path) == "AGENTS.md" {
		return true
	}
	for _, root := range []string{".claude/skills", ".agents/skills"} {
		if inside(path, root) {
			return true
		}
	}
	return false
}

func draftErrors(ctx record.ContextData, artifacts string, value any) ([]string, error) {
	errors, err := schema("draft", value)
	if err != nil || len(errors) > 0 {
		return errors, err
	}
	artifacts = resolve(ctx.Cwd, artifacts)
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	baseline, err := readJSON(file(artifacts, "baseline.json"))
	if err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if head != baseline["commit"] {
		errors = append(errors, "writer changed HEAD; only refine check may commit")
	}
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	if baseline["decisions_hash"] != decisionHash(data.Decisions) {
		errors = append(errors, "writer edited decision records directly")
	}
	draft := obj(value)
	ds, err := issues.ReadDrafts(draft)
	if err != nil {
		return nil, err
	}
	errors = append(errors, ds.Errors()...)
	known := map[string]bool{}
	graph := append([]doc{}, data.Decisions...)
	pending := obj(obj(input["pending_proposal"])["content"])
	for _, old := range stringsOf(pending["decision_ids"]) {
		graph = slices.DeleteFunc(graph, func(d doc) bool { return d["id"] == old && d["status"] == "proposed" })
	}
	for _, d := range graph {
		known[str(d["id"])] = true
	}
	for _, d := range items(draft["decisions"]) {
		key := str(d["key"])
		if known[key] {
			errors = append(errors, "duplicate decision key: "+key)
		}
		known[key] = true
		g := clone(d)
		g["id"] = key
		graph = append(graph, g)
	}
	errors = append(errors, decisions.GraphErrors(graph)...)
	successors := map[string]bool{}
	for _, d := range items(draft["decisions"]) {
		if d["decided_by"] == "owner" && d["source"] == "derived" {
			errors = append(errors, str(d["key"])+": a derived choice cannot be attributed to the owner")
		}
		for _, old := range stringsOf(d["supersedes"]) {
			if find(data.Decisions, old) == nil {
				errors = append(errors, "missing superseded decision "+old)
			}
			if successors[old] {
				errors = append(errors, old+": more than one successor")
			}
			successors[old] = true
		}
		for _, id := range decisions.Citations(str(d["body"])) {
			if !known[id] {
				errors = append(errors, "missing citation "+id)
			}
		}
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		errors = append(errors, err.Error())
		return errors, nil
	}
	raw, err := json.Marshal(input["config"])
	if err != nil {
		return nil, err
	}
	var original config.Config
	if err = json.Unmarshal(raw, &original); err != nil {
		return nil, err
	}
	original.Checks = c.Checks
	if !reflect.DeepEqual(original, c) {
		errors = append(errors, "config edits may change only checks")
	}
	traced := map[string]bool{}
	for _, binding := range items(draft["bindings"]) {
		path := str(binding["path"])
		if traced[path] {
			errors = append(errors, "duplicate binding path: "+path)
		}
		traced[path] = true
		if !bindingPath(path, original) {
			errors = append(errors, "binding is outside the allowed paths: "+path)
		}
		if !known[str(binding["decision"])] {
			errors = append(errors, "binding has no decision: "+path)
		}
	}
	paths, err := gitx.Run(ctx.Cwd, "diff", "--name-only", "--no-renames", "-z", str(input["proposal_base"]), "--")
	if err != nil {
		return nil, err
	}
	working, err := changes(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	edited := append(strings.Split(paths, "\x00"), working...)
	machinePaths := []string{}
	for _, id := range stringsOf(pending["decision_ids"]) {
		machinePaths = append(machinePaths, str(obj(input["folders"])["decisions"])+"/"+id+".md")
	}
	current, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	if current["run_id"] == ctx.RunID {
		for _, id := range stringsOf(obj(current["content"])["decision_ids"]) {
			machinePaths = append(machinePaths, c.Decisions+"/"+id+".md")
		}
	}
	for _, path := range edited {
		if path == "" || slices.Contains(machinePaths, path) && !slices.Contains(working, path) {
			continue
		}
		if !bindingPath(path, original) || !traced[path] {
			errors = append(errors, "edited outside the traced binding changes: "+path)
		}
		info, err := os.Lstat(filepath.Join(ctx.Cwd, path))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && !info.Mode().IsRegular() {
			errors = append(errors, "edited path must be a regular file: "+path)
		}
	}
	staged, err := gitx.Run(ctx.Cwd, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	unstaged, err := gitx.Run(ctx.Cwd, "diff", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(staged, "\x00") {
		if path != "" && slices.Contains(strings.Split(unstaged, "\x00"), path) {
			errors = append(errors, "index and worktree disagree: "+path)
		}
	}
	if _, err = concerns.Load(filepath.Join(ctx.Cwd, c.Standards)); err != nil {
		errors = append(errors, err.Error())
	}
	approval, err := os.ReadFile(file(artifacts, "approval.txt"))
	if os.IsNotExist(err) {
		errors = append(errors, "approval.txt is required in the configured language")
	} else if err != nil {
		return nil, err
	} else {
		text := string(approval)
		if !strings.Contains(text, fmt.Sprintf("approve=%d", num(input["next_proposal_version"]))) || !strings.Contains(text, "answers") {
			errors = append(errors, "approval.txt must explain approve=<version> and answers")
		}
		for _, group := range []string{"decisions", "units", "topics"} {
			for _, item := range items(draft[group]) {
				if !strings.Contains(text, str(item["key"])) {
					errors = append(errors, "approval.txt omits "+str(item["key"]))
				}
			}
		}
		for path := range traced {
			if !strings.Contains(text, path) {
				errors = append(errors, "approval.txt omits binding "+path)
			}
		}
	}
	return errors, nil
}

func recordText(front doc, keys []string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range keys {
		value, _ := json.Marshal(front[key])
		fmt.Fprintf(&b, "%s: %s\n", key, value)
	}
	b.WriteString("---\n" + strings.TrimSpace(body) + "\n")
	return b.String()
}
func decisionText(d doc) string {
	return recordText(d, []string{"id", "title", "status", "decided_by", "depends_on", "supersedes", "request"}, str(d["body"]))
}
func materialize(ctx record.ContextData, draft, input doc) (doc, error) {
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	previous := obj(obj(input["pending_proposal"])["content"])
	mapping := doc{}
	for _, d := range items(draft["decisions"]) {
		key := str(d["key"])
		id := str(obj(previous["mapping"])[key])
		if old := find(data.Decisions, id); old == nil || old["status"] != "proposed" {
			id = ""
		}
		if id == "" {
			id, err = decisions.NextID("decision", ctx.Branch, key, data)
			if err != nil {
				return nil, err
			}
			data.Decisions = append(data.Decisions, doc{"id": id})
		}
		mapping[key] = id
	}
	ids, writes, removals := []string{}, map[string]string{}, []string{}
	for _, d := range items(draft["decisions"]) {
		d = clone(d)
		id := str(mapping[str(d["key"])])
		d["id"], d["status"], d["request"] = id, "proposed", ctx.Branch
		deps := []string{}
		for _, dep := range stringsOf(d["depends_on"]) {
			if mapped := str(mapping[dep]); mapped != "" {
				dep = mapped
			}
			deps = append(deps, dep)
		}
		d["depends_on"] = deps
		body := str(d["body"])
		body = regexp.MustCompile(`\bN[1-9]\d*\b`).ReplaceAllStringFunc(body, func(key string) string {
			if mapped := str(mapping[key]); mapped != "" {
				return mapped
			}
			return key
		})
		d["body"] = body
		writes[str(obj(input["folders"])["decisions"])+"/"+id+".md"] = decisionText(d)
		ids = append(ids, id)
	}
	for _, id := range stringsOf(previous["decision_ids"]) {
		if slices.Contains(ids, id) {
			continue
		}
		d := find(data.Decisions, id)
		if d == nil || d["status"] != "proposed" {
			return nil, fmt.Errorf("cannot withdraw settled record %s", id)
		}
		removals = append(removals, str(d["path"]))
	}
	prospective := decisions.Data{Decisions: []doc{}}
	for _, d := range data.Decisions {
		path := str(d["path"])
		if path == "" || slices.Contains(removals, path) || writes[path] != "" {
			continue
		}
		prospective.Decisions = append(prospective.Decisions, d)
	}
	for path, text := range writes {
		d, err := decisions.Parse(text)
		if err != nil {
			return nil, err
		}
		d["path"] = path
		prospective.Decisions = append(prospective.Decisions, d)
	}
	if errors := decisions.Check(prospective); len(errors) > 0 {
		return nil, &record.RecordError{Message: "proposal failed the decision check", Errors: errors}
	}
	for path, body := range writes {
		if err = put(filepath.Join(ctx.Cwd, path), body); err != nil {
			return nil, err
		}
	}
	for _, path := range removals {
		if err = os.Remove(filepath.Join(ctx.Cwd, path)); err != nil {
			return nil, err
		}
	}
	return doc{"draft": draft, "mapping": mapping, "decision_ids": ids, "topic": input["topic"], "memo": input["memo"], "base_commit": input["proposal_base"], "ready": false}, nil
}
func check(ctx record.ContextData, artifacts string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	draft, err := readDraft(artifacts)
	if err != nil {
		return nil, err
	}
	errors, err := draftErrors(ctx, artifacts, draft)
	if err != nil {
		return nil, err
	}
	if len(errors) == 0 {
		options, err := resultOptions(ctx)
		if err != nil {
			return nil, err
		}
		if _, err = record.BuildResult(ctx.Dir, "refine", doc{"outcome": "pass"}, options); err != nil {
			return nil, err
		}
		committed, p, err := proposal.CommittedErrors(ctx, draft)
		if err != nil {
			return nil, err
		}
		input, err := readJSON(file(artifacts, "input.json"))
		if err != nil {
			return nil, err
		}
		if len(committed) > 0 || num(p["version"]) != num(input["next_proposal_version"]) {
			if num(input["next_proposal_version"]) != num(p["version"])+1 {
				return nil, fmt.Errorf("proposal version already has a commit; start the next draft version")
			}
			content, err := materialize(ctx, obj(draft), input)
			if err != nil {
				return nil, err
			}
			paths, err := changes(ctx.Cwd)
			if err != nil {
				return nil, err
			}
			p, err = proposal.Commit(ctx, paths, fmt.Sprintf("docs: propose refine decisions v%d", num(input["next_proposal_version"])), content)
			if err != nil {
				return nil, err
			}
			data, err := decisions.List(ctx.Cwd)
			if err != nil {
				return nil, err
			}
			if err = writeJSON(file(artifacts, "baseline.json"), doc{"commit": obj(p["content"])["commit"], "decisions_hash": decisionHash(data.Decisions)}); err != nil {
				return nil, err
			}
		}
	}
	out := doc{"valid": len(errors) == 0, "errors": errors}
	return out, writeJSON(file(artifacts, "check.json"), out)
}

var aspects = []string{"coverage", "additions", "open_points", "consistency", "sufficiency", "wording", "bindings", "issues"}

func round(ctx record.ContextData, artifacts string, judgment any) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	d, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, d)
	if e != nil {
		return nil, e
	}
	committed, proposal, e := proposal.CommittedErrors(ctx, d)
	if e != nil {
		return nil, e
	}
	errors = append(errors, committed...)
	je, e := schema("judgment", judgment)
	if e != nil {
		return nil, e
	}
	j := obj(judgment)
	if len(je) == 0 {
		for _, aspect := range aspects {
			n := 0
			for _, c := range items(j["criteria"]) {
				if c["aspect"] == aspect {
					n++
				}
			}
			if n != 1 {
				je = append(je, aspect+": exactly one criterion is required")
			}
		}
	}
	previous, e := readJSON(file(artifacts, "rounds/latest.json"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	n := num(previous["round"]) + 1
	if n > 3 {
		return nil, fmt.Errorf("refine is limited to three rounds")
	}
	errors = append(errors, je...)
	blocking, missing := 0, 0
	if len(je) > 0 {
		blocking, missing = 1, 1
	} else {
		for _, f := range items(j["findings"]) {
			if f["blocking"] == true || f["needs_decision"] == true {
				blocking++
			}
		}
		for _, c := range items(j["criteria"]) {
			if c["verdict"] == "fail" {
				blocking++
			}
			if c["verdict"] == "not_assessed" {
				missing++
			}
		}
	}
	approval, e := os.ReadFile(file(artifacts, "approval.txt"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	entry := doc{"approval_text": string(approval), "commit": obj(proposal["content"])["commit"], "proposal_version": proposal["version"], "round": n, "draft": d, "judgment": judgment, "errors": errors, "cleared": len(errors) == 0 && blocking == 0 && missing == 0}
	if e = writeJSON(file(artifacts, fmt.Sprintf("rounds/%d.json", n)), entry); e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "rounds/latest.json"), entry); e != nil {
		return nil, e
	}
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	input["next_proposal_version"] = num(proposal["version"]) + 1
	if num(proposal["version"]) > 0 {
		input["pending_proposal"], input["previous_unapproved"] = proposal, true
	}
	if e = writeJSON(file(artifacts, "input.json"), input); e != nil {
		return nil, e
	}
	return doc{"round": n, "valid": len(errors) == 0, "blocking_count": blocking, "not_assessed_count": missing}, nil
}

func output(result doc, version int) doc {
	qs := []doc{}
	for _, q := range items(result["questions"]) {
		qs = append(qs, doc{"id": q["id"], "text": q["text"]})
	}
	numbers := result["issue_numbers"]
	if numbers == nil {
		numbers = []int{}
	}
	return doc{"outcome": result["outcome"], "proposal_version": version, "decision_ids": stringsOf(result["decision_ids"]), "issue_numbers": numbers, "questions": qs, "reasons": stringsOf(obj(result["stop"])["unresolved"])}
}
func finish(ctx record.ContextData, artifacts string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	entry, err := readJSON(file(artifacts, "rounds/latest.json"))
	if err != nil {
		return nil, err
	}
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	problems, p, err := proposal.CommittedErrors(ctx, entry["draft"])
	if err != nil {
		return nil, err
	}
	if entry["commit"] != obj(p["content"])["commit"] || num(entry["proposal_version"]) != num(p["version"]) {
		problems = append(problems, "proposal commit changed after the judgment")
	}
	draft, err := readDraft(artifacts)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(draft, entry["draft"]) {
		problems = append(problems, "draft changed after the judgment")
	}
	content := obj(p["content"])
	result := doc{}
	if entry["cleared"] != true || len(problems) > 0 {
		problems = append(problems, stringsOf(entry["errors"])...)
		if len(problems) == 0 {
			problems = append(problems, "The refine draft did not clear review")
		}
		result = doc{"outcome": "stop", "stop": doc{"state": "The refine draft did not clear review", "unresolved": problems, "premise": ""}}
	} else {
		text := str(entry["approval_text"])
		text += fmt.Sprintf("\n\nProposal version: %d\nCommit: %s\nParent: #%d %s", num(p["version"]), str(content["commit"]), num(obj(content["topic"])["number"]), str(obj(content["topic"])["title"]))
		for _, group := range []string{"mapping", "decisions", "bindings", "units", "topics"} {
			value := obj(entry["draft"])[group]
			if group == "mapping" {
				value = content["mapping"]
			}
			encoded, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return nil, err
			}
			text += "\n\n" + group + ":\n" + string(encoded)
		}
		content["ready"], content["approval_text"] = true, text
		if err = proposal.Save(ctx, p, content); err != nil {
			return nil, err
		}
		result = doc{"outcome": "question", "decision_ids": content["decision_ids"], "questions": []doc{{"text": text, "target": doc{"kind": "proposal", "ref": strconv.Itoa(num(p["version"]))}, "next_workflows": []string{"refine"}, "invalidates": []string{}}}}
	}
	records := []doc{}
	for n := 1; n <= num(entry["round"]); n++ {
		roundEntry, err := readJSON(file(artifacts, fmt.Sprintf("rounds/%d.json", n)))
		if err != nil {
			return nil, err
		}
		observations, findings, verdict := []string{}, []doc{}, "pass"
		for _, c := range items(obj(roundEntry["judgment"])["criteria"]) {
			observations = append(observations, str(c["aspect"])+": "+str(c["checked"]))
			if c["verdict"] == "fail" {
				verdict = "fail"
			} else if c["verdict"] == "not_assessed" && verdict != "fail" {
				verdict = "not_assessed"
			}
		}
		for i, f := range items(obj(roundEntry["judgment"])["findings"]) {
			if f["blocking"] == true || f["needs_decision"] == true {
				verdict = "fail"
			}
			findings = append(findings, doc{"id": fmt.Sprintf("refine-%d", i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": fmt.Sprintf("%s %s: %s; %s", f["aspect"], f["item"], f["failure_scenario"], f["fix"]), "evidence": f["evidence"], "mechanical": f["mechanical"] == true, "deferred": false, "resolution_key": str(f["aspect"]) + "\n" + str(f["item"]) + "\n" + str(f["failure_scenario"])})
		}
		if len(stringsOf(roundEntry["errors"])) > 0 {
			verdict = "not_assessed"
			observations = append(observations, stringsOf(roundEntry["errors"])...)
		}
		version := roundEntry["proposal_version"]
		if num(version) == 0 {
			version = max(1, num(p["version"]))
		}
		records = append(records, doc{"judge": "refine", "phase": "decide", "round": n, "kind": "proposal", "target": doc{"proposal_version": version}, "materials": []string{file(artifacts, "input.json"), file(artifacts, fmt.Sprintf("rounds/%d.json", n)), str(input["criteria"])}, "observations": observations, "verdict": verdict, "findings": findings})
	}
	if err = record.WriteLoopJudgments(ctx, records, ""); err != nil {
		return nil, err
	}
	options, err := resultOptions(ctx)
	if err != nil {
		return nil, err
	}
	if num(p["version"]) > 0 {
		result["proposal_version"] = p["version"]
	}
	result["loops"] = []doc{{"name": "decide", "ended": func() string {
		if entry["cleared"] == true {
			return "cleared"
		}
		return "max_iterations"
	}(), "rounds": num(entry["round"])}}
	saved, err := record.WriteResult(ctx.Dir, "refine", result, options)
	if err != nil {
		return nil, err
	}
	return output(saved, num(p["version"])), nil
}
func approve(ctx record.ContextData, version string, environments ...cli.Env) (doc, error) {
	problems, err := approvalErrors(ctx, version)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, &record.RecordError{Message: "proposal cannot be approved", Errors: problems}
	}
	if err = approvalStart(ctx, version); err != nil {
		return nil, err
	}
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	content := obj(p["content"])
	ids := stringsOf(content["decision_ids"])
	path := creationPath(ctx, version)
	progress, err := issues.ReadCreation(path)
	if err != nil {
		return nil, err
	}
	if progress.Commit == "" {
		data, err := decisions.List(ctx.Cwd)
		if err != nil {
			return nil, err
		}
		paths := []string{}
		for _, id := range ids {
			d := find(data.Decisions, id)
			if d == nil || d["status"] != "proposed" {
				return nil, fmt.Errorf("%s is not proposed", id)
			}
			d["status"] = "agreed"
			paths = append(paths, str(d["path"]))
			for _, oldID := range stringsOf(d["supersedes"]) {
				old := find(data.Decisions, oldID)
				if old == nil {
					return nil, fmt.Errorf("missing superseded decision %s", oldID)
				}
				old["status"] = "superseded"
				if !slices.Contains(paths, str(old["path"])) {
					paths = append(paths, str(old["path"]))
				}
			}
		}
		if errors := decisions.Check(data); len(errors) > 0 {
			return nil, &record.RecordError{Message: "approval failed the decision check", Errors: errors}
		}
		for _, d := range data.Decisions {
			if slices.Contains(paths, str(d["path"])) {
				if err = put(filepath.Join(ctx.Cwd, str(d["path"])), decisionText(d)); err != nil {
					return nil, err
				}
			}
		}
		if len(paths) > 0 {
			if err = proposal.CommitPaths(ctx, paths, "docs: agree refine proposal "+version); err != nil {
				return nil, err
			}
		}
		progress.RunID = ctx.RunID
		progress.Commit, err = gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		if err = issues.SaveCreation(path, progress); err != nil {
			return nil, err
		}
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	ds, err := issues.ReadDrafts(content["draft"])
	if err != nil {
		return nil, err
	}
	env := cli.Env{Cwd: ctx.Cwd}
	if len(environments) > 0 {
		env = environments[0]
		env.Cwd = ctx.Cwd
	}
	topic := obj(content["topic"])
	parent := &issues.TopicDraft{Title: str(topic["title"]), Body: str(topic["body"])}
	comment := strings.Join(ids, "\n")
	progress, err = (issues.Client{Env: env}).CreateDrafts(path, ds, c.Issues.Labels, parent, num(topic["number"]), comment)
	if err != nil {
		return nil, err
	}
	options, err := resultOptions(ctx)
	if err != nil {
		return nil, err
	}
	v, _ := strconv.Atoi(version)
	saved, err := record.WriteResult(ctx.Dir, "refine", doc{"outcome": "pass", "proposal_version": v, "decision_ids": ids, "issue_numbers": progress.Numbers()}, options)
	if err != nil {
		return nil, err
	}
	return output(saved, v), nil
}
func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"start", "prepare", "check", "round", "finish", "approve", "not-started"} {
		commands = append(commands, cli.Command{Area: "refine", Name: mode, Usage: "devkit refine " + mode, Run: func(args []string, env cli.Env) error {
			ctx, err := record.Context(env)
			if err != nil {
				return err
			}
			display := ctx.Dir
			ctx.Dir = env.Path(ctx.Dir)
			artifacts := env.Getenv("ARTIFACTS_DIR")
			var out any
			switch mode {
			case "start":
				var started record.StartOutput
				started, err = start(ctx, env)
				started.Details = display
				out = started
			case "approve":
				out, err = approve(ctx, env.Getenv("INPUTS_APPROVE"), env)
			default:
				if artifacts == "" {
					return fmt.Errorf("ARTIFACTS_DIR must be set")
				}
				switch mode {
				case "prepare":
					out, err = prepare(ctx, env, artifacts, env.Getenv("INPUTS_OWNER_WORDS"), env.Getenv("INPUTS_SUPPLEMENT"), env.Getenv("INPUTS_MEMO"), env.Getenv("INPUTS_TOPIC"))
				case "check":
					out, err = check(ctx, artifacts)
				case "round":
					var judgment any
					judgment = doc{}
					if text := env.Getenv("INPUTS_JUDGMENT"); text != "" {
						err = json.Unmarshal([]byte(text), &judgment)
					}
					if err == nil {
						out, err = round(ctx, artifacts, judgment)
					}
				case "finish":
					out, err = finish(ctx, artifacts)
				case "not-started":
					var stopped doc
					err = json.Unmarshal([]byte(env.Getenv("INPUTS_START")), &stopped)
					out = output(doc{"outcome": "not_started", "stop": doc{"unresolved": append([]any{stopped["reason"]}, array(stopped["remaining"])...)}}, 0)
				}
			}
			if err != nil {
				return err
			}
			return env.JSON(out)
		}})
	}
	commands = append(commands, cli.Command{Area: "refine", Usage: "devkit refine start | prepare | check | round | finish | approve | not-started", Run: func(args []string, env cli.Env) error {
		if _, err := record.Context(env); err != nil {
			return err
		}
		if env.Getenv("ARTIFACTS_DIR") == "" {
			return fmt.Errorf("ARTIFACTS_DIR must be set")
		}
		return &cli.ExitError{Code: 1, Message: "Usage: devkit refine start | prepare | check | round | finish | approve | not-started"}

	}})
	return commands
}
