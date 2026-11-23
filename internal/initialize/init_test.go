package initialize

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func TestInitOwnershipAndForce(t *testing.T) {
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	r := cli.New("test", Commands())
	env := cli.Env{Cwd: dir, Stdout: &out, Stderr: &stderr}
	testutil.Equal(t, r.Execute([]string{"init", "repo"}, env), 0)
	target := filepath.Join(dir, "repo")
	_, err := os.Stat(filepath.Join(target, "docs"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.OK(t, filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == "executed" || entry.Name() == "executes-the-go-main-path.stubs.yaml" {
			t.Errorf("source fixture installed: %s", path)
		}
		return nil
	}))
	paths := []string{".devkit/config.json", ".devkit/standards/authority.md", ".archon/config.yaml", ".archon/workflows/dev/requirements/requirements.yaml", ".claude/skills/dev-discuss/SKILL.md", ".claude/skills/dev-runs/SKILL.md"}
	for _, path := range paths {
		_, err := kit.Files.ReadFile(path)
		testutil.OK(t, err)
		testutil.Put(t, target, path, "custom")
	}
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"init", "repo"}, env), 0)
	for i, path := range paths {
		data := string(testutil.Must(os.ReadFile(filepath.Join(target, path))))
		if i < 3 {
			testutil.Equal(t, data, "custom")
			testutil.Contains(t, out.String(), "Left alone: "+path)
		} else {
			testutil.Equal(t, data, string(testutil.Must(kit.Files.ReadFile(path))))
			testutil.Contains(t, out.String(), "Wrote: "+path)
		}
	}
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"init", "--force", "repo"}, env), 0)
	for _, path := range paths {
		testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(target, path)))), string(testutil.Must(kit.Files.ReadFile(path))))
	}
	testutil.Equal(t, strings.Contains(out.String(), "Left alone:"), false)
	_, err = os.Stat(filepath.Join(target, "embed.go"))
	testutil.Equal(t, os.IsNotExist(err), true)
}
func TestEveryKitFileIsEmbeddedExceptGoSource(t *testing.T) {
	diskRoot := filepath.Join("..", "..", "kit")
	disk := []string{}
	testutil.OK(t, filepath.WalkDir(diskRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() != "embed.go" {
			disk = append(disk, testutil.Must(filepath.Rel(diskRoot, path)))
		}
		return nil
	}))
	embedded := []string{}
	testutil.OK(t, fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			embedded = append(embedded, path)
		}
		return nil
	}))
	testutil.Equal(t, embedded, disk)
}
