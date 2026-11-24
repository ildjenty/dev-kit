package judge

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func judgedTree(ctx record.ContextData, g *gitx.Git, commit string) (string, error) {
	state := filepath.Dir(filepath.Dir(ctx.Dir))
	if !filepath.IsAbs(state) {
		state = filepath.Join(ctx.Cwd, state)
	}
	root := filepath.Join(state, "trees")
	slug := record.EncodeURIComponent(ctx.Branch)
	tree := filepath.Join(root, slug)
	markers := filepath.Join(root, ".commits")
	if err := os.MkdirAll(markers, 0755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".commits" || entry.Name() == slug {
			continue
		}
		branch, e := url.PathUnescape(entry.Name())
		if e != nil || record.EncodeURIComponent(branch) != entry.Name() || g.OK("show-ref", "--verify", "--quiet", "refs/heads/"+branch) {
			continue
		}
		if err = os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return "", err
		}
		if err = os.Remove(filepath.Join(markers, entry.Name())); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	marker := filepath.Join(markers, slug)
	data, readErr := os.ReadFile(marker)
	old := strings.TrimSpace(string(data))
	info, statErr := os.Lstat(tree)
	if err = os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if readErr != nil || statErr != nil || !info.IsDir() || old == "" || !g.OK("rev-parse", "--verify", "--end-of-options", old+"^{commit}") || g.UpdateTree(old, commit, tree) != nil {
		if err = os.RemoveAll(tree); err != nil {
			return "", err
		}
		if err = g.ExportTree(commit, tree); err != nil {
			return "", err
		}
	}
	if err = os.WriteFile(marker, []byte(commit+"\n"), 0644); err != nil {
		return "", err
	}
	return tree, nil
}
