package initialize

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRealRegistryInitReportsKitCompatibilityAndPreservesDiagnosticContract(t *testing.T) {
	bin := t.TempDir()
	devkit := filepath.Join(bin, "devkit")
	for _, build := range []struct{ output, source string }{
		{devkit, "../../cmd/devkit"},
		{filepath.Join(bin, "tools"), "testdata/tools.go"},
	} {
		cmd := exec.Command("go", "build", "-buildvcs=false", "-o", build.output, build.source)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", build.source, err, out)
		}
	}
	for _, name := range []string{"git", "archon", "claude", "codex", "gh"} {
		testutil.OK(t, os.Symlink(filepath.Join(bin, "tools"), filepath.Join(bin, name)))
	}
	t.Setenv("PATH", bin)
	t.Setenv("ARCHON_HOME", bin)
	t.Setenv("TYPESAFE_API_KEY", "")
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("DEVKIT_INSTALL_TEST_PROVIDERS", provider)
			invokeCLI := func(args ...string) (string, error) {
				t.Helper()
				cmd := exec.Command(devkit, args...)
				cmd.Dir = root
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			out, err := invokeCLI("init", "--yes", "--provider", provider)
			testutil.OK(t, err)
			testutil.Contains(t, out, "[required] kit and binary: OK")
			testutil.Contains(t, out, "[required] Archon provider "+provider+": OK")
			out, err = invokeCLI("doctor")
			testutil.OK(t, err)
			testutil.Contains(t, out, "[required] kit and binary: OK")
			t.Setenv("DEVKIT_INSTALL_TEST_PROVIDERS", "")
			out, err = invokeCLI("init", "--yes")
			testutil.OK(t, err)
			testutil.Contains(t, out, "[required] kit and binary: OK")
			testutil.Contains(t, out, "[required] Archon provider "+provider+": MISSING")
			out, err = invokeCLI("doctor")
			if err == nil {
				t.Fatal("doctor accepted a missing required provider")
			}
			testutil.Contains(t, out, "[required] Archon provider "+provider+": MISSING")
			testutil.Put(t, root, ".archon/workflows/dev/custom/custom.yml", "nodes: [")
			before := snapshot(t, root)
			out, err = invokeCLI("init", "--no")
			testutil.OK(t, err)
			testutil.Contains(t, out, "[required] workflow providers: MISSING")
			testutil.Equal(t, snapshot(t, root), before)
			testutil.Equal(t, strings.Contains(out, "Wrote:"), false)
		})
	}
}
