package plan

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/record"
)

func settle(artifacts string, outputs map[string]string, prototype, structureQuestion string) (document, error) {
	state, err := read(filepath.Join(rounds(artifacts), "latest.json"))
	if err != nil {
		return nil, err
	}
	judgments := document{}
	findings := []document{}
	missing := []document{}
	fixable := []document{}
	for _, e := range stringsOf(state["errors"]) {
		fixable = append(fixable, document{"judge": "machine", "row": "", "blocking": true, "needs_decision": false, "failure_scenario": e, "evidence": "", "fix": ""})
	}
	needs := []document{}
	for _, judge := range stringsOf(state["judges"]) {
		var j document
		if json.Unmarshal([]byte(outputs[judge]), &j) != nil || j == nil {
			j = document{"verdict": "not_assessed", "checked": "", "not_checked": "The judge has no output (failed or returned invalid JSON)", "findings": []document{}}
		}
		if j["verdict"] != "pass" && j["verdict"] != "fail" && j["verdict"] != "not_assessed" {
			j["verdict"] = "not_assessed"
			j["not_checked"] = "The judge returned no valid verdict"
		}
		if j["verdict"] == "fail" && len(entries(j["findings"])) == 0 {
			j["verdict"] = "not_assessed"
			j["not_checked"] = "The judge returned fail without findings"
		}
		judgments[judge] = j
		invalid := false
		for _, f := range entries(j["findings"]) {
			if f["deferred"] == true && (judge != "structure" || f["blocking"] == true || f["needs_decision"] == true) {
				j["verdict"] = "not_assessed"
				j["not_checked"] = "Invalid deferred finding: only nonblocking structure findings without an owner decision may be deferred"
				invalid = true
			}
		}
		if invalid {
			j["findings"] = []document{}
		}
		for i, f := range entries(j["findings"]) {
			row := document{"judge": judge, "id": fmt.Sprintf("%s-%d", judge, i+1)}
			for k, v := range f {
				row[k] = v
			}
			findings = append(findings, row)
			if row["blocking"] == true && row["needs_decision"] != true {
				fixable = append(fixable, row)
			}
			if row["needs_decision"] == true {
				needs = append(needs, row)
			}
		}
		if j["verdict"] != "pass" && j["verdict"] != "fail" {
			missing = append(missing, document{"judge": judge, "not_checked": j["not_checked"]})
		}

	}
	var back document
	if b := object(object(judgments["delivery"])["back"]); b["needed"] == true {
		back = b
	}
	state["judgments"] = judgments
	state["prototype"] = prototype
	state["structure_question"] = structureQuestion
	state["back"] = back
	state["fixable"] = fixable
	state["needs_decision"] = needs
	state["not_assessed"] = missing
	if err = write(filepath.Join(rounds(artifacts), fmt.Sprint(number(state["round"])), "settled.json"), state); err != nil {
		return nil, err
	}
	if err = write(filepath.Join(rounds(artifacts), "latest.json"), state); err != nil {
		return nil, err
	}
	if err = write(filepath.Join(artifacts, "plan", "findings.json"), document{"round": state["round"], "errors": state["errors"], "findings": findings, "not_assessed": missing}); err != nil {
		return nil, err
	}
	return document{"round": state["round"], "valid": state["valid"], "fixable_count": len(fixable), "needs_decision_count": len(needs), "not_assessed_count": len(missing), "back": back != nil, "done": state["structure_changed"] == true || state["valid"] == true && len(fixable) == 0 && len(missing) == 0 || back != nil || strings.TrimSpace(prototype) != ""}, nil
}
func findingText(f document) string {
	parts := []string{}
	for _, k := range []string{"row", "failure_scenario", "fix"} {
		if text(f[k]) != "" {
			parts = append(parts, text(f[k]))
		}
	}
	return strings.Join(parts, "\n")
}
func structureRef(items []document) string {
	pairs := [][2]string{}
	for _, i := range items {
		pairs = append(pairs, [2]string{text(i["file"]), text(i["change"])})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return jsLess(strings.Join(pairs[i][:], ","), strings.Join(pairs[j][:], ",")) })
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(pairs)
	data := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte(`\u2028`), []byte("\u2028"))
	data = bytes.ReplaceAll(data, []byte(`\u2029`), []byte("\u2029"))
	sum := sha256.Sum256(data)
	return fmt.Sprintf("structure:%x", sum[:6])
}
func contentHash(row document) string {
	data, _ := json.Marshal(row)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func ownerRowHash(state, row document) string {
	changes := filter(entries(state["changes"]), func(change document) bool {
		if change["quality"] != "structure" {
			return false
		}
		for _, touch := range stringsOf(row["touches"]) {
			if config.Covers(touch, text(change["file"])) || text(change["oldPath"]) != "" && config.Covers(touch, text(change["oldPath"])) {
				return true
			}
		}
		return false
	})
	if len(changes) == 0 {
		return contentHash(row)
	}
	return contentHash(document{"row": row, "structure": changes})
}
func ownerQuestions(state document, answers []document) []document {
	answered := func(kind, ref, hash string) bool {
		for _, a := range answers {
			target := object(a["target"])
			if target["kind"] == kind && target["ref"] == ref && target["content_hash"] == hash && state["requirements_version"] != nil && reflect.DeepEqual(object(a["versions"])["requirements_version"], state["requirements_version"]) {
				return true
			}
		}
		return false
	}
	ask := func(value, kind, ref, hash string) document {
		return document{"text": value, "target": document{"kind": kind, "ref": ref, "content_hash": hash}, "next_workflows": []string{"plan"}, "invalidates": []string{}}
	}
	questions := []document{}
	draft := object(state["draft"])
	for _, d := range entries(draft["decisions"]) {
		if d["decided_by"] != "owner" || d["when"] != "now" || text(d["ref"]) != "" || answered("decision", text(d["id"]), ownerRowHash(state, d)) {
			continue
		}
		parts := []string{text(d["id"]), text(d["what"]), text(d["chosen"])}
		parts = append(parts, stringsOf(d["candidates"])...)
		for _, e := range entries(d["evidence"]) {
			parts = append(parts, text(e["text"]))
		}
		parts = append(parts, stringsOf(d["touches"])...)
		questions = append(questions, ask(strings.Join(parts, "\n"), "decision", text(d["id"]), ownerRowHash(state, d)))
	}
	for i, f := range entries(state["needs_decision"]) {
		row := text(f["row"])
		if row == "" {
			row = fmt.Sprint(i + 1)
		}
		ref := text(f["judge"]) + ":" + row
		if answered("finding", ref, contentHash(f)) {
			continue
		}
		parts := []string{}
		for _, k := range []string{"judge", "failure_scenario", "evidence", "fix"} {
			if text(f[k]) != "" {
				parts = append(parts, text(f[k]))
			}
		}
		questions = append(questions, ask(strings.Join(parts, "\n"), "finding", ref, contentHash(f)))
	}
	structure := filter(entries(state["changes"]), func(change document) bool {
		if change["quality"] != "structure" {
			return false
		}
		for _, row := range entries(draft["decisions"]) {
			if text(row["ref"]) == "" && (row["decided_by"] != "owner" || row["when"] != "now") {
				continue
			}
			for _, touch := range stringsOf(row["touches"]) {
				if config.Covers(touch, text(change["file"])) {
					return false
				}
			}
		}
		return true
	})
	ref := structureRef(structure)
	structureRows := filter(entries(draft["decisions"]), func(d document) bool { return d["quality"] == "structure" })
	hash := contentHash(document{"changes": structure, "rows": structureRows})
	if len(structure) > 0 && !answered("change", ref, hash) {
		parts := []string{text(state["structure_question"]), text(state["skeleton_commit"])}
		for _, i := range structure {
			parts = append(parts, fmt.Sprintf("- %s: %s", text(i["file"]), text(i["change"])))
		}
		for _, d := range entries(draft["decisions"]) {
			if d["quality"] == "structure" {
				parts = append(parts, fmt.Sprintf("- %s: %s -> %s", text(d["id"]), text(d["what"]), text(d["chosen"])))
			}
		}
		questions = append(questions, ask(strings.Join(parts, "\n"), "change", ref, hash))
	}
	return questions
}
func outcomeOf(state document, answers []document) document {
	if strings.TrimSpace(text(state["prototype"])) != "" {
		return document{"outcome": "question", "questions": []document{{"text": state["prototype"], "target": document{"kind": "decision", "ref": "prototype"}, "next_workflows": []string{"plan"}, "invalidates": []string{}}}}
	}
	stop := func(message string, unresolved []string) document {
		return document{"outcome": "stop", "stop": document{"state": message, "unresolved": unresolved, "premise": ""}}
	}
	if state["valid"] != true {
		return stop(fmt.Sprintf("The plan draft still fails the machine checks after round %d", number(state["round"])), stringsOf(state["errors"]))
	}
	if back := object(state["back"]); back != nil {
		return document{"outcome": "back", "back": document{"to": "requirements", "premise": back["premise"], "observation": back["observation"]}}
	}
	if qs := ownerQuestions(state, answers); len(qs) > 0 {
		return document{"outcome": "question", "questions": qs}
	}
	if missing := entries(state["not_assessed"]); len(missing) > 0 {
		unresolved := []string{}
		for _, m := range missing {
			unresolved = append(unresolved, fmt.Sprintf("%s: %s", text(m["judge"]), text(m["not_checked"])))
		}
		return stop(fmt.Sprintf("Some plan judges could not assess round %d", number(state["round"])), unresolved)
	}
	if fixable := entries(state["fixable"]); len(fixable) > 0 {
		carried := []document{}
		for i, f := range fixable {
			id := text(f["id"])
			if id == "" {
				id = fmt.Sprintf("%s-%d", text(f["judge"]), i+1)
			}
			carried = append(carried, document{"id": id, "judge": f["judge"], "text": findingText(f)})
		}
		return document{"outcome": "carry", "carried": carried}
	}
	for judge, value := range object(state["judgments"]) {
		if object(value)["verdict"] != "pass" {
			return stop("A plan judge did not pass", []string{judge + ": " + text(object(value)["verdict"])})
		}
	}
	return document{"outcome": "pass"}
}
func finish(ctx record.ContextData, artifacts string, o record.ResultOptions) (document, error) {
	if o.Now == "" {
		o.Now = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}
	state, err := read(filepath.Join(rounds(artifacts), "latest.json"))
	if err != nil {
		return nil, err
	}
	if state["phase"] != "structure" && state["phase"] != "detail" {
		return nil, fmt.Errorf("plan phase must be structure or detail")
	}
	if ctx.RunID == "" || state["run_id"] != ctx.RunID {
		return nil, fmt.Errorf("plan phase belongs to a different run")
	}
	if _, ok := state["judgments"]; !ok {
		return nil, fmt.Errorf("the last round was not settled")
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return nil, err
	}
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return nil, err
	}
	if requirements == nil || !reflect.DeepEqual(state["requirements_version"], requirements["version"]) {
		return nil, fmt.Errorf("plan phases must use the current requirements version")
	}
	if state["phase"] == "detail" {
		fixed, e := read(filepath.Join(artifacts, "plan", "structure.json"))
		if e != nil {
			return nil, e
		}
		if fixed["phase"] != "structure" || object(object(fixed["judgments"])["structure"])["verdict"] != "pass" || fixed["run_id"] != ctx.RunID || !reflect.DeepEqual(fixed["requirements_version"], state["requirements_version"]) || outcomeOf(fixed, answers)["outcome"] != "pass" {
			return nil, fmt.Errorf("a valid completed structure phase of this run is required")
		}
		object(state["judgments"])["structure"] = object(fixed["judgments"])["structure"]
		state["structure_question"] = fixed["structure_question"]
	}
	outcome := outcomeOf(state, answers)
	if state["phase"] == "structure" && outcome["outcome"] == "carry" {
		unresolved := []string{}
		for _, f := range entries(outcome["carried"]) {
			unresolved = append(unresolved, text(f["text"]))
		}
		outcome = document{"outcome": "stop", "stop": document{"state": "Structure findings remain; restart plan before detail", "unresolved": unresolved, "premise": ""}}
	}
	if text(state["phase"]) != "" {
		loops := []document{}
		if state["phase"] == "detail" {
			fixed, e := read(filepath.Join(artifacts, "plan", "structure.json"))
			if e != nil {
				return nil, e
			}
			loops = append(loops, loopResult(fixed))
		}
		outcome["loops"] = append(loops, loopResult(state))
	}
	if outcome["outcome"] == "pass" && (state["phase"] != "detail" || object(object(state["judgments"])["delivery"])["verdict"] != "pass") {
		return nil, fmt.Errorf("a completed detail phase is required")
	}
	if outcome["outcome"] == "pass" {
		commit, err := agreeRows(ctx, object(state["draft"]), state)
		if err != nil {
			return nil, err
		}
		state["skeleton_commit"] = commit
		o.Commit = commit
	}
	history := []document{}
	for n := 1; n <= number(state["round"]); n++ {
		round, e := readOr(filepath.Join(rounds(artifacts), fmt.Sprint(n), "settled.json"), nil)
		if e != nil {
			return nil, e
		}
		if round != nil {
			history = append(history, round)
		}
	}
	if len(history) == 0 {
		history = append(history, state)
	}
	var recorded document
	for _, round := range history {
		if round["valid"] == true {
			recorded = round
		}
	}
	if state["valid"] == true {
		recorded = state
	}
	version := 0
	if recorded != nil {
		d := object(recorded["draft"])
		requirements, err := record.Latest(ctx.Dir, "requirements")
		if err != nil {
			return nil, err
		}
		body := document{"skeleton_commit": recorded["skeleton_commit"]}
		for _, k := range []string{"decisions", "increments", "tests", "kept_behaviors"} {
			body[k] = d[k]
		}
		saved, err := record.Write(ctx, "plan", body, o.Now)
		if err != nil {
			return nil, err
		}
		version = number(saved.Entry["version"])
		records := []document{}
		for _, round := range history {
			judgments := object(round["judgments"])
			for _, judge := range concerns.Names() {
				j, ok := judgments[judge]
				if !ok {
					continue
				}
				judgment := object(j)
				input, err := read(filepath.Join(artifacts, "plan", "input.json"))
				if err != nil {
					return nil, err
				}
				paths := stringsOf(object(object(input["quality_materials"])[judge])["criteria"])
				paths = append(paths, fmt.Sprintf("Judgment materials (%s.json, round %d)", judge, number(round["round"])))
				observations := []string{}
				if text(judgment["checked"]) != "" {
					observations = append(observations, text(judgment["checked"]))
				}
				if text(judgment["not_checked"]) != "" {
					observations = append(observations, "Not checked: "+text(judgment["not_checked"]))
				}
				findings := []document{}
				for i, f := range entries(judgment["findings"]) {
					findings = append(findings, document{"id": fmt.Sprintf("%s-%d", judge, i+1), "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": findingText(f), "evidence": f["evidence"], "deferred": f["deferred"] == true, "mechanical": f["mechanical"] == true, "resolution_key": text(f["row"]) + "\n" + text(f["failure_scenario"])})
				}
				records = append(records, document{"judge": judge, "phase": round["phase"], "round": round["round"], "kind": "plan", "target": document{"requirements_version": requirements["version"], "plan_version": version, "skeleton_commit": round["skeleton_commit"]}, "materials": paths, "observations": observations, "verdict": judgment["verdict"], "findings": findings})
			}
		}
		if err = record.WriteLoopJudgments(ctx, records, o.Now); err != nil {
			return nil, err
		}
	}
	o.RunID = ctx.RunID
	result, err := record.WriteResult(ctx.Dir, "plan", outcome, o)
	if err != nil {
		return nil, err
	}
	questions := []document{}
	for _, q := range entries(result["questions"]) {
		questions = append(questions, document{"id": q["id"], "text": q["text"]})
	}
	carried := []string{}
	for _, c := range entries(result["carried"]) {
		carried = append(carried, fmt.Sprintf("%s: %s", text(c["id"]), text(c["text"])))
	}
	reasons := stringsOf(object(result["stop"])["unresolved"])
	if back := object(result["back"]); back != nil {
		reasons = []string{fmt.Sprintf("%s: %s", text(back["premise"]), text(back["observation"]))}
	}
	return document{"outcome": result["outcome"], "plan_version": version, "skeleton_commit": state["skeleton_commit"], "questions": questions, "carried": carried, "reasons": reasons}, nil
}

func loopResult(state document) document {
	ended := "cleared"
	if len(entries(state["fixable"])) > 0 || len(entries(state["not_assessed"])) > 0 || state["valid"] != true {
		ended = "max_iterations"
	}
	if state["structure_changed"] == true {
		ended = "stuck"
	}
	return document{"name": state["phase"], "ended": ended, "rounds": number(state["phase_round"])}
}
