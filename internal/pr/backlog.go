package pr

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := saveBacklog(ctx.Cwd, file, value); err != nil {
		return "", err
	}
	files := []string{file}
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

func saveBacklog(cwd, file string, value doc) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(cwd)
	if err != nil {
		return err
	}
	defer root.Close()
	dir := "."
	for _, part := range strings.Split(filepath.Dir(file), string(filepath.Separator)) {
		dir = filepath.Join(dir, part)
		info, err := root.Lstat(dir)
		if os.IsNotExist(err) {
			if err = root.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = root.Lstat(dir)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("backlog ancestor is not a directory: %s", dir)
		}
	}
	mode := os.FileMode(0644)
	info, err := root.Lstat(file)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backlog destination is not a regular file: %s", file)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	temp := filepath.Join(dir, ".backlog-"+rand.Text())
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return root.Rename(temp, file)
}
