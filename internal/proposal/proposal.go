package proposal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type Document = map[string]any

func CommitPaths(ctx record.ContextData, paths []string, message string) error {
	if len(paths) == 0 {
		_, err := gitx.Run(ctx.Cwd, "commit", "--allow-empty", "--only", "-m", message)
		return err
	}
	slices.Sort(paths)
	if _, err := gitx.Run(ctx.Cwd, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return err
	}
	_, err := gitx.Run(ctx.Cwd, append([]string{"commit", "--only", "-m", message, "--"}, paths...)...)
	return err
}

func CommittedErrors(ctx record.ContextData, draft any) ([]string, Document, error) {
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, nil, err
	}
	c, _ := p["content"].(map[string]any)
	a, err := json.Marshal(draft)
	if err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(c["draft"])
	if err != nil {
		return nil, nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, nil, err
	}
	status, err := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, nil, err
	}
	if p["run_id"] != ctx.RunID || c["commit"] != head || status != "" || string(a) != string(b) {
		return []string{"checkout changed after the judgment or draft has no committed proposal; run check again"}, p, nil
	}
	return []string{}, p, nil
}

func Save(ctx record.ContextData, p, content Document) error {
	p["content"] = content
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	version, _ := p["version"].(float64)
	if n, ok := p["version"].(int); ok {
		version = float64(n)
	}
	path := filepath.Join(ctx.Dir, "proposals", fmt.Sprintf("%d.json", int(version)))
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".proposal-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(info.Mode().Perm()); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	closed := f.Close()
	if err != nil {
		return err
	}
	if closed != nil {
		return closed
	}
	return os.Rename(f.Name(), path)
}

func Commit(ctx record.ContextData, paths []string, message string, content Document) (Document, error) {
	if err := CommitPaths(ctx, paths, message); err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	content["commit"] = head
	written, err := record.Write(ctx, "proposal", Document{"content": content}, "")
	if err != nil {
		return nil, err
	}
	return written.Entry, nil
}
