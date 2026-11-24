package refine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/claudecli"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/codexcli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
)

const relevanceSchema = `{"type":"object","additionalProperties":false,"properties":{"relevant_ids":{"type":"array","uniqueItems":true,"items":{"type":"string","pattern":"^D-[0-9a-f]{6,64}$"}}},"required":["relevant_ids"]}`

func decisionHash(ds []doc) string {
	b, _ := json.Marshal(ds)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func decisionIndex(ds []doc) []doc {
	index := []doc{}
	for _, d := range ds {
		index = append(index, doc{"id": d["id"], "title": d["title"], "status": d["status"], "depends_on": stringsOf(d["depends_on"]), "supersedes": stringsOf(d["supersedes"]), "path": d["path"]})
	}
	return index
}

func alwaysRelevant(input doc, data decisions.Data) map[string]bool {
	selected := map[string]bool{}
	add := func(ids []string) {
		for _, id := range ids {
			if find(data.Decisions, id) != nil {
				selected[id] = true
			}
		}
	}
	var dependencies func(string)
	dependencies = func(id string) {
		if selected[id] {
			return
		}
		add([]string{id})
		for _, dep := range stringsOf(find(data.Decisions, id)["depends_on"]) {
			dependencies(dep)
		}
	}
	for _, id := range stringsOf(obj(input["topic"])["depends_on"]) {
		dependencies(id)
	}
	for _, value := range []any{input["topic"], input["pending_proposal"], input["previous"]} {
		b, _ := json.Marshal(value)
		add(decisions.Citations(string(b)))
	}
	for {
		before := len(selected)
		for _, d := range data.Decisions {
			for _, old := range stringsOf(d["supersedes"]) {
				if selected[str(d["id"])] || selected[old] {
					add([]string{str(d["id"]), old})
				}
			}
		}
		if len(selected) == before {
			break
		}
	}
	cocited := []string{}
	for _, unit := range data.Units {
		if unit["state"] != "open" {
			continue
		}
		ids := stringsOf(unit["decisions"])
		for _, id := range ids {
			if selected[id] {
				cocited = append(cocited, ids...)
				break
			}
		}
	}
	add(cocited)
	return selected
}

func relevance(env cli.Env, cwd, artifacts string, input doc, data decisions.Data, settings config.Config) ([]doc, error) {
	always := alwaysRelevant(input, data)
	candidates, agreed := []doc{}, []doc{}
	alwaysIDs := []string{}
	for _, d := range data.Decisions {
		if always[str(d["id"])] {
			alwaysIDs = append(alwaysIDs, str(d["id"]))
		}
		if d["status"] == "agreed" {
			agreed = append(agreed, d)
			if !always[str(d["id"])] {
				candidates = append(candidates, doc{"id": d["id"], "title": d["title"], "body": d["body"], "path": d["path"]})
			}
		}
	}
	material := doc{"topic": doc{"title": obj(input["topic"])["title"], "body": obj(input["topic"])["body"]}, "requests": input["requests"], "memo": input["memo"]}
	selection := doc{"material": material, "decisions": candidates, "always_ids": alwaysIDs, "language": input["language"], "standards": obj(input["folders"])["standards"]}
	if err := writeJSON(file(artifacts, "relevance-input.json"), selection); err != nil {
		return nil, err
	}
	result := doc{"source": "always", "reason": ""}
	selected := map[string]doc{}
	if len(candidates) > 0 {
		baseEnv := env
		env.Cwd = cwd
		env.LookupEnv = func(key string) (string, bool) {
			if key == "ARTIFACTS_DIR" {
				return artifacts, true
			}
			return baseEnv.Getenv(key), true
		}
		runCLI := claudecli.Run
		if settings.Provider == "codex" {
			runCLI = codexcli.Run
		}
		answer, err := runCLI(env, "refine", "refine/commands/relevance.md", relevanceSchema, codexcli.Timeout)
		if err == nil {
			for _, d := range candidates {
				id := str(d["id"])
				selected[id] = doc{"id": id, "source": settings.Provider, "included": false}
			}
			for _, id := range stringsOf(answer["relevant_ids"]) {
				if find(candidates, id) == nil {
					err = fmt.Errorf("CLI returned an unknown or non-candidate decision")
					break
				}
				selected[id] = doc{"id": id, "source": settings.Provider, "included": true}
			}
		}
		result["source"] = settings.Provider
		if err != nil {
			result["source"] = "all"
			result["reason"] = settings.Provider + " unavailable: " + err.Error()
			selected = map[string]doc{}
			for _, d := range agreed {
				id := str(d["id"])
				selected[id] = doc{"id": id, "source": "all", "included": true}
			}
		}
	}
	entries, relevant := []doc{}, []doc{}
	for _, d := range data.Decisions {
		id := str(d["id"])
		entry := selected[id]
		if always[id] {
			entry = doc{"id": id, "source": "always", "included": true}
		}
		if entry == nil {
			entry = doc{"id": id, "source": "excluded", "included": false}
		}
		entries = append(entries, entry)
		if entry["included"] == true {
			relevant = append(relevant, d)
		}
	}
	result["decisions"] = entries
	if err := writeJSON(file(artifacts, "relevance.json"), result); err != nil {
		return nil, err
	}
	return relevant, nil
}
