package plan

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func agreeRows(ctx record.ContextData, draft document, states ...document) (string, error) {
	requirements, err := record.Latest(ctx.Dir, "requirements")
	if err != nil {
		return "", err
	}
	answers, err := record.ListKeyed(ctx.Dir, "answer")
	if err != nil {
		return "", err
	}
	for _, row := range entries(draft["decisions"]) {
		if row["decided_by"] != "owner" || row["when"] != "now" {
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
	return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
}
