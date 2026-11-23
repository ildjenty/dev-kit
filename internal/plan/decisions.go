package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"gopkg.in/yaml.v3"
)

func agreeRows(ctx record.ContextData, draft document, states ...document) (string, error) {
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return "", err
	}
	data, err := decisions.List(ctx.Cwd)
	if err != nil {
		return "", err
	}
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return "", err
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return "", err
	}
	for _, row := range entries(draft["decisions"]) {
		if row["decided_by"] != "owner" || row["when"] != "now" || text(row["ref"]) != "" {
			continue
		}
		state := document{}
		if len(states) > 0 {
			state = states[0]
		}
		bound := false
		for _, answer := range answers {
			target := object(answer["target"])
			if target["kind"] == "decision" && target["ref"] == row["id"] && target["content_hash"] == ownerRowHash(state, row) && requirements != nil && reflect.DeepEqual(object(answer["versions"])["requirements_version"], requirements["version"]) && strings.TrimSpace(text(answer["answer"])) != "" {
				bound = true
			}
		}
		if !bound {
			return "", fmt.Errorf("owner row %s has no bound answer", text(row["id"]))
		}
	}
	created := false
	for _, row := range entries(draft["decisions"]) {
		if row["when"] != "now" || text(row["ref"]) != "" || row["decided_by"] != "owner" && row["decided_by"] != "principle" {
			continue
		}
		id, err := decisions.NextID("decision", ctx.Branch, "plan:"+text(row["id"]), data)
		if err != nil {
			return "", err
		}
		raw, err := json.Marshal(row)
		if err != nil {
			return "", err
		}
		deps := decisions.Citations(string(raw))
		front, err := yaml.Marshal(document{"id": id, "title": row["what"], "status": "agreed", "decided_by": row["decided_by"], "request": ctx.Branch, "depends_on": deps, "supersedes": []string{}})
		if err != nil {
			return "", err
		}
		body, err := json.MarshalIndent(row, "", "  ")
		if err != nil {
			return "", err
		}
		path := filepath.Join(ctx.Cwd, c.Decisions, id+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf("---\n%s---\n%s\n\n```json\n%s\n```\n", front, text(row["chosen"]), body)), 0644); err != nil {
			return "", err
		}
		created = true
		row["ref"] = id
		data.Decisions = append(data.Decisions, document{"id": id})
	}
	if !created {
		return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	}
	materialized, err := decisions.List(ctx.Cwd)
	if err != nil {
		return "", err
	}
	if problems := decisions.Check(materialized); len(problems) > 0 {
		return "", &record.RecordError{Message: "agreed decisions are invalid", Errors: problems}
	}
	message, err := gitx.Run(ctx.Cwd, "log", "-1", "--format=%B")
	if err != nil {
		return "", err
	}
	runs, err := record.List(ctx.Dir, "run")
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(message, "feat: skeleton: ") {
		for _, run := range runs {
			if run["workflow"] != "plan" || !strings.Contains("\n"+message+"\n", "\nArchon-Run: "+text(run["run_id"])+"\n") {
				continue
			}
			if _, err := gitx.Run(ctx.Cwd, "add", "-A"); err != nil {
				return "", err
			}
			if _, err := gitx.Run(ctx.Cwd, "commit", "--amend", "--no-edit", "-q"); err != nil {
				return "", err
			}
			return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
		}
	}
	return commitSkeleton(ctx, "agreed decisions")
}
