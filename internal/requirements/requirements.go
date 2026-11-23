package requirements

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type document = record.Document

func object(v any) document { d, _ := v.(map[string]any); return d }
func entries(v any) []document {
	result := []document{}
	switch a := v.(type) {
	case []document:
		return a
	case []any:
		for _, v := range a {
			result = append(result, object(v))
		}
	}
	return result
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
func rounds(artifacts string) string { return filepath.Join(artifacts, "requirements", "rounds") }

func prepare(ctx record.ContextData, artifacts, ownerWords, supplement, display string, unitIDs ...string) (document, error) {
	if display == "" {
		display = artifacts
	}
	requests, err := record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	unitID := ""
	if len(unitIDs) > 0 {
		unitID = unitIDs[0]
	}
	if unitID == "" {
		for _, r := range requests {
			if text(r["unit"]) != "" {
				unitID = text(r["unit"])
			}
		}
	}
	var unit document
	agreed := []document{}
	if unitID != "" {
		data, err := decisions.List(ctx.Cwd)
		if err != nil {
			return nil, err
		}
		for _, u := range data.Units {
			if u["id"] == unitID {
				unit = u
			}
		}
		if unit == nil {
			return nil, fmt.Errorf("unit %s does not exist", unitID)
		}
		agreed, err = decisions.UnitDecisions(data, unitID)
		if err != nil {
			return nil, err
		}
	}
	asked := false
	for _, r := range requests {
		if r["owner_words"] != "" {
			asked = true
		}
	}
	if strings.TrimSpace(ownerWords) == "" && strings.TrimSpace(supplement) != "" && !asked && unit == nil {
		return nil, fmt.Errorf("a supplement needs the owner's words with it when no request is recorded yet")
	}
	if strings.TrimSpace(ownerWords) != "" || strings.TrimSpace(supplement) != "" || unit != nil && !slices.ContainsFunc(requests, func(r document) bool { return r["unit"] == unitID }) {
		if _, err = record.Write(ctx, "request", document{"owner_words": ownerWords, "supplement": supplement, "unit": unitID}, ""); err != nil {
			return nil, err
		}
	}
	requests, err = record.List(ctx.Dir, "request")
	if err != nil {
		return nil, err
	}
	asked = false
	for _, r := range requests {
		if r["owner_words"] != "" {
			asked = true
		}
	}
	if !asked && unit == nil {
		return nil, fmt.Errorf("no request is recorded yet and the run got none (owner_words)")
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
		row := document{"question_id": a["question_id"], "question": a["question"], "target": a["target"], "answer": a["answer"]}
		if q := questions[text(a["question_id"])]; q != nil {
			row["asked_by"] = q["workflow"]
		}
		answers = append(answers, row)
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	previous, err := record.Latest(ctx.Dir, "requirements")
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
	selected := mapping.Materials(filepath.Join(display, "criteria", source.Paths[0]), "judge:requirements", "plan")
	paths := selected.Criteria
	returned, err := record.Returned(ctx, "requirements")
	if err != nil {
		return nil, err
	}
	input := document{"returned": returned, "unit": unit, "agreed_decisions": agreed, "language": c.Language, "standards": source.Paths[0], "criteria_folder": filepath.Join(display, "criteria", source.Paths[0]), "requests": rs, "previous": strip(previous, "version", "request", "run_id", "recorded_at"), "answers": answers, "criteria": paths, "judge_materials": selected, "writer_materials": mapping.Materials(filepath.Join(display, "criteria", source.Paths[0]), "writer:requirements", "plan")}
	if err = os.MkdirAll(rounds(artifacts), 0755); err != nil {
		return nil, err
	}
	file := filepath.Join(artifacts, "requirements", "input.json")
	if err = write(file, input); err != nil {
		return nil, err
	}
	return document{"input": filepath.Join(display, "requirements", "input.json"), "criteria": filepath.Join(display, "criteria")}, nil
}

func verdictOf(j document) string {
	cs := entries(j["criteria"])
	missing := false
	for _, aspect := range []string{"coverage", "additions", "open_points", "decidable"} {
		found := false
		for _, c := range cs {
			if c["aspect"] != aspect {
				continue
			}
			found = true
			if c["verdict"] == "fail" {
				return "fail"
			}
			if c["verdict"] != "pass" {
				missing = true
			}
		}
		if !found {
			missing = true
		}
	}

	for _, f := range entries(j["findings"]) {
		if f["blocking"] == true {
			return "fail"
		}
	}
	if missing {
		return "not_assessed"
	}
	return "pass"
}
func schemaErrors(d document) ([]string, error) {
	v := strip(d)
	if v == nil {
		v = document{}
	}
	v["version"] = 1
	v["request"] = 1
	v["run_id"] = "-"
	v["recorded_at"] = "-"
	return record.Validate("requirements", v)
}
func round(artifacts string, d, j document) (document, error) {
	previous, _ := read(filepath.Join(rounds(artifacts), "latest.json"))
	n := number(previous["round"]) + 1
	errors, err := schemaErrors(d)
	if err != nil {
		return nil, err
	}
	entry := document{"round": n, "requirements": d, "judgment": j, "schema_errors": errors, "verdict": verdictOf(j)}
	if err = write(filepath.Join(rounds(artifacts), fmt.Sprintf("%d.json", n)), entry); err != nil {
		return nil, err
	}
	if err = write(filepath.Join(rounds(artifacts), "latest.json"), entry); err != nil {
		return nil, err
	}
	blocking, missing := 0, 0
	if verdictOf(j) == "fail" {
		blocking++
	}
	if verdictOf(j) == "not_assessed" {
		missing++
	}
	for _, f := range entries(j["findings"]) {
		if f["blocking"] == true {
			blocking++
		}
	}
	for _, c := range entries(j["criteria"]) {
		if c["verdict"] == "not_assessed" {
			missing++
		}
	}
	return document{"round": n, "valid": len(errors) == 0, "blocking_count": blocking, "not_assessed_count": missing}, nil
}
func stringsOf(v any) []string {
	if a, ok := v.([]string); ok {
		return a
	}
	a, _ := v.([]any)
	out := []string{}
	for _, v := range a {
		out = append(out, text(v))
	}
	return out
}
func findingText(f document) string {
	parts := []string{fmt.Sprintf("%s %s", text(f["aspect"]), text(f["item"]))}
	for _, k := range []string{"failure_scenario", "fix"} {
		if text(f[k]) != "" {
			parts = append(parts, text(f[k]))
		}
	}
	return strings.Join(parts, "\n")
}
func outcomeOf(e document) document {
	stop := func(state string, unresolved []string) document {
		return document{"outcome": "stop", "stop": document{"state": state, "unresolved": unresolved, "premise": ""}}
	}
	if errors := stringsOf(e["schema_errors"]); len(errors) > 0 {
		return stop(fmt.Sprintf("The requirements draft still does not match the record schema after round %d", number(e["round"])), errors)
	}
	j := object(e["judgment"])
	unresolved := []string{}
	for _, f := range entries(j["findings"]) {
		if f["blocking"] == true {
			unresolved = append(unresolved, findingText(f))
		}
	}
	for _, c := range entries(j["criteria"]) {
		if c["verdict"] != "pass" {
			unresolved = append(unresolved, fmt.Sprintf("%s was not assessed: %s", text(c["aspect"]), text(c["not_checked"])))
		}
	}
	if verdictOf(j) != "pass" && len(unresolved) == 0 {
		unresolved = append(unresolved, "Required judgment aspects are missing or did not pass")
	}
	if len(unresolved) > 0 {
		return stop(fmt.Sprintf("The requirements judgment did not converge after %d rounds", number(e["round"])), unresolved)
	}
	qs := []document{}
	for _, p := range entries(object(e["requirements"])["open_points"]) {
		qs = append(qs, document{"text": p["text"], "target": document{"kind": "open_point", "ref": p["id"]}, "next_workflows": []string{"requirements"}, "invalidates": []string{}})
	}
	if len(qs) > 0 {
		return document{"outcome": "question", "questions": qs}
	}
	return document{"outcome": "pass"}
}
func finish(ctx record.ContextData, artifacts string, o record.ResultOptions) (document, error) {
	if o.Now == "" {
		o.Now = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}
	e, err := read(filepath.Join(rounds(artifacts), "latest.json"))
	if err != nil {
		return nil, err
	}
	version := 0
	j := object(e["judgment"])
	if len(stringsOf(e["schema_errors"])) == 0 {
		saved, err := record.Write(ctx, "requirements", object(e["requirements"]), o.Now)
		if err != nil {
			return nil, err
		}
		version = number(saved.Entry["version"])
		input, err := read(filepath.Join(artifacts, "requirements", "input.json"))
		if err != nil {
			return nil, err
		}
		materials := []string{"All original requests", "All owner answers", fmt.Sprintf("Requirements draft from round %d", number(e["round"]))}
		materials = append(materials, stringsOf(input["criteria"])...)
		observations := []string{}
		for _, c := range entries(j["criteria"]) {
			observations = append(observations, fmt.Sprintf("%s: %s", text(c["aspect"]), text(c["checked"])))
		}
		findings := []document{}
		for i, f := range entries(j["findings"]) {
			findings = append(findings, document{"id": fmt.Sprintf("requirements-%d", i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": findingText(f), "evidence": f["evidence"]})
		}
		_, err = record.Write(ctx, "judgment", document{"judge": "requirements", "kind": "requirements", "target": document{"requirements_version": version}, "materials": materials, "observations": observations, "verdict": e["verdict"], "findings": findings}, o.Now)
		if err != nil {
			return nil, err
		}
	}
	o.RunID = ctx.RunID
	result, err := record.WriteResult(ctx.Dir, "requirements", outcomeOf(e), o)
	if err != nil {
		return nil, err
	}
	questions := []document{}
	for _, q := range entries(result["questions"]) {
		questions = append(questions, document{"id": q["id"], "text": q["text"]})
	}
	return document{"outcome": result["outcome"], "requirements_version": version, "questions": questions, "reasons": stringsOf(object(result["stop"])["unresolved"])}, nil
}

func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"prepare", "round", "finish", "not-started"} {
		commands = append(commands, cli.Command{Area: "requirements", Name: mode, Usage: "devkit requirements " + mode, Run: func(args []string, env cli.Env) error {
			if mode == "not-started" {
				if len(args) < 1 {
					return cli.Usage("devkit requirements not-started <start-json>")
				}
				var s document
				if err := json.Unmarshal([]byte(args[0]), &s); err != nil {
					return err
				}
				return env.JSON(document{"outcome": "not_started", "requirements_version": 0, "questions": []document{}, "reasons": append([]string{text(s["reason"])}, stringsOf(s["remaining"])...)})
			}
			if env.Getenv("ARTIFACTS_DIR") == "" {
				return fmt.Errorf("ARTIFACTS_DIR must be set")
			}
			artifacts := env.Path(env.Getenv("ARTIFACTS_DIR"))
			var out document
			var err error
			if mode == "round" {
				var d, j document
				if err = json.Unmarshal([]byte(env.Getenv("INPUTS_REQUIREMENTS")), &d); err != nil {
					return err
				}
				if err = json.Unmarshal([]byte(env.Getenv("INPUTS_JUDGMENT")), &j); err != nil {
					return err
				}
				out, err = round(artifacts, d, j)
			} else {
				ctx, e := record.Context(env)
				if e != nil {
					return e
				}
				ctx.Dir = env.Path(ctx.Dir)
				if mode == "prepare" {
					out, err = prepare(ctx, artifacts, env.Getenv("INPUTS_OWNER_WORDS"), env.Getenv("INPUTS_SUPPLEMENT"), env.Getenv("ARTIFACTS_DIR"), env.Getenv("INPUTS_UNIT"))
				} else {
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
