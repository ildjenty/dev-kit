package judge

import (
	"encoding/json"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type doc = record.Document

func object(v any) doc  { d, _ := v.(map[string]any); return d }
func text(v any) string { s, _ := v.(string); return s }
func truth(v any) bool  { b, _ := v.(bool); return b }
func docs(v any) []doc {
	if d, ok := v.([]doc); ok {
		return d
	}
	out := []doc{}
	if a, ok := v.([]any); ok {
		for _, v := range a {
			out = append(out, object(v))
		}
	}
	return out
}
func stringsOf(v any) []string {
	if a, ok := v.([]string); ok {
		return a
	}
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, v := range a {
			out = append(out, text(v))
		}
	}
	return out
}
func clone(d doc) doc {
	out := doc{}
	for k, v := range d {
		out[k] = v
	}
	return out
}
func read(file string) (doc, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var d doc
	err = json.Unmarshal(b, &d)
	return d, err
}
func optional(file string) (doc, error) {
	d, err := read(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return d, err
}
func save(file string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}
func qualities() []string { return concerns.Names() }
func context(env cli.Env) (record.ContextData, error) {
	ctx, err := record.Context(env)
	ctx.Dir = env.Path(ctx.Dir)
	return ctx, err
}
func envDoc(env cli.Env, key string) (doc, error) {
	s := env.Getenv(key)
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var d doc
	err := json.Unmarshal([]byte(s), &d)
	return d, err
}
func Commands() []cli.Command {
	return []cli.Command{
		{Area: "judge", Name: "prepare", Usage: "devkit judge prepare", Run: func(args []string, env cli.Env) error {
			ctx, err := context(env)
			if err != nil {
				return err
			}
			result, err := prepare(ctx, env.Path(env.Getenv("ARTIFACTS_DIR")), env.Getenv("ARTIFACTS_DIR"))
			if err != nil {
				return err
			}
			return env.JSON(result)
		}},
		{Area: "judge", Name: "changes", Usage: "devkit judge changes", Run: func(args []string, env cli.Env) error {
			result, err := writeChanges(env.Cwd, env.Path(env.Getenv("ARTIFACTS_DIR")))
			if err != nil {
				return err
			}
			return env.JSON(result)
		}},
		{Area: "judge", Name: "route", Usage: "devkit judge route", Run: runRoute},
		{Area: "judge", Name: "launch", Usage: "devkit judge launch", Run: func(args []string, env cli.Env) error {
			routed, err := envDoc(env, "ROUTE")
			if err != nil {
				return err
			}
			if routed == nil {
				return fmt.Errorf("route did not finish")
			}
			result, err := routeChanges(env.Cwd, env.Path(env.Getenv("ARTIFACTS_DIR")), routed)
			if err != nil {
				return err
			}
			return env.JSON(result)
		}},
		{Area: "judge", Name: "completion", Usage: "devkit judge completion", Run: runCompletion},
		{Area: "judge", Name: "collect", Usage: "devkit judge collect", Run: collect},
		{Area: "judge", Name: "finish", Usage: "devkit judge finish", Run: finish},
	}
}
func completion(plan doc, commit string, g *gitx.Git) (doc, error) {
	tests := []doc{}
	missing := []doc{}
	for _, t := range docs(plan["tests"]) {
		if t["method"] == "test" {
			tests = append(tests, t)
		}
	}
	for _, t := range tests {
		ref := commit + ":" + text(t["file"])
		status := "missing_file"
		if g.OK("cat-file", "-e", ref) {
			kind, err := g.Run("cat-file", "-t", ref)
			if err != nil {
				return nil, err
			}
			if kind == "blob" {
				source, err := g.Run("show", ref)
				if err != nil {
					return nil, err
				}
				if strings.Contains(source, text(t["name"])) {
					continue
				}
				status = "missing_name"
			}
		}
		item := clone(t)
		item["status"] = status
		missing = append(missing, item)
	}
	return doc{"proceed": len(missing) == 0, "checked_count": len(tests), "missing_count": len(missing), "missing": missing}, nil
}
func runCompletion(args []string, env cli.Env) error {
	dir := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "judge")
	target, err := read(env.Path(filepath.Join(dir, "target.json")))
	if err != nil {
		return err
	}
	plan, err := optional(env.Path(filepath.Join(dir, "plan.json")))
	if err != nil {
		return err
	}
	g, err := gitx.New(env.Cwd, env.Getenv("BASE_BRANCH"))
	if err != nil {
		return err
	}
	result, err := completion(plan, text(target["commit"]), g)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "completion.json")
	if err = save(env.Path(path), result); err != nil {
		return err
	}
	delete(result, "missing")
	result["path"] = path
	return env.JSON(result)
}
func gather(outputs, launch doc, materials ...doc) ([]doc, error) {
	judges := []doc{}
	for _, q := range qualities() {
		if !truth(launch["launch_"+q]) {
			continue
		}
		output := object(outputs[q])
		if output == nil {
			return nil, fmt.Errorf("%s did not finish; no complete judgment can be recorded", q)
		}
		criteria := docs(output["criteria"])
		if len(criteria) == 0 {
			criteria = []doc{{"aspect": q, "verdict": "not_assessed", "checked": "", "not_checked": "The launched judge returned empty criteria"}}
		}
		if len(materials) > 0 {
			for _, path := range stringsOf(object(materials[0][q])["criteria"]) {
				aspect := strings.TrimSuffix(filepath.Base(path), ".md")
				found := false
				for _, c := range criteria {
					if c["aspect"] == aspect {
						found = true
					}
				}
				if !found {
					criteria = append(criteria, doc{"aspect": aspect, "verdict": "not_assessed", "checked": "", "not_checked": "Missing required criterion: " + path})
				}
			}
		}
		output["criteria"] = criteria
		findings := []doc{}
		for i, f := range docs(output["findings"]) {
			f = clone(f)
			if truth(f["deferred"]) && (q != "structure" || truth(f["blocking"]) || truth(f["needs_decision"])) {
				output["criteria"] = []doc{{"aspect": q, "verdict": "not_assessed", "checked": "", "not_checked": "Invalid deferred finding: only nonblocking structure findings without an owner decision may be deferred"}}
				findings = []doc{}
				break
			}
			f["id"] = fmt.Sprintf("%s-%d", q, i+1)
			findings = append(findings, f)
		}
		judges = append(judges, doc{"judge": q, "criteria": output["criteria"], "findings": findings})
	}
	return judges, nil
}
func collect(args []string, env cli.Env) error {
	launch, err := envDoc(env, "CHANGES")
	if err != nil {
		return err
	}
	outputs := doc{}
	for _, q := range qualities() {
		d, err := envDoc(env, "JUDGE_"+strings.ToUpper(q))
		if err != nil {
			return err
		}
		outputs[q] = d
	}
	materials := doc{}
	for _, q := range qualities() {
		if !truth(launch["launch_"+q]) {
			continue
		}
		material, err := read(env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "judge", "materials", q+".json")))
		if err != nil {
			return err
		}
		materials[q] = material
	}
	judges, err := gather(outputs, launch, materials)
	if err != nil {
		return err
	}
	if err = save(env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "judge", "findings.json")), doc{"judges": judges}); err != nil {
		return err
	}
	count, blocking := 0, 0
	for _, j := range judges {
		for _, f := range docs(j["findings"]) {
			count++
			if truth(f["blocking"]) {
				blocking++
			}
		}
	}
	return env.JSON(doc{"judges_count": len(judges), "findings_count": count, "blocking_count": blocking})
}
