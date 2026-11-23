package testutil

import (
	"encoding/json"
	"github.com/ildjenty/dev-kit/kit"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func Must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
func OK(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func Equal(t testing.TB, got, want any) {
	t.Helper()
	g := Must(json.Marshal(got))
	w := Must(json.Marshal(want))
	var a, b any
	OK(t, json.Unmarshal(g, &a))
	OK(t, json.Unmarshal(w, &b))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("got %s; want %s", g, w)
	}
}
func Contains(t testing.TB, text, part string) {
	t.Helper()
	if !strings.Contains(text, part) {
		t.Fatalf("%q does not contain %q", text, part)
	}
}
func Fails(t testing.TB, err error, part string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	Contains(t, err.Error(), part)
}
func Put(t testing.TB, root, path, text string) {
	t.Helper()
	file := filepath.Join(root, path)
	OK(t, os.MkdirAll(filepath.Dir(file), 0755))
	OK(t, os.WriteFile(file, []byte(text), 0644))
}
func Git(t testing.TB, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func Repo(t testing.TB) string {
	t.Helper()
	repo := t.TempDir()
	Git(t, repo, "init", "-q", "-b", "main")
	Git(t, repo, "commit", "-q", "--allow-empty", "-m", "base")
	Git(t, repo, "checkout", "-q", "-b", "feature/x")
	Git(t, repo, "commit", "-q", "--allow-empty", "-m", "work")
	return repo
}

func Standards(t testing.TB, root, dir string) {
	t.Helper()
	OK(t, fs.WalkDir(kit.Files, ".devkit/standards", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := kit.Files.ReadFile(path)
		if err != nil {
			return err
		}
		Put(t, root, filepath.Join(dir, strings.TrimPrefix(path, ".devkit/standards/")), string(data))
		return nil
	}))
}
