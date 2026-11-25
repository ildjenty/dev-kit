package status

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestStateDirRejectsInvalidProjectsAndUnresolvablePaths(t *testing.T) {
	root, bin := t.TempDir(), t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	env := cli.Env{Cwd: root, Environ: os.Environ(), LookupEnv: func(string) (string, bool) { return "", false }}
	for _, body := range []string{"{}", "null", `{"projects":null}`, `{"projects":{}}`, `{"projects":[null]}`, `{"projects":[{}]}`, `{"projects":[{"key":"x"}]}`} {
		testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '"+body+"'\n")
		testutil.OK(t, os.Chmod(filepath.Join(bin, "archon"), 0755))
		dir, why := StateDir(env, root)
		testutil.Equal(t, dir, "")
		testutil.Contains(t, why, "Could not read")
		testutil.Contains(t, why, "projects")
	}
	missing := filepath.Join(root, "deleted-project")
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '{\"projects\":[{\"key\":\"old\",\"default_cwd\":\""+missing+"\"},{\"key\":\"current\",\"default_cwd\":\""+root+"\"}]}'\n")
	dir, why := StateDir(env, root)
	testutil.Contains(t, dir, "workspaces/current/state")
	testutil.Contains(t, why, missing)
	testutil.Contains(t, why, "no such file")
	loop := filepath.Join(root, "loop")
	testutil.OK(t, os.Symlink("loop", loop))
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '{\"projects\":[{\"key\":\"x\",\"default_cwd\":\""+loop+"\"}]}'\n")
	dir, why = StateDir(env, root)
	testutil.Equal(t, dir, "")
	testutil.Contains(t, why, loop)
	testutil.Contains(t, why, "resolve")
	dir, why = StateDir(env, loop)
	testutil.Equal(t, dir, "")
	testutil.Contains(t, why, loop)
	testutil.Contains(t, why, "resolve")
}

func TestReadRejectsMalformedBranchEncoding(t *testing.T) {
	records := t.TempDir()
	path := filepath.Join(records, "bad%XX")
	testutil.OK(t, os.Mkdir(path, 0755))
	_, err := Read(records, "", 3)
	testutil.Fails(t, err, path)
	testutil.Fails(t, err, "escape")
}

type failingOutput struct {
	calls, failAt int
	err           error
}

func (w *failingOutput) Write(b []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, w.err
	}
	return len(b), nil
}

func TestStatusKeepsFirstOutputError(t *testing.T) {
	failure := errors.New("output unavailable")
	for _, at := range []int{1, 2, 3} {
		w := &failingOutput{failAt: at, err: failure}
		err := show(cli.Env{Stdout: w}, "state", "", []Branch{{Branch: "x", Errors: []string{"answer unreadable"}, Next: "repair"}})
		if !errors.Is(err, failure) {
			t.Fatalf("write %d: got %v", at, err)
		}
	}
}
