package pr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
)

func parseText(s string) (doc, error) {
	var d doc
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil, err
	}
	for _, key := range []string{"title", "opening", "how_to_see"} {
		if strings.TrimSpace(text(d[key])) == "" {
			return nil, &record.RecordError{Message: "the writer left " + key + " empty"}
		}
	}
	if strings.Contains(text(d["title"]), "\n") {
		return nil, &record.RecordError{Message: "the title is more than one line"}
	}
	for _, key := range []string{"what_changed", "how_checked", "what_remains", "decisions", "after_merge"} {
		if strings.TrimSpace(text(object(d["headings"])[key])) == "" {
			return nil, &record.RecordError{Message: "the writer left heading " + key + " empty"}
		}
	}
	return d, nil
}
func line(parts ...any) string {
	out := []string{}
	for _, v := range parts {
		if v != nil && v != "" {
			out = append(out, fmt.Sprint(v))
		}
	}
	return "- " + strings.Join(out, " · ")
}
func body(d, lists doc) string {
	lines := []string{}
	headings := object(d["headings"])
	part := func(key, prose string, items []string) {
		lines = append(lines, "### "+text(headings[key]), "", strings.TrimSpace(prose), "")
		lines = append(lines, items...)
		lines = append(lines, "")
	}
	items := []string{}
	for _, i := range docs(lists["changed"]) {
		items = append(items, line(i["status"], i["path"], i["oldPath"], i["quality"], i["allowed_by"]))
	}
	part("what_changed", text(d["opening"]), items)
	items = []string{}
	for _, c := range docs(lists["checked"]) {
		v := object(c["verification"])
		items = append(items, line(c["id"], c["text"], v["method"], v["file"], v["name"]))
	}
	for _, j := range docs(lists["judgments"]) {
		items = append(items, line(j["judge"], j["verdict"]))
		for _, o := range stringsOf(j["observations"]) {
			items = append(items, line(o))
		}
	}
	part("how_checked", text(d["how_to_see"]), items)
	items = []string{}
	for _, f := range docs(lists["remaining"]) {
		items = append(items, line(f["judge"], f["id"], f["text"]))
	}
	for _, t := range docs(lists["not_checked"]) {
		items = append(items, line(t["criterion"], t["method"]))
	}
	part("what_remains", "", items)
	items = []string{}
	for _, d := range docs(lists["decisions"]) {
		parts := []any{d["id"], d["decided_by"], d["what"], d["chosen"]}
		for _, p := range stringsOf(d["principles"]) {
			parts = append(parts, p)
		}
		items = append(items, line(parts...))
	}
	for _, a := range docs(lists["answers"]) {
		items = append(items, line(a["question_id"], a["question"], a["answer"]))
	}
	part("decisions", "", items)
	part("after_merge", text(d["after_merge"]), nil)
	return strings.Join(lines, "\n")
}
func judgmentOf(judge, facts doc, materials []string) doc {
	findings := []doc{}
	for i, f := range docs(judge["findings"]) {
		id := f["id"]
		if text(id) == "" {
			id = fmt.Sprintf("R%d", i+1)
		}
		findings = append(findings, doc{"id": id, "blocking": f["blocking"], "needs_decision": f["needs_decision"], "text": f["text"], "evidence": f["evidence"]})
	}
	versions := object(facts["versions"])
	target := doc{"requirements_version": versions["requirements_version"], "base": facts["base"], "commit": facts["head"]}
	if versions["plan_version"] != nil {
		target["plan_version"] = versions["plan_version"]
	}
	observations := judge["observations"]
	if observations == nil {
		observations = []string{}
	}
	return doc{"judge": "report", "kind": "report", "target": target, "materials": materials, "observations": observations, "verdict": judge["verdict"], "findings": findings}
}
func passed(j doc) bool {
	if j["verdict"] != "pass" {
		return false
	}
	for _, f := range docs(j["findings"]) {
		if truth(f["blocking"]) || truth(f["needs_decision"]) {
			return false
		}
	}
	return true
}
func notConvergedDoc(j doc) doc {
	unresolved := []string{}
	findings := docs(j["findings"])
	if len(findings) > 0 {
		for _, f := range findings {
			if truth(f["blocking"]) || truth(f["needs_decision"]) {
				unresolved = append(unresolved, text(f["id"])+": "+text(f["text"]))
			}
		}
	} else {
		unresolved = append(unresolved, "The report verdict is "+text(j["verdict"]))
	}
	return stopDoc("The PR body did not pass in three rounds; no pull request was opened", unresolved)
}

var materials = []string{"pr/pr.md", "pr/pr-title.txt", "pr/lists.json", "pr/diff.patch", "pr/requirements.json", "pr/judgments.json", "pr/criteria/"}

func reportMaterials(title, body []byte) []string {
	out := append([]string{}, materials...)
	return append(out, fmt.Sprintf("pr/pr-title.txt#sha256=%x", sha256.Sum256(title)), fmt.Sprintf("pr/pr.md#sha256=%x", sha256.Sum256(body)))
}

func report(mode string, env cli.Env) error {
	out := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr")
	stateFile := env.Path(filepath.Join(out, "report-state.json"))
	switch mode {
	case "assemble":
		d, err := parseText(env.Getenv("INPUTS_TEXT"))
		if err != nil {
			return err
		}
		lists, err := read(env.Path(filepath.Join(out, "lists.json")))
		if err != nil {
			return err
		}
		previous, err := optional(stateFile)
		if err != nil {
			return err
		}
		if previous != nil {
			previous["passed"] = false
			if err = save(stateFile, previous); err != nil {
				return err
			}
		}
		if err = os.WriteFile(env.Path(filepath.Join(out, "pr.md")), []byte(body(d, lists)), 0644); err != nil {
			return err
		}
		if err = os.WriteFile(env.Path(filepath.Join(out, "pr-title.txt")), []byte(strings.TrimSpace(text(d["title"]))+"\n"), 0644); err != nil {
			return err
		}
		return env.JSON(doc{"path": filepath.Join(out, "pr.md")})
	case "judgment":
		ctx, err := context(env)
		if err != nil {
			return err
		}
		facts, err := read(env.Path(filepath.Join(out, "facts.json")))
		if err != nil {
			return err
		}
		var judged doc
		if err = json.Unmarshal([]byte(env.Getenv("INPUTS_JUDGMENT")), &judged); err != nil {
			return err
		}
		title, err := os.ReadFile(env.Path(filepath.Join(out, "pr-title.txt")))
		if err != nil {
			return err
		}
		body, err := os.ReadFile(env.Path(filepath.Join(out, "pr.md")))
		if err != nil {
			return err
		}
		judgment := judgmentOf(judged, facts, reportMaterials(title, body))
		entry, err := record.Write(ctx, "judgment", judgment, "")
		if err != nil {
			return err
		}
		previous, err := optional(stateFile)
		if err != nil {
			return err
		}
		round := 1
		if previous != nil {
			round = int(previous["round"].(float64)) + 1
		}
		ok := passed(judgment)
		state := doc{"round": round, "passed": ok, "verdict": judgment["verdict"], "judgment": entry.Number, "findings": judgment["findings"]}
		if err = save(stateFile, state); err != nil {
			return err
		}
		if err = save(env.Path(filepath.Join(out, "report-findings.json")), judgment["findings"]); err != nil {
			return err
		}
		return env.JSON(doc{"passed": ok, "round": round})
	case "state":
		state, err := optional(stateFile)
		if err != nil {
			return err
		}
		if state == nil {
			return &record.RecordError{Message: "the report judge left no judgment"}
		}
		if !truth(state["passed"]) {
			if err = save(env.Path(filepath.Join(out, "outcome.json")), notConvergedDoc(state)); err != nil {
				return err
			}
		}
		return env.JSON(doc{"passed": state["passed"], "round": state["round"]})
	}
	return cli.Usage("devkit pr assemble | judgment | state")
}
