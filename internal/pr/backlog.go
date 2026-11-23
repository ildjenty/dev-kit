package pr

import (
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func backlog(judgments, results []doc) doc {
	out := doc{"deferred": []doc{}, "carried": []doc{}, "mechanical": []doc{}, "accepted_objections": []doc{}}
	for i, j := range judgments {
		source := doc{"run_id": j["run_id"], "judgment": i + 1, "judge": j["judge"], "kind": j["kind"], "target": j["target"]}
		for _, key := range []string{"phase", "round"} {
			if j[key] != nil {
				source[key] = j[key]
			}
		}
		for _, f := range docs(j["findings"]) {
			entry := doc{"finding": f, "source": source}
			if truth(f["deferred"]) && !truth(f["resolved"]) {
				out["deferred"] = append(docs(out["deferred"]), entry)
			}
			if truth(f["mechanical"]) {
				out["mechanical"] = append(docs(out["mechanical"]), entry)
			}
			if j["judge"] == "dispute" && !truth(f["blocking"]) {
				out["accepted_objections"] = append(docs(out["accepted_objections"]), entry)
			}
		}
	}
	for _, r := range results {
		for _, f := range docs(r["carried"]) {
			sources := []doc{}
			for i, j := range judgments {
				if j["run_id"] != r["run_id"] || j["judge"] != f["judge"] {
					continue
				}
				for _, original := range docs(j["findings"]) {
					if original["id"] == f["id"] && !truth(original["resolved"]) {
						sources = append(sources, doc{"run_id": j["run_id"], "judgment": i + 1, "judge": j["judge"], "kind": j["kind"], "target": j["target"]})
					}
				}
			}
			for _, source := range sources {
				out["carried"] = append(docs(out["carried"]), doc{"finding": f, "source": source})
			}
		}
	}
	return out
}

func commitBacklog(ctx record.ContextData, judgments, results []doc) (string, error) {
	file := filepath.Join(".devkit", "backlog", record.EncodeURIComponent(ctx.Branch)+".json")
	if err := os.MkdirAll(filepath.Join(ctx.Cwd, filepath.Dir(file)), 0755); err != nil {
		return "", err
	}
	if err := save(filepath.Join(ctx.Cwd, file), backlog(judgments, results)); err != nil {
		return "", err
	}
	if _, err := gitx.Run(ctx.Cwd, "add", "--", file); err != nil {
		return "", err
	}
	staged, err := gitx.Run(ctx.Cwd, "diff", "--cached", "--name-only", "--", file)
	if err != nil {
		return "", err
	}
	if staged != "" {
		if _, err = gitx.Run(ctx.Cwd, "commit", "-q", "-m", "chore: record review backlog\n\nArchon-Run: "+ctx.RunID, "--", file); err != nil {
			return "", err
		}
	}
	return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
}
