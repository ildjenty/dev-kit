package refine

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func selectedTopic(ctx record.ContextData, id string) (doc, error) {
	previous, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	result, err := record.Latest(ctx.Dir, "result")
	if err != nil {
		return nil, err
	}
	if id == "" {
		if result["outcome"] != "pass" {
			id = str(obj(obj(previous["content"])["topic"])["id"])
		}
	}
	if id == "" {
		return nil, nil
	}
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	topic := find(decisions.PickableTopics(data), id)
	if topic == nil {
		prior := obj(obj(previous["content"])["topic"])
		if prior["id"] == id && find(data.Topics, id) == nil && obj(previous["content"])["materialized"] != true {
			return prior, nil
		}
		return nil, fmt.Errorf("topic %s is not pickable; every dependency must be agreed", id)
	}
	return topic, nil
}

func topicText(topic doc) string {
	return recordText(topic, []string{"id", "title", "status", "depends_on"}, str(topic["body"]))
}

func topicErrors(input, draft doc, known []doc) []string {
	errors := []string{}
	graph := append([]doc{}, known...)
	for _, d := range items(draft["decisions"]) {
		d = clone(d)
		d["id"] = d["key"]
		graph = append(graph, d)
	}
	topics := []doc{}
	for _, topic := range items(input["topics"]) {
		owned := input["previous_unapproved"] == true && has(obj(obj(input["pending_proposal"])["content"])["topic_ids"], str(topic["id"]))
		if topic["id"] != obj(input["topic"])["id"] && !owned {
			topics = append(topics, topic)
		}
	}
	for _, topic := range items(draft["topics"]) {
		topics = append(topics, topic)
		for _, id := range stringsOf(topic["depends_on"]) {
			if find(graph, id) == nil {
				errors = append(errors, str(topic["key"])+": missing decision "+id)
			}
		}
		for _, id := range decisions.Citations(str(topic["body"])) {
			if find(graph, id) == nil {
				errors = append(errors, str(topic["key"])+": missing citation "+id)
			}
		}
	}
	for _, u := range items(draft["units"]) {
		area := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if area[id] {
				return
			}
			area[id] = true
			for _, dep := range stringsOf(find(graph, id)["depends_on"]) {
				visit(dep)
			}
		}
		for _, id := range stringsOf(u["decisions"]) {
			visit(id)
		}
		for _, topic := range topics {
			if topic["status"] == "settled" {
				continue
			}
			deps := stringsOf(topic["depends_on"])
			related := len(deps) == 0
			seen := map[string]bool{}
			var overlaps func(string) bool
			overlaps = func(id string) bool {
				if area[id] {
					return true
				}
				if seen[id] {
					return false
				}
				seen[id] = true
				for _, dep := range stringsOf(find(graph, id)["depends_on"]) {
					if overlaps(dep) {
						return true
					}
				}
				return false
			}
			for _, id := range deps {
				related = related || overlaps(id)
			}
			if related {
				errors = append(errors, str(u["key"])+": area has an open topic "+str(topic["id"])+str(topic["key"]))
			}
		}
	}
	return errors
}

func phaseAspects(phase string) []string {
	if phase == "decide" {
		return []string{"coverage", "additions", "open_points", "consistency", "sufficiency"}
	}
	return []string{"wording"}
}

func phaseErrors(ctx record.ContextData, artifacts, phase string, draft, input doc) []string {
	errors := []string{}
	if phase != "decide" && phase != "document" {
		return []string{"phase must be decide or document"}
	}
	if phase == "decide" {
		if len(array(draft["units"]))+len(array(draft["kept_docs"]))+len(array(draft["kept_units"])) > 0 {
			errors = append(errors, "decide may only write decisions, topics and open points")
		}
		paths, err := changes(ctx.Cwd)
		if err != nil {
			return append(errors, err.Error())
		}
		if len(paths) > 0 {
			errors = append(errors, "decide may not edit the checkout")
		}
	} else {
		frozen, err := readJSON(file(artifacts, "decided.json"))
		if err != nil || frozen["cleared"] != true || len(array(obj(frozen["draft"])["open_points"])) > 0 {
			return []string{"document requires a cleared decide phase"}
		}
		for _, field := range []string{"decisions", "topics", "rebased", "open_points"} {
			a, _ := json.Marshal(draft[field])
			b, _ := json.Marshal(obj(frozen["draft"])[field])
			if string(a) != string(b) {
				errors = append(errors, "document changed settled "+field)
			}
		}
		if len(array(draft["open_points"])) > 0 {
			errors = append(errors, "document requires settled open points")
		}
		docs, err := docsSnapshot(ctx.Cwd, str(obj(input["folders"])["docs"]))
		if err != nil {
			return append(errors, err.Error())
		}
		for _, decision := range items(draft["decisions"]) {
			key := str(decision["key"])
			reflected := false
			for _, body := range docs {
				for _, token := range draftKeys.FindAllString(body, -1) {
					reflected = reflected || token == key
				}
				old := str(obj(obj(obj(input["pending_proposal"])["content"])["mapping"])[key])
				reflected = reflected || old != "" && slices.Contains(decisions.Citations(body), old)
			}
			if !reflected {
				errors = append(errors, key+": new decision is not reflected in a document")
			}
		}
	}
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return append(errors, err.Error())
	}
	a, _ := json.Marshal(data.Topics)
	b, _ := json.Marshal(input["topics"])
	if string(a) != string(b) {
		errors = append(errors, "writer edited topic records directly")
	}
	return errors
}

func decided(ctx record.ContextData, artifacts string) (doc, error) {
	artifacts = resolve(ctx.Cwd, artifacts)
	entry, err := readJSON(file(artifacts, "decide/rounds/latest.json"))
	if err != nil {
		return nil, err
	}
	problems, err := draftErrors(ctx, artifacts, entry["draft"], "decide")
	if err != nil {
		return nil, err
	}
	ready := entry["cleared"] == true && len(problems) == 0 && len(array(obj(entry["draft"])["open_points"])) == 0
	if ready {
		if err = writeJSON(file(artifacts, "decided.json"), entry); err != nil {
			return nil, err
		}
	}
	return doc{"ready": ready}, nil
}

func commitPaths(cwd string, paths []string, message string) (string, error) {
	changed, err := changes(cwd)
	if err != nil {
		return "", err
	}
	for _, path := range changed {
		if !slices.Contains(paths, path) {
			return "", fmt.Errorf("commit includes an unapproved path: %s", path)
		}
	}
	return gitx.Run(cwd, append([]string{"commit", "--only", "-m", message, "--"}, paths...)...)
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
	valid := false
	for _, run := range h.Runs {
		if run["run_id"] == ctx.RunID && run["workflow"] == "refine" && has(run["answers"], str(q["id"])) {
			valid = true
		}
	}
	if !valid || answer["run_id"] != ctx.RunID || answer["workflow"] != "refine" || answer["answer"] != "approve="+version || obj(answer["target"])["kind"] != "proposal" || obj(answer["target"])["ref"] != version || answer["commit"] != result["commit"] {
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
