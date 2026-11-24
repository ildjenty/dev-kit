package plan

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type document = record.Document

func object(v any) document { d, _ := v.(map[string]any); return d }
func entries(v any) []document {
	out := []document{}
	switch a := v.(type) {
	case []document:
		return a
	case []any:
		for _, v := range a {
			out = append(out, object(v))
		}
	}
	return out
}
func text(v any) string { s, _ := v.(string); return s }
func number(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}
func stringsOf(v any) []string {
	if a, ok := v.([]string); ok {
		return a
	}
	out := []string{}
	a, _ := v.([]any)
	for _, v := range a {
		out = append(out, text(v))
	}
	return out
}
func strip(d document, keys ...string) document {
	if d == nil {
		return nil
	}
	out := document{}
	for k, v := range d {
		out[k] = v
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}
func read(file string) (document, error) {
	b, e := os.ReadFile(file)
	if e != nil {
		return nil, e
	}
	var d document
	e = json.Unmarshal(b, &d)
	return d, e
}
func readOr(file string, fallback document) (document, error) {
	d, e := read(file)
	if os.IsNotExist(e) {
		return fallback, nil
	}
	return d, e
}
func write(file string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(file), 0755); e != nil {
		return e
	}
	return os.WriteFile(file, append(b, '\n'), 0644)
}
func rounds(artifacts string) string { return filepath.Join(artifacts, "plan", "rounds") }
func filter(rows []document, accept func(document) bool) []document {
	out := []document{}
	for _, r := range rows {
		if accept(r) {
			out = append(out, r)
		}
	}
	return out
}
func qualitiesOf(d document) []string {
	out := []string{}
	for _, r := range entries(d["decisions"]) {
		q := text(r["quality"])
		if !slices.Contains(out, q) {
			out = append(out, q)
		}
	}
	return out
}

func previousPlan(dir string) (document, error) {
	requirements, err := record.Latest(dir, "requirements")
	if err != nil {
		return nil, err
	}
	p, err := record.Latest(dir, "plan")
	if err != nil {
		return nil, err
	}
	if p == nil || p["requirements_version"] != requirements["version"] {
		return nil, nil
	}
	judgments, err := record.List(dir, "judgment")
	if err != nil {
		return nil, err
	}
	findings := []document{}
	for _, j := range judgments {
		if j["kind"] != "plan" || object(j["target"])["plan_version"] != p["version"] {
			continue
		}
		for _, f := range entries(j["findings"]) {
			if f["resolved"] == true {
				continue
			}
			row := document{"judge": j["judge"]}
			for k, v := range f {
				row[k] = v
			}
			findings = append(findings, row)
		}
	}
	results, err := record.List(dir, "result")
	if err != nil {
		return nil, err
	}
	carried := []document{}
	for _, r := range results {
		if r["run_id"] == p["run_id"] {
			carried = entries(r["carried"])
		}
	}
	return document{"plan": strip(p, "version", "requirements_version", "run_id", "recorded_at"), "findings": findings, "carried": carried}, nil
}
func prepare(ctx record.ContextData, artifacts, display string) (document, error) {
	if display == "" {
		display = artifacts
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return nil, err
	}
	if requirements == nil {
		return nil, fmt.Errorf("no requirements are recorded yet")
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("no request is recorded yet")
	}
	source, err := criteria.CopyCriteria(criteria.Options{Root: ctx.Cwd, Base: text(requests[0]["criteria_commit"]), Dir: filepath.Join(artifacts, "criteria")})
	if err != nil {
		return nil, err
	}
	qs, err := record.ListKeyed(ctx.Dir, "question")
	if err != nil {
		return nil, err
	}
	questions := map[string]document{}
	for _, q := range qs {
		questions[text(q["id"])] = q
	}
	as, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return nil, err
	}
	answers := []document{}
	for _, a := range as {
		row := document{"question_id": a["question_id"], "question": a["question"], "target": a["target"], "versions": a["versions"], "answer": a["answer"]}
		if q := questions[text(a["question_id"])]; q != nil {
			row["asked_by"] = q["workflow"]
		}
		answers = append(answers, row)
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	previous, err := previousPlan(ctx.Dir)
	if err != nil {
		return nil, err
	}
	rs := []document{}
	for i, r := range requests {
		if r["owner_words"] != "" || r["supplement"] != "" {
			rs = append(rs, document{"number": i + 1, "owner_words": r["owner_words"], "supplement": r["supplement"]})
		}
	}
	mapping, err := concerns.Load(filepath.Join(artifacts, "criteria", source.Paths[0]))
	if err != nil {
		return nil, err
	}
	configured := mapping.Judged("plan")
	selectedMaterials := map[string]concerns.Material{}
	for _, q := range configured {
		selectedMaterials[q] = mapping.Materials(filepath.Join(display, "criteria", source.Paths[0]), q, "plan")
	}
	pending, err := record.PendingOwnerRows(ctx.Dir)
	if err != nil {
		return nil, err
	}
	returned, err := record.Returned(ctx, "plan")
	if err != nil {
		return nil, err
	}
	input := document{"returned": returned, "pending_owner_rows": pending, "language": c.Language, "standards": source.Paths[0], "criteria_folder": filepath.Join(display, "criteria", source.Paths[0]), "qualities": configured, "quality_materials": selectedMaterials, "writer_materials": mapping.Materials(filepath.Join(display, "criteria", source.Paths[0]), "writer:plan", "plan"), "skeleton_check": c.SkeletonCheck, "requests": rs, "requirements": strip(requirements, "request", "run_id", "recorded_at"), "answers": answers, "previous": previous, "criteria": filepath.Join(display, "criteria")}
	if err = record.WritePlanDraftSchema(filepath.Join(artifacts, "plan")); err != nil {
		return nil, err
	}
	file := filepath.Join(artifacts, "plan", "input.json")
	if err = write(file, input); err != nil {
		return nil, err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return nil, err
	}
	base, err := g.MergeBase("")
	if err != nil {
		return nil, err
	}
	return document{"input": filepath.Join(display, "plan", "input.json"), "criteria": filepath.Join(display, "criteria"), "base": base}, nil
}

func draftErrors(d, requirements document, commit string, phases ...string) ([]string, error) {
	if d == nil {
		return []string{"The plan draft $ARTIFACTS_DIR/plan/draft.json is missing"}, nil
	}
	if commit == "" {
		commit = strings.Repeat("0", 40)
	}
	v := strip(d)
	v["version"] = 1
	v["requirements_version"] = 1
	v["run_id"] = "-"
	v["recorded_at"] = "-"
	v["skeleton_commit"] = commit
	errors, err := record.Validate("plan", v)
	if err != nil || len(errors) > 0 {
		return errors, err
	}
	if len(phases) > 0 && phases[0] == "structure" {
		for _, row := range entries(d["decisions"]) {
			if row["quality"] != "structure" {
				errors = append(errors, "Structure phase accepts only structure rows")
			}
		}
		for _, key := range []string{"increments", "tests", "kept_behaviors"} {
			if len(entries(d[key])) > 0 {
				errors = append(errors, "Structure phase leaves "+key+" empty")
			}
		}
		return errors, nil
	}
	ids := []string{}
	for _, c := range entries(requirements["completion_conditions"]) {
		if !slices.Contains(ids, text(c["id"])) {
			ids = append(ids, text(c["id"]))
		}
	}
	tested, units := []string{}, []string{}
	for _, t := range entries(d["tests"]) {
		tested = append(tested, text(t["criterion"]))
	}
	for _, u := range entries(d["increments"]) {
		units = append(units, stringsOf(u["criteria"])...)
	}
	for _, id := range ids {
		if !slices.Contains(tested, id) {
			errors = append(errors, fmt.Sprintf("Completion condition %s has no test-plan entry", id))
		}
		if !slices.Contains(units, id) {
			errors = append(errors, fmt.Sprintf("Completion condition %s has no release unit", id))
		}
	}
	seen := map[string]bool{}
	for _, id := range append(tested, units...) {
		if !seen[id] && !slices.Contains(ids, id) {
			errors = append(errors, id+" is not a completion condition in the requirements")
		}
		seen[id] = true
	}
	return errors, nil
}
func materials(quality string, input, draft, skeleton, facts, changes document) document {
	material := object(object(input["quality_materials"])[quality])
	out := document{"quality": quality, "language": input["language"], "standards": input["standards"], "criteria_folder": input["criteria_folder"], "criteria": material["criteria"], "principles": material["principles"], "every_quality": material["every_quality"], "common": material["common"], "authority": material["authority"], "requirements": input["requirements"], "owner_answers": input["answers"], "skeleton": skeleton}
	if quality == "delivery" {
		for _, k := range []string{"decisions", "increments", "tests", "kept_behaviors"} {
			out[k] = draft[k]
		}
		out["facts"] = document{"before": facts["before"]}
		out["changes"] = changes["items"]
		out["previous_carried"] = entries(object(input["previous"])["carried"])
	} else {
		out["decisions"] = filter(entries(draft["decisions"]), func(d document) bool { return d["quality"] == quality })
		out["changes"] = filter(entries(changes["items"]), func(d document) bool { return d["quality"] == quality })
		if quality == "structure" {
			out["facts"] = document{"before": facts["before"], "after": facts["after"], "locality": facts["locality"]}
		}
		out["previous_findings"] = filter(entries(object(input["previous"])["findings"]), func(d document) bool { return d["judge"] == quality })
	}
	return out
}

func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"prepare", "facts", "check", "settle", "finish", "advance", "not-started"} {
		commands = append(commands, cli.Command{Area: "plan", Name: mode, Usage: "devkit plan " + mode, Run: func(args []string, env cli.Env) error {
			if mode == "not-started" {
				if len(args) < 1 {
					return cli.Usage("devkit plan not-started <start-json>")
				}
				var s document
				if err := json.Unmarshal([]byte(args[0]), &s); err != nil {
					return err
				}
				return env.JSON(document{"outcome": "not_started", "plan_version": 0, "skeleton_commit": "", "questions": []document{}, "carried": []string{}, "reasons": append([]string{text(s["reason"])}, stringsOf(s["remaining"])...)})
			}
			if env.Getenv("ARTIFACTS_DIR") == "" {
				return fmt.Errorf("ARTIFACTS_DIR must be set")
			}
			artifacts := env.Path(env.Getenv("ARTIFACTS_DIR"))
			var out document
			var err error
			if mode == "settle" {
				outputs := map[string]string{}
				for _, j := range concerns.Names() {
					outputs[j] = env.Getenv("INPUTS_" + strings.ToUpper(j))
				}
				var uncertain []string
				if value := env.Getenv("INPUTS_UNCERTAIN"); strings.TrimSpace(value) != "" {
					if err := json.Unmarshal([]byte(value), &uncertain); err != nil {
						return err
					}
				}
				out, err = settle(artifacts, outputs, env.Getenv("INPUTS_PROTOTYPE"), env.Getenv("INPUTS_STRUCTURE_QUESTION"), uncertain)
			} else {
				ctx, e := record.Context(env)
				if e != nil {
					return e
				}
				ctx.Dir = env.Path(ctx.Dir)
				switch mode {
				case "prepare":
					out, err = prepare(ctx, artifacts, env.Getenv("ARTIFACTS_DIR"))
				case "facts":
					out, err = factsCommand(ctx, artifacts, env.Getenv("ARTIFACTS_DIR"))
				case "check":
					out, err = checkCommand(ctx, artifacts, env.Environ, env.Getenv("ARTIFACTS_DIR"), env.Getenv("PLAN_PHASE"))
				case "advance":
					out, err = advance(ctx, artifacts)
				case "finish":
					g, e := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
					if e != nil {
						return e
					}
					base, e := g.MergeBase("")
					if e != nil {
						return e
					}
					commit, e := g.Head()
					if e != nil {
						return e
					}
					out, err = finish(ctx, artifacts, record.ResultOptions{Base: base, Commit: commit})
				}
			}
			if err != nil {
				return err
			}
			return env.JSON(out)
		}})
	}
	return commands
}
