package refine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
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
func docsSnapshot(cwd, folder string) (map[string]string, error) {
	paths, e := decisions.MarkdownFiles(filepath.Join(cwd, folder))
	if e != nil {
		return nil, e
	}
	r := map[string]string{}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(cwd, p)
		if e != nil {
			return nil, e
		}
		r[filepath.ToSlash(rel)] = string(b)
	}
	return r, nil
}
func changedDocs(cwd, folder string) (doc, error) {
	paths, e := changes(cwd)
	if e != nil {
		return nil, e
	}
	r := doc{}
	for _, p := range paths {
		if !inside(p, folder) {
			continue
		}
		b, e := os.ReadFile(filepath.Join(cwd, p))
		if os.IsNotExist(e) {
			r[p] = nil
		} else if e != nil {
			return nil, e
		} else {
			r[p] = string(b)
		}
	}
	return r, nil
}

var draftKeys = regexp.MustCompile(`\b(?:N[1-9]\d*|UN[1-9]\d*|TN[1-9]\d*)\b`)

func replaceKeys(text string, mapping doc) string {
	return draftKeys.ReplaceAllStringFunc(text, func(k string) string {
		if s, ok := mapping[k].(string); ok {
			return s
		}
		return k
	})
}
func question(text, kind, ref string) doc {
	return doc{"text": text, "target": doc{"kind": kind, "ref": ref}, "next_workflows": []string{"refine"}, "invalidates": []string{}}
}
func resultOptions(ctx record.ContextData) (record.ResultOptions, error) {
	base, e := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ctx.BaseBranch)
	if e != nil {
		return record.ResultOptions{}, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	return record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}, e
}
func approvalErrors(ctx record.ContextData, version string) ([]string, error) {
	p, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	r, e := record.Latest(ctx.Dir, "result")
	if e != nil {
		return nil, e
	}
	if !regexp.MustCompile(`^[1-9]\d*$`).MatchString(version) || strconv.Itoa(num(p["version"])) != version {
		return []string{"approve must name the latest proposal version"}, nil
	}
	qs := items(r["questions"])
	if obj(p["content"])["materialized"] != true || r["run_id"] != p["run_id"] || r["outcome"] != "question" || len(qs) != 1 || obj(qs[0]["target"])["kind"] != "proposal" || obj(qs[0]["target"])["ref"] != version {
		return []string{"the latest proposal result did not ask for approval"}, nil
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if r["commit"] != head {
		return []string{"HEAD changed after the proposal asked for approval"}, nil
	}
	return []string{}, nil
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
	if _, e := selectedTopic(ctx, env.Getenv("INPUTS_TOPIC")); e != nil {
		return stopped(e.Error()), nil
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
		answers = append(answers, record.Answer{QuestionID: str(items(r["questions"])[0]["id"]), Answer: "approve=" + approve})
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
		if obj(proposals[i]["content"])["materialized"] == true {
			pending = proposals[i]
			break
		}
	}

	selected := ""
	if len(topicID) > 0 {
		selected = topicID[0]
	}
	topic, e := selectedTopic(ctx, selected)
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
	_, e = criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: str(requests[0]["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
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
	mapping, e := concerns.Load(filepath.Join(artifacts, "criteria", c.Standards))
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
		id, err := decisions.NextID("topic", ctx.Branch, "request", data)
		if err != nil {
			return nil, err
		}
		topic = doc{"id": id, "title": text, "body": text, "status": "open", "depends_on": []string{}}
	}
	if pending != nil && obj(obj(pending["content"])["topic"])["id"] != topic["id"] {
		pending = nil
	}
	affected := decisions.Affected(data)
	affected.Decisions = decisionIndex(affected.Decisions)
	input := doc{"pending_proposal": pending, "previous_unapproved": pending != nil, "topic": topic, "topics": data.Topics, "requests": rs, "memo": memo, "answers": as, "previous": previous, "decisions": decisionIndex(data.Decisions), "units": data.Units, "documents": data.Documents, "affected": affected, "language": c.Language, "criteria": filepath.Join(displayArtifacts, "criteria", c.Standards), "folders": doc{"docs": c.Docs, "decisions": c.Decisions, "units": c.Units, "topics": c.Topics, "standards": c.Standards}, "next_proposal_version": num(previous["version"]) + 1, "judge_materials": mapping.Materials(filepath.Join(displayArtifacts, "criteria", c.Standards), "judge:refine", "plan"), "writer_materials": mapping.Materials(filepath.Join(displayArtifacts, "criteria", c.Standards), "writer:refine", "plan")}
	relevant, e := relevance(env, ctx.Cwd, artifacts, input, data, c)
	if e != nil {
		return nil, e
	}
	input["relevant_decisions"] = relevant
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
	docs, e := docsSnapshot(ctx.Cwd, c.Docs)
	if e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, "baseline.json"), doc{"commit": head, "docs": docs, "decisions_hash": decisionHash(data.Decisions)}); e != nil {
		return nil, e
	}
	return doc{"input": file(displayArtifacts, "input.json"), "criteria": input["criteria"]}, nil
}
func draftErrors(ctx record.ContextData, artifacts string, value any, phases ...string) ([]string, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	errors, e := schema("draft", value)
	if e != nil || len(errors) > 0 {
		return errors, e
	}
	phase := "document"
	if len(phases) > 0 {
		phase = phases[0]
	}
	draft := obj(value)
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	baseline, e := readJSON(file(artifacts, "baseline.json"))
	if e != nil {
		return nil, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if head != baseline["commit"] {
		errors = append(errors, "writer changed HEAD; only finish may commit")
	}
	data, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	existing := data.Decisions
	newIDs := []string{}
	known := []string{}
	for _, d := range existing {
		known = append(known, str(d["id"]))
	}
	for _, d := range items(draft["decisions"]) {
		newIDs = append(newIDs, str(d["key"]))
		known = append(known, str(d["key"]))
	}
	for _, pair := range [][2]string{{"topics", "key"}, {"decisions", "key"}, {"units", "key"}, {"open_points", "id"}, {"kept_docs", "path"}, {"kept_units", "id"}} {
		seen := map[string]bool{}
		duplicate := false
		for _, d := range items(draft[pair[0]]) {
			k := str(d[pair[1]])
			duplicate = duplicate || seen[k]
			seen[k] = true
		}
		if duplicate {
			errors = append(errors, "duplicate "+pair[1])
		}
	}
	superseded := []string{}
	for _, d := range items(draft["decisions"]) {
		for _, id := range stringsOf(d["supersedes"]) {
			if !slices.Contains(superseded, id) {
				superseded = append(superseded, id)
			}
		}
		for _, id := range stringsOf(d["depends_on"]) {
			if !slices.Contains(known, id) {
				errors = append(errors, fmt.Sprintf("%s: missing dependency %s", d["key"], id))
			}
		}
		for _, id := range stringsOf(d["supersedes"]) {
			if find(existing, id) == nil {
				errors = append(errors, fmt.Sprintf("%s: missing superseded decision %s", d["key"], id))
			}
		}
		if d["decided_by"] == "owner" && d["source"] == "derived" {
			errors = append(errors, fmt.Sprintf("%s: a derived choice cannot be attributed to the owner", d["key"]))
		}
	}
	for _, id := range superseded {
		n := 0
		for _, d := range items(draft["decisions"]) {
			if has(d["supersedes"], id) {
				n++
			}
		}
		if n > 1 {
			errors = append(errors, id+": more than one successor")
		}
	}
	graph := []doc{}
	prospective := data
	prospective.Decisions = []doc{}
	for _, d := range existing {
		g := clone(d)
		g["depends_on"] = stringsOf(d["depends_on"])
		graph = append(graph, g)
		p := clone(d)
		if slices.Contains(superseded, str(d["id"])) {
			p["status"] = "superseded"
		}
		prospective.Decisions = append(prospective.Decisions, p)
	}
	impact := decisions.Affected(prospective)
	targets := map[string]bool{}
	for _, r := range items(draft["rebased"]) {
		d := find(graph, str(r["id"]))
		successor := find(existing, str(r["to"]))
		for _, n := range items(draft["decisions"]) {
			if n["key"] == r["to"] {
				successor = n
				break
			}
		}
		key := str(r["id"]) + ":" + str(r["from"])
		if targets[key] {
			errors = append(errors, key+": duplicate rebase")
		}
		targets[key] = true
		if find(impact.Decisions, str(r["id"])) == nil || !has(d["depends_on"], str(r["from"])) || !has(successor["supersedes"], str(r["from"])) {
			errors = append(errors, str(r["id"])+": invalid dependency rebase")
		} else {
			deps := stringsOf(d["depends_on"])
			for i, id := range deps {
				if id == r["from"] {
					deps[i] = str(r["to"])
				}
			}
			d["depends_on"] = deps
		}
	}
	obsolete := []string{}
	for _, d := range prospective.Decisions {
		if d["status"] == "superseded" {
			obsolete = append(obsolete, str(d["id"]))
		}
	}
	for _, d := range impact.Decisions {
		after := find(graph, str(d["id"]))
		bad := false
		for _, id := range stringsOf(after["depends_on"]) {
			bad = bad || slices.Contains(obsolete, id)
		}
		if !slices.Contains(superseded, str(d["id"])) && bad {
			errors = append(errors, str(d["id"])+": affected decision is neither superseded nor consistently rebased")
		}
	}
	fullGraph := append([]doc{}, graph...)
	for _, d := range items(draft["decisions"]) {
		d = clone(d)
		d["id"] = d["key"]
		fullGraph = append(fullGraph, d)
	}
	errors = append(errors, decisions.GraphErrors(fullGraph)...)
	unitIDs := []string{}
	for _, u := range items(input["units"]) {
		unitIDs = append(unitIDs, str(u["id"]))
	}
	for _, u := range items(draft["units"]) {
		unitIDs = append(unitIDs, str(u["key"]))
	}
	retainedUnits := map[string]doc{}
	previous := obj(obj(input["pending_proposal"])["content"])
	for _, u := range items(draft["units"]) {
		key := str(u["key"])
		id := key
		if input["previous_unapproved"] == true && str(obj(previous["mapping"])[key]) != "" {
			id = str(obj(previous["mapping"])[key])
		}
		retainedUnits[id] = u
		if strings.HasPrefix(key, "U-") {
			existing := find(items(input["units"]), key)
			if existing == nil {
				errors = append(errors, key+": retained unit does not exist")
			} else if existing["state"] == "done" {
				errors = append(errors, "done unit may not be edited or removed: "+key)
			}
		}
		for _, id := range stringsOf(u["decisions"]) {
			if slices.Contains(superseded, id) {
				errors = append(errors, key+": open unit cites superseded decision "+id)
			}
			if !slices.Contains(known, id) {
				errors = append(errors, str(u["key"])+": missing decision "+id)
			}
		}
		for _, id := range stringsOf(u["depends_on"]) {
			if !slices.Contains(unitIDs, id) {
				errors = append(errors, str(u["key"])+": missing unit "+id)
			}
		}
	}
	unitGraph := items(input["units"])
	for _, u := range items(draft["units"]) {
		u = clone(u)
		u["id"] = u["key"]
		unitGraph = append(unitGraph, u)
	}
	errors = append(errors, decisions.GraphErrors(unitGraph)...)
	for _, pair := range []struct {
		kind          string
		before, after any
	}{{"decision", baseline["decisions_hash"], decisionHash(data.Decisions)}, {"unit", input["units"], data.Units}} {
		b, _ := json.Marshal(pair.before)
		a, _ := json.Marshal(pair.after)
		if string(b) != string(a) {
			errors = append(errors, "writer edited "+pair.kind+" records directly")
		}
	}
	edited, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	folders := obj(input["folders"])
	for _, u := range items(input["units"]) {
		if u["state"] == "done" && slices.Contains(edited, str(u["path"])) {
			errors = append(errors, "done unit may not be edited or removed: "+str(u["id"]))
		}
	}
	for _, p := range edited {
		if !inside(p, str(folders["docs"])) {
			errors = append(errors, "edited outside the docs folder: "+p)
			continue
		}
		for _, reserved := range []string{"standards", "decisions", "units", "topics"} {
			if inside(p, str(folders[reserved])) {
				errors = append(errors, "edited a "+reserved+" file, which refine's writer may not change: "+p)
			}
		}
	}
	if phase == "document" {
		for _, u := range items(input["units"]) {
			id := str(u["id"])
			retained := retainedUnits[id]
			if u["state"] == "done" || retained == nil && input["previous_unapproved"] == true && has(previous["unit_ids"], id) {
				continue
			}
			for _, old := range stringsOf(u["decisions"]) {
				if !slices.Contains(superseded, old) {
					continue
				}
				for _, successor := range items(draft["decisions"]) {
					if has(successor["supersedes"], old) && !has(retained["decisions"], str(successor["key"])) {
						errors = append(errors, id+": open unit must be retained citing the successor of superseded decision "+old)
					}
				}
			}
		}
		for _, u := range impact.Units {
			handled := false
			for _, k := range items(draft["kept_units"]) {
				handled = handled || k["id"] == u["id"]
			}
			if !handled {
				errors = append(errors, str(u["id"])+": affected unit was not handled with a reason")
			}
		}
		for _, d := range impact.Documents {
			kept := false
			for _, k := range items(draft["kept_docs"]) {
				kept = kept || k["path"] == d.Path
			}
			if !slices.Contains(edited, d.Path) && !kept {
				errors = append(errors, d.Path+": affected document was not edited or kept with a reason")
			}
		}
	}
	for _, k := range items(draft["kept_units"]) {
		if find(data.Units, str(k["id"])) == nil {
			errors = append(errors, str(k["id"])+": kept unit does not exist")
		}
	}
	for _, k := range items(draft["kept_docs"]) {
		found := false
		for _, d := range data.Documents {
			found = found || d.Path == k["path"]
		}
		if !found {
			errors = append(errors, str(k["path"])+": kept document does not exist")
		}
	}
	inspect := func(text, at string) {
		for _, id := range decisions.Citations(text) {
			if !slices.Contains(known, id) {
				errors = append(errors, at+": missing citation "+id)
			}
		}
		for _, key := range regexp.MustCompile(`\bN[1-9]\d*\b`).FindAllString(text, -1) {
			if !slices.Contains(newIDs, key) {
				errors = append(errors, at+": missing draft key "+key)
			}
		}
	}
	docs, e := docsSnapshot(ctx.Cwd, str(folders["docs"]))
	if e != nil {
		return nil, e
	}
	paths := []string{}
	for p := range docs {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		inspect(docs[p], p)
	}
	for _, d := range items(draft["decisions"]) {
		inspect(str(d["body"]), str(d["key"]))
	}
	for _, topic := range items(draft["topics"]) {
		inspect(str(topic["body"]), str(topic["key"]))
	}
	for _, u := range items(draft["units"]) {
		inspect(str(u["goal"])+"\n"+strings.Join(stringsOf(u["completion_conditions"]), "\n"), str(u["key"]))
	}
	if phase == "document" && len(array(draft["open_points"])) == 0 {
		b, e := os.ReadFile(file(artifacts, "approval.txt"))
		if os.IsNotExist(e) || e == nil && strings.TrimSpace(string(b)) == "" {
			errors = append(errors, "approval.txt is required in the configured language")
		} else if e != nil {
			return nil, e
		} else {
			text := string(b)
			for _, d := range append(append(items(draft["decisions"]), items(draft["units"])...), items(draft["topics"])...) {
				key := str(d["key"])
				if !regexp.MustCompile(`\b` + regexp.QuoteMeta(key) + `\b`).MatchString(text) {
					errors = append(errors, "approval.txt omits "+key)
				}
			}
			if !strings.Contains(text, "approve="+strconv.Itoa(num(input["next_proposal_version"]))) || !strings.Contains(text, "answers") {
				errors = append(errors, "approval.txt must explain approve=<version> and answers")
			}
		}
	}
	errors = append(errors, topicErrors(input, draft, data.Decisions)...)
	errors = append(errors, phaseErrors(ctx, artifacts, phase, draft, input)...)
	return errors, nil
}
func check(ctx record.ContextData, artifacts string, phases ...string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	d, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, d, phases...)
	if e != nil {
		return nil, e
	}
	if len(errors) == 0 && (len(phases) == 0 || phases[0] == "document") {
		reflection, err := reflectionInput(ctx.Cwd, artifacts)
		if err != nil {
			return nil, err
		}
		if err = writeJSON(file(artifacts, "reflect-input.json"), reflection); err != nil {
			return nil, err
		}
	}
	out := doc{"valid": len(errors) == 0, "errors": errors}
	return out, writeJSON(file(artifacts, "check.json"), out)
}
func round(ctx record.ContextData, artifacts string, judgment any, phases ...string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	d, e := readDraft(artifacts)
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, d, phases...)
	if e != nil {
		return nil, e
	}
	je, e := schema("judgment", judgment)
	if e != nil {
		return nil, e
	}
	j := obj(judgment)
	if len(je) == 0 {
		phase := "document"
		if len(phases) > 0 {
			phase = phases[0]
		}
		for _, finding := range items(j["findings"]) {
			if !slices.Contains(phaseAspects(phase), str(finding["aspect"])) && !(phase == "document" && finding["aspect"] == "reflection") {
				je = append(je, "finding is outside the current phase")
			}
		}
		if len(items(j["criteria"])) != len(phaseAspects(phase)) {
			je = append(je, "criteria must contain only the current phase aspects")
		}
		for _, aspect := range phaseAspects(phase) {
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
	phase := "document"
	if len(phases) > 0 {
		phase = phases[0]
	}
	previous, e := readJSON(file(artifacts, phase+"/rounds/latest.json"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	n := num(previous["round"]) + 1
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
	entry := doc{"round": n, "draft": d, "judgment": judgment, "errors": errors, "cleared": len(errors) == 0 && blocking == 0 && missing == 0}
	if e = writeJSON(file(artifacts, fmt.Sprintf(phase+"/rounds/%d.json", n)), entry); e != nil {
		return nil, e
	}
	if e = writeJSON(file(artifacts, phase+"/rounds/latest.json"), entry); e != nil {
		return nil, e
	}
	return doc{"round": n, "valid": len(errors) == 0, "blocking_count": blocking, "not_assessed_count": missing}, nil
}
func recordText(front doc, keys []string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range keys {
		v, _ := json.Marshal(front[k])
		fmt.Fprintf(&b, "%s: %s\n", k, v)
	}
	b.WriteString("---\n" + strings.TrimSpace(body) + "\n")
	return b.String()
}
func decisionText(d doc) string {
	return recordText(d, []string{"id", "title", "status", "decided_by", "depends_on", "supersedes", "request"}, str(d["body"]))
}
func restoreDocs(ctx record.ContextData, artifacts, docs string) error {
	baseline, e := readJSON(file(artifacts, "baseline.json"))
	if e != nil {
		return e
	}
	paths, e := changes(ctx.Cwd)
	if e != nil {
		return e
	}
	for _, p := range paths {
		if !inside(p, docs) {
			continue
		}
		tracked, e := gitx.Run(ctx.Cwd, "ls-tree", str(baseline["commit"]), "--", p)
		if e != nil {
			return e
		}
		if tracked != "" {
			_, e = gitx.Run(ctx.Cwd, "restore", "--source", str(baseline["commit"]), "--staged", "--worktree", "--", p)
		} else {
			e = os.Remove(filepath.Join(ctx.Cwd, p))
			if os.IsNotExist(e) {
				e = nil
			}
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func rewrite(v any, mapping doc) any {
	switch x := v.(type) {
	case string:
		return replaceKeys(x, mapping)
	case []any:
		r := []any{}
		for _, v := range x {
			r = append(r, rewrite(v, mapping))
		}
		return r
	case map[string]any:
		r := doc{}
		for k, v := range x {
			r[k] = rewrite(v, mapping)
		}
		return r
	}
	return v
}
func materialize(ctx record.ContextData, artifacts string, draft, input doc) (doc, error) {
	data, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	prior := obj(input["pending_proposal"])
	previous := obj(prior["content"])
	reusable := input["previous_unapproved"] == true
	mapping := doc{}
	for _, kind := range []string{"decision", "unit", "topic"} {
		for _, item := range items(draft[kind+"s"]) {
			key := str(item["key"])
			if kind == "unit" && strings.HasPrefix(key, "U-") {
				existing := find(data.Units, key)
				if existing == nil || existing["state"] == "done" {
					return nil, fmt.Errorf("%s is not an open unit", key)
				}
				mapping[key] = key
				continue
			}
			old := str(obj(previous["mapping"])[key])
			var existing doc
			switch kind {
			case "decision":
				existing = find(data.Decisions, old)
				if existing["status"] != "proposed" {
					existing = nil
				}
			case "unit":
				existing = find(data.Units, old)
				if existing["state"] == "done" {
					existing = nil
				}
			case "topic":
				existing = find(data.Topics, old)
				if existing["status"] != "open" {
					existing = nil
				}
			}
			if reusable && existing != nil && has(previous[kind+"_ids"], old) {
				mapping[key] = old
			} else {
				id, err := decisions.NextID(kind, ctx.Branch, key, data)
				if err != nil {
					return nil, err
				}
				mapping[key] = id
				switch kind {
				case "decision":
					data.Decisions = append(data.Decisions, doc{"id": id})
				case "unit":
					data.Units = append(data.Units, doc{"id": id})
				case "topic":
					data.Topics = append(data.Topics, doc{"id": id})
				}
			}
		}
	}

	resolved := obj(rewrite(draft, mapping))
	folders := obj(input["folders"])
	paths := []string{}
	add := func(p string) {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	if reusable {
		for _, kind := range []string{"decision", "unit", "topic"} {
			for key, value := range obj(previous["mapping"]) {
				id := str(value)
				if !has(previous[kind+"_ids"], id) || mapping[key] == value {
					continue
				}
				group := data.Decisions
				if kind == "unit" {
					group = data.Units
				}
				if kind == "topic" {
					group = data.Topics
				}
				item := find(group, id)
				if item == nil {
					continue
				}
				if kind == "unit" && item["state"] == "done" {
					continue
				}
				if kind == "decision" && item["status"] != "proposed" || kind == "topic" && item["status"] != "open" {
					return nil, fmt.Errorf("cannot withdraw settled record %s", id)
				}
				p := str(item["path"])
				if e = os.Remove(filepath.Join(ctx.Cwd, p)); e != nil {
					return nil, e
				}
				add(p)
			}
		}
	}
	edited, e := changes(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	for _, p := range edited {
		if inside(p, str(folders["docs"])) {
			add(p)
		}
	}
	docs, e := docsSnapshot(ctx.Cwd, str(folders["docs"]))
	if e != nil {
		return nil, e
	}
	sorted := []string{}
	for p := range docs {
		sorted = append(sorted, p)
	}
	slices.Sort(sorted)
	for _, p := range sorted {
		if slices.ContainsFunc(data.Units, func(u doc) bool { return u["state"] == "done" && u["path"] == p }) {
			continue
		}
		text := replaceKeys(docs[p], mapping)
		if text != docs[p] {
			if e = put(filepath.Join(ctx.Cwd, p), text); e != nil {
				return nil, e
			}
			add(p)
		}
	}
	for _, d := range items(resolved["decisions"]) {
		id := str(d["key"])
		if v, ok := mapping[id]; ok {
			id = str(v)
		}
		p := str(folders["decisions"]) + "/" + id + ".md"
		d = clone(d)
		d["id"], d["status"], d["request"] = id, "proposed", ctx.Branch
		if e = put(filepath.Join(ctx.Cwd, p), decisionText(d)); e != nil {
			return nil, e
		}
		add(p)
	}
	for _, u := range items(resolved["units"]) {
		id := str(u["key"])
		if v, ok := mapping[id]; ok {
			id = str(v)
		}
		p := str(folders["units"]) + "/" + id + ".md"
		if existing := find(data.Units, id); str(existing["path"]) != "" {
			p = str(existing["path"])
		}
		body := str(u["goal"]) + "\n\n"
		for _, c := range stringsOf(u["completion_conditions"]) {
			body += "- " + c + "\n"
		}
		u = clone(u)
		u["id"] = id
		if e = put(filepath.Join(ctx.Cwd, p), recordText(u, []string{"id", "title", "decisions", "depends_on"}, body)); e != nil {
			return nil, e
		}
		add(p)
	}
	for _, r := range items(resolved["rebased"]) {
		d := find(data.Decisions, str(r["id"]))
		deps := stringsOf(d["depends_on"])
		for i, id := range deps {
			if id == r["from"] {
				deps[i] = str(r["to"])
			}
		}
		d["depends_on"] = deps
		p := str(d["path"])
		if e = put(filepath.Join(ctx.Cwd, p), decisionText(d)); e != nil {
			return nil, e
		}
		add(p)
	}
	topicIDs := []string{}
	for _, topic := range items(resolved["topics"]) {
		topic = clone(topic)
		topic["id"] = topic["key"]
		topic["status"] = "open"
		p := str(folders["topics"]) + "/" + str(topic["id"]) + ".md"
		if e = put(filepath.Join(ctx.Cwd, p), topicText(topic)); e != nil {
			return nil, e
		}
		add(p)
		topicIDs = append(topicIDs, str(topic["id"]))
	}
	topic := obj(input["topic"])
	p := str(folders["topics"]) + "/" + str(topic["id"]) + ".md"
	if e = put(filepath.Join(ctx.Cwd, p), topicText(topic)); e != nil {
		return nil, e
	}
	add(p)
	if memo := str(input["memo"]); memo != "" {
		slug := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(ctx.Branch, "-")
		p := filepath.ToSlash(filepath.Join(filepath.Dir(str(folders["decisions"])), "discussions", time.Now().UTC().Format("2006-01-02")+"-"+slug+".md"))
		if e = put(filepath.Join(ctx.Cwd, p), replaceKeys(memo, mapping)+"\n"); e != nil {
			return nil, e
		}
		add(p)
	}
	materialized, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	if errors := decisions.Check(materialized); len(errors) > 0 {
		return nil, &record.RecordError{Message: "materialized proposal failed the decision check", Errors: errors}
	}
	if len(paths) > 0 {
		if _, e = gitx.Run(ctx.Cwd, append([]string{"add", "--"}, paths...)...); e != nil {
			return nil, e
		}
		staged, e := gitx.Run(ctx.Cwd, "diff", "--cached", "--name-only")
		if e != nil {
			return nil, e
		}
		if staged != "" {
			if _, e = commitPaths(ctx.Cwd, paths, "docs: propose decisions and work units"); e != nil {
				return nil, e
			}
		}
	}
	ds, us := []string{}, []string{}
	for _, d := range items(draft["decisions"]) {
		ds = append(ds, str(mapping[str(d["key"])]))
	}
	for _, u := range items(draft["units"]) {
		us = append(us, str(mapping[str(u["key"])]))
	}
	return doc{"mapping": mapping, "decision_ids": ds, "unit_ids": us, "topic_ids": topicIDs, "topic": topic}, nil
}
func output(result doc, version int, extra doc) doc {
	qs := []doc{}
	for _, q := range items(result["questions"]) {
		qs = append(qs, doc{"id": q["id"], "text": q["text"]})
	}
	out := doc{"outcome": result["outcome"], "proposal_version": version, "questions": qs, "reasons": stringsOf(obj(result["stop"])["unresolved"]), "decision_ids": []string{}, "unit_ids": []string{}, "affected": decisions.Data{Decisions: []doc{}, Documents: []decisions.Document{}, Units: []doc{}}}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
func finish(ctx record.ContextData, artifacts string) (doc, error) {
	displayArtifacts := artifacts
	artifacts = resolve(ctx.Cwd, artifacts)
	phase := "document"
	entry, e := readJSON(file(artifacts, phase+"/rounds/latest.json"))
	if os.IsNotExist(e) {
		phase = "decide"
		entry, e = readJSON(file(artifacts, phase+"/rounds/latest.json"))
	}
	if e != nil {
		return nil, e
	}
	input, e := readJSON(file(artifacts, "input.json"))
	if e != nil {
		return nil, e
	}
	errors, e := draftErrors(ctx, artifacts, entry["draft"], phase)
	if e != nil {
		return nil, e
	}
	errors = append(stringsOf(entry["errors"]), errors...)
	draft, j := obj(entry["draft"]), obj(entry["judgment"])
	folders := obj(input["folders"])
	docs, e := changedDocs(ctx.Cwd, str(folders["docs"]))
	if e != nil {
		return nil, e
	}
	content := doc{"draft": entry["draft"], "judgment": entry["judgment"], "rounds": entry["round"], "memo": input["memo"], "documents": docs, "materialized": false, "topic": input["topic"], "phase": phase}
	var resultDoc doc
	if entry["cleared"] != true || len(errors) > 0 {
		unresolved := append([]string{}, errors...)
		for _, f := range items(j["findings"]) {
			if f["blocking"] == true || f["needs_decision"] == true {
				unresolved = append(unresolved, str(f["failure_scenario"]))
			}
		}
		for _, c := range items(j["criteria"]) {
			if c["verdict"] != "pass" {
				text := str(c["not_checked"])
				if text == "" {
					text = str(c["checked"])
				}
				unresolved = append(unresolved, str(c["aspect"])+": "+text)
			}
		}
		if len(unresolved) == 0 {
			unresolved = append(unresolved, "The last round did not pass")
		}
		resultDoc = doc{"outcome": "stop", "stop": doc{"state": "The refine draft did not clear the review", "unresolved": unresolved, "premise": ""}}
		if e = restoreDocs(ctx, artifacts, str(folders["docs"])); e != nil {
			return nil, e
		}
	} else if phase == "decide" && len(array(draft["open_points"])) == 0 {
		return nil, fmt.Errorf("document phase has not completed")
	} else if len(array(draft["open_points"])) > 0 {
		qs := []doc{}
		for _, p := range items(draft["open_points"]) {
			qs = append(qs, question(str(p["text"]), "open_point", str(p["id"])))
		}
		resultDoc = doc{"outcome": "question", "questions": qs}
		if e = restoreDocs(ctx, artifacts, str(folders["docs"])); e != nil {
			return nil, e
		}
	} else {
		m, e := materialize(ctx, artifacts, draft, input)
		if e != nil {
			return nil, e
		}
		for k, v := range m {
			content[k] = v
		}
		content["materialized"] = true
		b, e := os.ReadFile(file(artifacts, "approval.txt"))
		if e != nil {
			return nil, e
		}
		content["approval_text"] = replaceKeys(strings.TrimSpace(string(b)), obj(content["mapping"]))
		resultDoc = doc{"outcome": "question", "questions": []doc{question(str(content["approval_text"]), "proposal", strconv.Itoa(num(input["next_proposal_version"])))}}
	}
	if phase == "document" {
		frozen, err := readJSON(file(artifacts, "decided.json"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		content["decide"] = frozen
	}
	proposal, e := record.Write(ctx, "proposal", doc{"content": content}, "")
	if e != nil {
		return nil, e
	}
	phaseEntries := map[string]doc{phase: entry}
	if phase == "document" {
		frozen, err := readJSON(file(artifacts, "decided.json"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if frozen != nil {
			phaseEntries["decide"] = frozen
		}
	}
	loops := []doc{}
	records := []doc{}
	for _, phaseName := range []string{"decide", "document"} {
		phaseEntry := phaseEntries[phaseName]
		if phaseEntry == nil {
			continue
		}
		ended := "cleared"
		if phaseEntry["cleared"] != true {
			ended = "stuck"
			if num(phaseEntry["round"]) >= 3 {
				ended = "max_iterations"
			}
		}
		loops = append(loops, doc{"name": phaseName, "ended": ended, "rounds": phaseEntry["round"]})
		for n := 1; n <= num(phaseEntry["round"]); n++ {
			roundEntry, e := readJSON(file(artifacts, fmt.Sprintf("%s/rounds/%d.json", phaseName, n)))
			if e != nil {
				return nil, e
			}
			de, e := schema("draft", roundEntry["draft"])
			if e != nil {
				return nil, e
			}
			je, e := schema("judgment", roundEntry["judgment"])
			if e != nil {
				return nil, e
			}
			if len(de) == 0 && len(je) == 0 {
				j := obj(roundEntry["judgment"])
				observations := []string{}
				verdict := "pass"
				for _, c := range items(j["criteria"]) {
					observations = append(observations, str(c["aspect"])+": "+str(c["checked"]))
					if c["verdict"] == "not_assessed" && verdict != "fail" {
						verdict = "not_assessed"
					}
					if c["verdict"] == "fail" {
						verdict = "fail"
					}
				}
				findings := []doc{}
				for i, f := range items(j["findings"]) {
					if f["blocking"] == true || f["needs_decision"] == true {
						verdict = "fail"
					}
					findings = append(findings, doc{"id": fmt.Sprintf("refine-%d", i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": fmt.Sprintf("%s %s: %s; %s", f["aspect"], f["item"], f["failure_scenario"], f["fix"]), "evidence": f["evidence"], "mechanical": f["mechanical"] == true, "deferred": f["deferred"] == true, "resolution_key": str(f["aspect"]) + "\n" + str(f["item"]) + "\n" + str(f["failure_scenario"])})
				}
				if errors := stringsOf(roundEntry["errors"]); len(errors) > 0 {
					verdict = "not_assessed"
					observations = append(observations, errors...)
				}
				records = append(records, doc{"judge": "refine", "phase": phaseName, "round": n, "kind": "proposal", "target": doc{"proposal_version": proposal.Entry["version"]}, "materials": []string{file(displayArtifacts, "input.json"), file(displayArtifacts, fmt.Sprintf("%s/rounds/%d.json", phaseName, n)), str(input["criteria"])}, "observations": observations, "verdict": verdict, "findings": findings})
			} else {
				records = append(records, doc{"judge": "refine", "phase": phaseName, "round": n, "kind": "proposal", "target": doc{"proposal_version": proposal.Entry["version"]}, "materials": []string{file(displayArtifacts, fmt.Sprintf("%s/rounds/%d.json", phaseName, n))}, "observations": append(de, je...), "verdict": "not_assessed", "findings": []doc{}})
			}
		}
	}
	if e = record.WriteLoopJudgments(ctx, records, ""); e != nil {
		return nil, e
	}

	resultDoc["loops"] = loops

	ds, us := stringsOf(content["decision_ids"]), stringsOf(content["unit_ids"])
	resultDoc["proposal_version"], resultDoc["decision_ids"], resultDoc["unit_ids"] = proposal.Entry["version"], ds, us
	o, e := resultOptions(ctx)
	if e != nil {
		return nil, e
	}
	result, e := record.WriteResult(ctx.Dir, "refine", resultDoc, o)
	if e != nil {
		return nil, e
	}
	return output(result, num(proposal.Entry["version"]), doc{"decision_ids": ds, "unit_ids": us}), nil
}
func approve(ctx record.ContextData, version string) (doc, error) {
	errors, e := approvalErrors(ctx, version)
	if e != nil {
		return nil, e
	}
	if len(errors) > 0 {
		return nil, &record.RecordError{Message: "proposal cannot be approved", Errors: errors}
	}
	if e = approvalStart(ctx, version); e != nil {
		return nil, e
	}
	status, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return nil, e
	}
	if status != "" {
		return nil, fmt.Errorf("approval requires a clean worktree")
	}
	proposal, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	data, e := decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	content := obj(proposal["content"])
	ids := stringsOf(content["decision_ids"])
	paths := []string{}
	add := func(p string) {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	for _, id := range ids {
		d := find(data.Decisions, id)
		if d == nil || d["status"] != "proposed" {
			return nil, fmt.Errorf("%s is not proposed", id)
		}
		d["status"] = "agreed"
		for _, oldID := range stringsOf(d["supersedes"]) {
			old := find(data.Decisions, oldID)
			if old == nil {
				return nil, fmt.Errorf("%s does not exist", oldID)
			}
			old["status"] = "superseded"
			add(str(old["path"]))
		}
		add(str(d["path"]))
	}
	topic := obj(content["topic"])
	topic = find(data.Topics, str(topic["id"]))
	if topic == nil || topic["status"] != "open" {
		return nil, fmt.Errorf("proposal topic is not open")
	}
	topic["status"] = "settled"
	add(str(topic["path"]))
	if invalid := decisions.Check(data); len(invalid) > 0 {
		return nil, &record.RecordError{Message: "approval failed the decision check", Errors: invalid}
	}
	for _, p := range paths {
		if topic["path"] == p {
			if e = put(filepath.Join(ctx.Cwd, p), topicText(topic)); e != nil {
				return nil, e
			}
			continue
		}
		for _, d := range data.Decisions {
			if d["path"] == p {
				if e = put(filepath.Join(ctx.Cwd, p), decisionText(d)); e != nil {
					return nil, e
				}
				break
			}
		}
	}
	if len(paths) > 0 {
		if _, e = gitx.Run(ctx.Cwd, append([]string{"add", "--"}, paths...)...); e != nil {
			return nil, e
		}
		if _, e = commitPaths(ctx.Cwd, paths, "docs: agree refine proposal "+version); e != nil {
			return nil, e
		}
	}
	data, e = decisions.List(ctx.Cwd)
	if e != nil {
		return nil, e
	}
	remaining := decisions.Affected(data)
	o, e := resultOptions(ctx)
	if e != nil {
		return nil, e
	}
	v, _ := strconv.Atoi(version)
	extra := doc{"decision_ids": ids, "unit_ids": content["unit_ids"], "affected": remaining}
	result, e := record.WriteResult(ctx.Dir, "refine", doc{"outcome": "pass", "proposal_version": v, "decision_ids": ids, "unit_ids": content["unit_ids"], "affected": remaining}, o)
	if e != nil {
		return nil, e
	}
	return output(result, v, extra), nil
}
func Commands() []cli.Command {
	commands := []cli.Command{{Area: "refine", Name: "reflect", Usage: "devkit refine reflect", Run: runReflect}}
	for _, mode := range []string{"start", "prepare", "check", "round", "finish", "decided", "approve", "not-started"} {
		commands = append(commands, cli.Command{Area: "refine", Name: mode, Usage: "devkit refine " + mode, Run: func(args []string, env cli.Env) error {
			ctx, e := record.Context(env)
			if e != nil {
				return e
			}
			displayDir := ctx.Dir
			ctx.Dir = env.Path(ctx.Dir)
			artifacts := env.Getenv("ARTIFACTS_DIR")
			var out any
			if mode == "start" {
				var started record.StartOutput
				started, e = start(ctx, env)
				started.Details = displayDir
				out = started
			} else if mode == "approve" {
				out, e = approve(ctx, env.Getenv("INPUTS_APPROVE"))
			} else {
				if artifacts == "" {
					return fmt.Errorf("ARTIFACTS_DIR must be set")
				}
				switch mode {
				case "prepare":
					out, e = prepare(ctx, env, artifacts, env.Getenv("INPUTS_OWNER_WORDS"), env.Getenv("INPUTS_SUPPLEMENT"), env.Getenv("INPUTS_MEMO"), env.Getenv("INPUTS_TOPIC"))
				case "check":
					out, e = check(ctx, artifacts, env.Getenv("INPUTS_PHASE"))
				case "round":
					var j any
					e = json.Unmarshal([]byte(env.Getenv("INPUTS_JUDGMENT")), &j)
					if e == nil && env.Getenv("INPUTS_PHASE") == "document" {
						checked, err := readJSON(file(resolve(ctx.Cwd, artifacts), "check.json"))
						if err != nil {
							return err
						}
						if checked["valid"] == true {
							j, e = roundReflection(env, obj(j))
						}
					}
					if e == nil {
						out, e = round(ctx, artifacts, j, env.Getenv("INPUTS_PHASE"))
					}
				case "decided":
					out, e = decided(ctx, artifacts)
				case "finish":
					out, e = finish(ctx, artifacts)
				case "not-started":
					var s doc
					e = json.Unmarshal([]byte(env.Getenv("INPUTS_START")), &s)
					if e == nil {
						reasons := append([]any{s["reason"]}, array(s["remaining"])...)
						out = output(doc{"outcome": "not_started"}, 0, doc{"reasons": reasons})
					}
				}
			}
			if e != nil {
				return e
			}
			return env.JSON(out)
		}})
	}
	commands = append(commands, cli.Command{Area: "refine", Usage: "devkit refine start | prepare | check | reflect | round | decided | finish | approve | not-started", Run: func(args []string, env cli.Env) error {
		if _, err := record.Context(env); err != nil {
			return err
		}
		if env.Getenv("ARTIFACTS_DIR") == "" {
			return fmt.Errorf("ARTIFACTS_DIR must be set")
		}
		return &cli.ExitError{Code: 1, Message: "Usage: devkit refine start | prepare | check | reflect | round | decided | finish | approve | not-started"}
	}})
	return commands
}
