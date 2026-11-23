package pr

import (
	"fmt"
	"strings"

	"github.com/ildjenty/dev-kit/internal/record"
)

func version(v any, missing string) string {
	if v == nil {
		return missing
	}
	return fmt.Sprint(v)
}
func failures(step []doc, versions doc) []string {
	out := []string{}
	for _, j := range step {
		target := object(j["target"])
		at := fmt.Sprintf("%s (run %s)", text(j["judge"]), text(j["run_id"]))
		if version(target["requirements_version"], "null") != version(versions["requirements_version"], "null") || version(target["plan_version"], "null") != version(versions["plan_version"], "null") {
			out = append(out, fmt.Sprintf("%s judged requirements %s and plan %s, not the latest %s and %s", at, version(target["requirements_version"], "undefined"), version(target["plan_version"], "none"), version(versions["requirements_version"], "null"), version(versions["plan_version"], "none")))
		}
		if j["verdict"] != "pass" {
			out = append(out, at+" is "+text(j["verdict"]))
		}
		for _, f := range docs(j["findings"]) {
			if truth(f["blocking"]) {
				out = append(out, fmt.Sprintf("%s has a blocking finding %s: %s", at, text(f["id"]), text(f["text"])))
			}
		}
	}
	return out
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func verifyChain(judgments []doc, head string, versions doc, id *string) doc {
	step := record.JudgedStep(judgments, id, head)
	if step == nil {
		patch := "none"
		if id != nil {
			patch = *id
		}
		return doc{"ok": false, "steps": []doc{}, "reasons": []string{fmt.Sprintf("no code judgment judged the change of %s (change-id %s)", short(head), patch)}, "judged_commit": nil}
	}
	reasons := failures(step.Judgments, versions)
	judges := []string{}
	hasScope := false
	for _, j := range step.Judgments {
		judges = append(judges, text(j["judge"]))
		if j["judge"] == "scope" {
			hasScope = true
		}
	}
	if !hasScope {
		reasons = append(reasons, "the judgment of "+short(step.Commit)+" has no "+strings.Join([]string{"scope"}, ", ")+" judge")
	}
	return doc{"ok": len(reasons) == 0, "steps": []doc{{"commit": step.Commit, "run_id": step.RunID, "judges": judges}}, "reasons": reasons, "judged_commit": step.Commit}
}
