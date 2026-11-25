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
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

func ownerInput(env cli.Env) ([]doc, error) {
	mode := env.Getenv("INPUTS_AI")
	if mode != "" && mode != "additions" && mode != "none" {
		return nil, fmt.Errorf("ai must be additions or none")
	}
	var value any = []any{}
	if raw := env.Getenv("INPUTS_OWNER_DECISIONS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("owner_decisions: %w", err)
		}
	}
	problems, err := schema("owner-decisions", value)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, &record.RecordError{Message: "invalid owner_decisions", Errors: problems}
	}
	result := items(value)
	for _, d := range result {
		for _, field := range []string{"title", "body", "owner_words"} {
			if strings.TrimSpace(str(d[field])) == "" {
				return nil, fmt.Errorf("owner_decisions: %s must not be blank", field)
			}
		}
		for _, field := range []string{"supersedes", "depends_on"} {
			if d[field] == nil {
				d[field] = []any{}
			}
		}
	}
	if mode == "none" && len(result) == 0 {
		return nil, fmt.Errorf("ai=none requires owner_decisions")
	}
	return result, nil
}

func statusText(ctx record.ContextData, d doc, status string) (string, error) {
	b, err := os.ReadFile(filepath.Join(ctx.Cwd, str(d["path"])))
	if err != nil {
		return "", err
	}
	text := string(b)
	end := strings.Index(text[3:], "\n---") + 3
	if end < 3 {
		return "", fmt.Errorf("missing front matter: %s", d["path"])
	}
	return regexp.MustCompile(`(?m)^status:.*$`).ReplaceAllString(text[:end], "status: "+strconv.Quote(status)) + text[end:], nil
}

func materializeOwner(ctx record.ContextData, input []doc) ([]doc, error) {
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	writes := map[string]string{}
	successors := map[string]string{}
	for _, given := range input {
		encoded, err := json.Marshal(given)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("owner:%x", sha256.Sum256(encoded))
		var existing doc
		for _, d := range data.Decisions {
			if d["request"] == ctx.Branch && d["owner_input"] == key {
				existing = d
				break
			}
		}
		if existing != nil {
			continue
		}
		id, err := decisions.NextID("decision", ctx.Branch, key, data)
		if err != nil {
			return nil, err
		}
		d := clone(given)
		d["id"], d["status"], d["decided_by"], d["request"] = id, "agreed", "owner", ctx.Branch
		d["provenance"], d["owner_input"], d["path"] = "owner", key, c.Decisions+"/"+id+".md"
		front := recordText(d, []string{"id", "title", "status", "decided_by", "provenance", "owner_input", "depends_on", "supersedes", "request"}, "")
		words, err := json.Marshal(given["owner_words"])
		if err != nil {
			return nil, err
		}
		text := strings.TrimSuffix(front, "\n") + str(given["body"]) + "\n\nOwner words: " + string(words) + "\n"
		parsed, err := decisions.Parse(text)
		if err != nil {
			return nil, err
		}
		parsed["path"] = d["path"]
		data.Decisions = append(data.Decisions, parsed)
		writes[str(d["path"])] = text
		for _, oldID := range stringsOf(d["supersedes"]) {
			old := find(data.Decisions, oldID)
			if old == nil || old["status"] != "agreed" || successors[oldID] != "" {
				return nil, fmt.Errorf("owner_decisions: invalid supersedes %s", oldID)
			}
			successors[oldID] = id
			text, err := statusText(ctx, old, "superseded")
			if err != nil {
				return nil, err
			}
			writes[str(old["path"])] = text
			old["status"] = "superseded"
		}
	}
	if problems := decisions.Check(data); len(problems) > 0 {
		return nil, &record.RecordError{Message: "owner_decisions failed the decision check", Errors: problems}
	}
	paths := []string{}
	for path, text := range writes {
		if err := put(filepath.Join(ctx.Cwd, path), text); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if len(paths) > 0 {
		if err := proposal.CommitPaths(ctx, paths, "docs: record owner decisions"); err != nil {
			return nil, err
		}
	}
	fixed := []doc{}
	for _, d := range data.Decisions {
		if d["request"] == ctx.Branch && d["provenance"] == "owner" {
			fixed = append(fixed, d)
		}
	}
	slices.SortFunc(fixed, func(a, b doc) int { return strings.Compare(str(a["id"]), str(b["id"])) })
	return fixed, nil
}

func decisionIDs(content doc) []string {
	ids := stringsOf(content["owner_decision_ids"])
	for _, id := range stringsOf(content["decision_ids"]) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func publishOwner(ctx record.ContextData, artifacts string, env cli.Env) (doc, error) {
	input, err := readJSON(file(resolve(ctx.Cwd, artifacts), "input.json"))
	if err != nil {
		return nil, err
	}
	if input["ai"] != "none" || len(items(input["fixed_owner_decisions"])) == 0 {
		return nil, fmt.Errorf("owner publication requires prepared owner_decisions and ai=none")
	}
	baseline, err := readJSON(file(resolve(ctx.Cwd, artifacts), "baseline.json"))
	if err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	status, err := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if head != baseline["commit"] || status != "" {
		return nil, fmt.Errorf("owner publication requires the clean prepared HEAD")
	}
	options, err := resultOptions(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := record.BuildResult(ctx.Dir, "refine", doc{"outcome": "pass"}, options); err != nil {
		return nil, err
	}
	fixed := items(input["fixed_owner_decisions"])
	titles, ids := []string{}, []string{}
	for _, d := range fixed {
		titles, ids = append(titles, str(d["title"])), append(ids, str(d["id"]))
	}
	title, summary := fmt.Sprintf("Record %d owner decisions", len(titles)), "Record owner decisions:"
	lang := strings.ToLower(strings.ReplaceAll(str(input["language"]), "_", "-"))
	if lang == "ja" || strings.HasPrefix(lang, "ja-") {
		title, summary = fmt.Sprintf("所有者の判断を%d件記録", len(titles)), "所有者の判断を記録します。"
	}
	draft := doc{"pr_title": title, "pr_summary": summary + "\n\n- " + strings.Join(titles, "\n- "), "decisions": []doc{}, "bindings": []doc{}, "units": []doc{}, "topics": []doc{}}
	content := doc{"ai": "none", "draft": draft, "commit": head, "owner_decisions": fixed, "owner_decision_ids": ids, "decision_ids": []string{}, "ready": true}
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	previous := obj(p["content"])
	if previous["ai"] != "none" || previous["commit"] != head || !reflect.DeepEqual(previous["owner_decision_ids"], array(ids)) {
		written, err := record.Write(ctx, "proposal", doc{"content": content}, "")
		if err != nil {
			return nil, err
		}
		p = written.Entry
	} else {
		content = previous
	}
	if err := proposal.SaveDetails(ctx, p); err != nil {
		return nil, err
	}
	progress, err := proposal.PublishProposal(ctx, env, content, creationPath(ctx, fmt.Sprint(num(p["version"]))))
	if err != nil {
		return nil, err
	}
	saved, err := record.WriteResult(ctx.Dir, "refine", doc{"outcome": "pass", "proposal_version": num(p["version"]), "decision_ids": ids, "issue_numbers": []int{}, "pr_number": progress.PRNumber, "pr_url": progress.PRURL, "publication": progress.Publication}, options)
	if err != nil {
		return nil, err
	}
	return output(saved, num(p["version"])), nil
}
