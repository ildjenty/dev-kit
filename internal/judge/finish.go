package judge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
)

func verdictOf(criteria, findings []doc) string {
	if len(criteria) == 0 {
		return "not_assessed"
	}
	for _, c := range criteria {
		if c["verdict"] == "fail" {
			return "fail"
		}
	}
	for _, f := range findings {
		if truth(f["blocking"]) && !truth(f["deferred"]) {
			return "fail"
		}
	}
	for _, c := range criteria {
		if c["verdict"] == "not_assessed" {
			return "not_assessed"
		}
	}
	return "pass"
}
func joined(d doc, keys []string, sep string) string {
	parts := []string{}
	for _, k := range keys {
		if v := text(d[k]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}
func toRecordFinding(f doc) doc {
	return doc{"id": f["id"], "blocking": truth(f["blocking"]) && !truth(f["deferred"]), "needs_decision": truth(f["needs_decision"]) && !truth(f["deferred"]), "deferred": truth(f["deferred"]), "mechanical": truth(f["mechanical"]), "text": joined(f, []string{"failure_scenario", "fix"}, " "), "evidence": joined(f, []string{"file", "line", "evidence"}, ":")}
}
func missingStep(inputs doc) string {
	p := object(inputs["prepare"])
	if p == nil {
		return "prepare"
	}
	if truth(p["stopped"]) {
		return ""
	}
	c := object(inputs["changes"])
	if c == nil {
		return "changes"
	}
	if !truth(c["proceed"]) {
		return ""
	}
	for _, key := range []string{"gate", "completion", "collect"} {
		if object(inputs[key]) == nil {
			if key == "gate" {
				return "checks"
			}
			return key
		}
	}
	return ""
}
func stop(state string, unresolved []string) doc {
	return doc{"outcome": "stop", "stop": doc{"state": state, "unresolved": unresolved, "premise": ""}}
}
func listed(changes []doc) []string {
	out := []string{}
	for _, c := range changes {
		out = append(out, fmt.Sprintf("%s %s (%s)", text(c["status"]), text(c["path"]), text(c["quality"])))
	}
	return out
}
func decide(inputs doc) ([]doc, doc) {
	p, c, gate, completion, findings := object(inputs["prepare"]), object(inputs["changes"]), object(inputs["gate"]), object(inputs["completion"]), object(inputs["findings"])
	if truth(p["stopped"]) {
		return []doc{}, stop(text(p["reason"]), stringsOf(p["details"]))
	}
	if len(docs(c["unlisted"])) > 0 && p["has_requirements"] == false {
		return []doc{}, stop("this change needs requirements and a plan: start `requirements` describing it", listed(docs(c["unlisted"])))
	}
	if len(docs(c["unlisted"])) > 0 {
		return []doc{}, doc{"outcome": "back", "back": doc{"to": "plan", "premise": "Every decision change must be listed in the plan", "observation": strings.Join(listed(docs(c["unlisted"])), " / ")}}
	}
	machine := []doc{}
	for _, name := range stringsOf(gate["failing"]) {
		message := "Check " + name + " did not pass at the judged commit"
		if slices.Contains(stringsOf(gate["preexisting"]), name) {
			message += "; it fails on the base branch too"
		}
		machine = append(machine, doc{"id": "check-" + name, "blocking": true, "needs_decision": false, "text": message, "evidence": gate["results"]})
	}
	for i, m := range docs(completion["missing"]) {
		machine = append(machine, doc{"id": fmt.Sprintf("completion-%d", i+1), "blocking": true, "needs_decision": false, "text": fmt.Sprintf("%s: %s does not contain the planned test %s (%s)", text(m["criterion"]), text(m["file"]), text(m["name"]), text(m["status"])), "evidence": "judge/completion.json"})
	}
	if !truth(gate["passed"]) && len(machine) == 0 {
		machine = append(machine, doc{"id": "checks", "blocking": true, "needs_decision": false, "text": "The checks did not pass", "evidence": gate["results"]})
	}
	skipped := []doc{}
	for _, name := range stringsOf(gate["skipped"]) {
		skipped = append(skipped, doc{"id": "skipped-" + name, "blocking": false, "needs_decision": false, "text": "Check " + name + " was not run", "evidence": gate["results"]})
	}
	judgments := []doc{}
	var test doc
	needsPlan := false
	for _, j := range docs(findings["judges"]) {
		observations := []string{}
		for _, c := range docs(j["criteria"]) {
			s := fmt.Sprintf("%s %s: %s", text(c["aspect"]), text(c["verdict"]), text(c["checked"]))
			if text(c["not_checked"]) != "" {
				s += " / " + text(c["not_checked"])
			}
			observations = append(observations, s)
		}
		ff := []doc{}
		for _, f := range docs(j["findings"]) {
			if f["aspect"] == "without_plan" && truth(f["blocking"]) {
				needsPlan = true
			}
			ff = append(ff, toRecordFinding(f))
		}
		judgment := doc{"judge": j["judge"], "materials": []string{"judge/materials/" + text(j["judge"]) + ".json", "judge/diff.patch", "judge/answers.json", "judge/accepted-objections.json"}, "observations": observations, "verdict": verdictOf(docs(j["criteria"]), docs(j["findings"])), "findings": ff}
		judgments = append(judgments, judgment)
		if j["judge"] == "test" {
			test = judgment
		}
	}
	observations := []string{}
	for _, check := range docs(gate["checks"]) {
		s := fmt.Sprintf("%s: %s / %s / %s", text(check["name"]), text(check["status"]), text(check["command"]), text(check["log"]))
		if text(check["base"]) != "" {
			s += " / base: " + text(check["base"]) + " / " + text(check["base_log"])
		}
		if text(check["reason"]) != "" {
			s += " / " + text(check["reason"])
		}
		observations = append(observations, s)
	}
	if test == nil {
		test = doc{"judge": "test", "materials": []string{"judge/completion.json", text(gate["results"])}, "observations": observations, "verdict": "pass", "findings": []doc{}}
		judgments = append(judgments, test)
	} else {
		test["materials"] = append(stringsOf(test["materials"]), "judge/completion.json", text(gate["results"]))
		test["observations"] = append(stringsOf(test["observations"]), observations...)
	}
	test["findings"] = append(append(docs(test["findings"]), machine...), skipped...)
	if len(machine) > 0 {
		test["verdict"] = "fail"
	}
	pending := []doc{}
	blocking := []string{}
	incomplete := []string{}
	for _, j := range judgments {
		for _, f := range docs(j["findings"]) {
			if truth(f["needs_decision"]) {
				pending = append(pending, doc{"id": text(j["judge"]) + ":" + text(f["id"]), "what": f["text"], "decided_by": "owner"})
			}
			if truth(f["blocking"]) && !truth(f["deferred"]) {
				blocking = append(blocking, text(f["id"])+": "+text(f["text"]))
			}
		}
		if j["verdict"] != "pass" {
			incomplete = append(incomplete, stringsOf(j["observations"])...)
		}
	}
	if names := stringsOf(gate["preexisting"]); len(names) > 0 {
		logs := []string{}
		for _, check := range docs(gate["checks"]) {
			if slices.Contains(names, text(check["name"])) {
				logs = append(logs, text(check["name"])+": "+text(check["log"]))
			}
		}
		result := stop(checks.BaseMessage(names), logs)
		result["stop"].(doc)["base_failed"] = true
		return judgments, result
	}
	if p["has_requirements"] == false && needsPlan {
		return judgments, stop("this change needs requirements and a plan: start `requirements` describing it", blocking)
	}
	if len(pending) > 0 {
		if p["has_requirements"] == false {
			return judgments, stop("this change needs requirements and a plan: start `requirements` describing it", []string{text(object(record.BackToPlan(pending)["back"])["observation"])})
		}
		return judgments, record.BackToPlan(pending)
	}
	if len(blocking) > 0 {
		return judgments, stop("Blocking findings remain; use the fix workflow", blocking)
	}
	for _, j := range judgments {
		if j["verdict"] != "pass" {
			return judgments, stop("Some criteria did not pass", incomplete)
		}
	}
	return judgments, doc{"outcome": "pass"}
}
func finish(args []string, env cli.Env) error {
	if err := env.Require("ARTIFACTS_DIR"); err != nil {
		return err
	}
	ctx, err := context(env)
	if err != nil {
		return err
	}
	dir := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "judge"))
	inputs := doc{}
	for _, key := range []string{"prepare", "changes", "gate", "completion", "collect"} {
		d, err := envDoc(env, strings.ToUpper(key))
		if err != nil {
			return err
		}
		inputs[key] = d
	}
	if missing := missingStep(inputs); missing != "" {
		return fmt.Errorf("%s did not finish; no result is written", missing)
	}
	gate := object(inputs["gate"])
	if path := text(gate["results"]); path != "" {
		data, err := os.ReadFile(env.Path(path))
		if err == nil {
			checks := []doc{}
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				var check doc
				if err = json.Unmarshal([]byte(line), &check); err != nil {
					return fmt.Errorf("%s: %w", env.Path(path), err)
				}
				checks = append(checks, check)
			}
			gate["checks"] = checks
		} else {
			return err
		}
	}
	if !truth(object(inputs["prepare"])["stopped"]) {
		target, err := read(filepath.Join(dir, "target.json"))
		if err != nil {
			return err
		}
		if _, err := expectedPlan(dir, target, truth(object(inputs["prepare"])["has_plan"])); err != nil {
			return err
		}
		changes, err := read(filepath.Join(dir, "changes.json"))
		if err != nil {
			return err
		}
		if changes == nil {
			return fmt.Errorf("changes.json is missing changes; no result is written")
		}
		proceed := truth(object(inputs["changes"])["proceed"])
		inputs["changes"] = changes
		if proceed {
			for _, key := range []string{"completion", "findings"} {
				d, err := read(filepath.Join(dir, key+".json"))
				if err != nil {
					return err
				}
				inputs[key] = d
			}
			if object(inputs["completion"])["missing"] == nil {
				return fmt.Errorf("completion.json is missing planned test results; no result is written")
			}
			launch, err := read(filepath.Join(dir, "launched.json"))
			if err != nil {
				return err
			}
			launched := stringsOf(launch["launched"])
			if len(launched) == 0 {
				return fmt.Errorf("launched.json is missing launched judges; no result is written")
			}
			for _, q := range launched {
				if !slices.ContainsFunc(docs(object(inputs["findings"])["judges"]), func(j doc) bool { return j["judge"] == q }) {
					return fmt.Errorf("%s did not finish; no result is written", q)
				}
			}
		}
	}
	judgments, outcome := decide(inputs)
	target, err := optional(filepath.Join(dir, "target.json"))
	if err != nil {
		return err
	}
	for _, j := range judgments {
		j["kind"] = "code"
		t := doc{"base": target["base"], "commit": target["commit"], "change_id": target["change_id"]}
		for _, key := range []string{"requirements_version", "plan_version"} {
			if target[key] != nil {
				t[key] = target[key]
			}
		}
		j["target"] = t
		_, err := record.Write(ctx, "judgment", j, "")
		if err != nil {
			return err
		}
	}
	if err = save(filepath.Join(dir, "outcome.json"), outcome); err != nil {
		return err
	}
	p := object(inputs["prepare"])
	result, err := record.WriteResult(ctx.Dir, "judge", outcome, record.ResultOptions{RunID: ctx.RunID, Base: text(p["base"]), Commit: text(p["commit"])})
	if err != nil {
		return err
	}
	return env.JSON(result)
}
