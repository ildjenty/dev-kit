package pr

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/unitdone"
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
	runs, err := record.List(ctx.Dir, "run")
	if err != nil {
		return "", err
	}
	value := backlog(judgments, results)
	reviews := []doc{}
	for _, run := range runs {
		review := object(run["review"])
		if run["workflow"] == "fix" && review != nil {
			reviews = append(reviews, doc{"finding": doc{"id": text(run["run_id"]) + "/review", "text": review["text"]}, "source": doc{"kind": "review", "run_id": run["run_id"], "target": doc{"commit": review["commit"]}}})
		}
	}
	if len(reviews) > 0 {
		value["review"] = reviews
	}
	if err := save(filepath.Join(ctx.Cwd, file), value); err != nil {
		return "", err
	}
	files := []string{file}
	unit, err := unitdone.RequestUnit(ctx.Dir)
	if err != nil {
		return "", err
	}
	if unit != "" {
		c, err := config.Load(ctx.Cwd)
		if err != nil {
			return "", err
		}
		marker := unitdone.Marker{Unit: unit, Branch: ctx.Branch}
		data, err := json.Marshal(marker)
		if err != nil {
			return "", err
		}
		if _, err = unitdone.Parse(data); err != nil {
			return "", err
		}
		path := filepath.Join(c.Units, unit+".done.json")
		exists, err := unitdone.Exists(ctx.Cwd, c.Units, unit)
		if err != nil {
			return "", err
		}
		if !exists {
			if err = os.MkdirAll(filepath.Join(ctx.Cwd, c.Units), 0755); err != nil {
				return "", err
			}
			if err = os.WriteFile(filepath.Join(ctx.Cwd, path), append(data, '\n'), 0644); err != nil {
				return "", err
			}
		}
		files = append(files, path)
	}
	if _, err := gitx.Run(ctx.Cwd, append([]string{"add", "--"}, files...)...); err != nil {
		return "", err
	}
	staged, err := gitx.Run(ctx.Cwd, append([]string{"diff", "--cached", "--name-only", "--"}, files...)...)
	if err != nil {
		return "", err
	}
	if staged != "" {
		if _, err = gitx.Run(ctx.Cwd, append([]string{"commit", "-q", "-m", "chore: record review backlog\n\nArchon-Run: " + ctx.RunID, "--"}, files...)...); err != nil {
			return "", err
		}
	}
	return gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
}
