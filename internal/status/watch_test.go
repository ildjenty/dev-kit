package status

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestWatchReportsRetrievalFailuresAndMissingStatus(t *testing.T) {
	for _, tc := range []struct{ name, script, cause string }{
		{"failure", "echo 'run not found' >&2; exit 4", "run not found"},
		{"partial status", "echo 'Status: completed'; echo 'storage failure' >&2; exit 5", "storage failure"},
		{"missing status", "echo 'Changed output'; echo 'diagnostic' >&2", "no Status field\ndiagnostic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testutil.Put(t, root, "archon", "#!/bin/sh\n"+tc.script+"\n")
			testutil.OK(t, os.Chmod(filepath.Join(root, "archon"), 0700))
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			var out bytes.Buffer
			err := watch([]string{"run-42"}, cli.Env{Cwd: root, Stdout: &out})
			testutil.Fails(t, err, "run run-42")
			testutil.Fails(t, err, tc.cause)
			testutil.Equal(t, out.Len(), 0)
		})
	}
}

func TestWatchEndsWhenEveryRunEnds(t *testing.T) {
	root := t.TempDir()
	testutil.Put(t, root, "archon", "#!/bin/sh\nprintf 'Status: %s\\n' \"$3\"\n")
	testutil.OK(t, os.Chmod(filepath.Join(root, "archon"), 0700))
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	var out bytes.Buffer
	testutil.OK(t, cli.New("test", Commands()).Run([]string{"watch", "completed", "failed", "cancelled", "abandoned"}, cli.Env{Cwd: root, Stdout: &out}))
	testutil.Equal(t, out.String(), "status: completed=completed failed=failed cancelled=cancelled abandoned=abandoned\n")
	testutil.Fails(t, watch(nil, cli.Env{}), "Usage:")
}
