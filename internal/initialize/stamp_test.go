package initialize

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestInstallStampFreshUnchangedYesAndNo(t *testing.T) {
	for _, state := range []string{"fresh", "same", "same-without-stamp", "yes", "no", "no-without-stamp", "dry"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			args := []string{}
			old := "{\"devkit_version\":\"older\",\"kit_digest\":\"old-digest\"}\n"
			if state != "fresh" {
				invoke(t, root)
				testutil.Put(t, root, kitinfo.StampPath, old)
				if state != "same" && state != "same-without-stamp" {
					testutil.Put(t, root, kitinfo.Workflows+"/requirements/requirements.yaml", "hand edit")
					testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, kitinfo.StampPath)))), old)
					testutil.OK(t, os.Remove(filepath.Join(root, kitinfo.Workflows, "plan/plan.yaml")))
				}
			}
			switch state {
			case "same":
				args = []string{"--no"}
			case "yes":
				args = []string{"--yes"}
			case "no", "no-without-stamp":
				args = []string{"--no"}
			case "dry":
				args = []string{"--yes", "--dry-run"}
			}
			if state == "no-without-stamp" || state == "same-without-stamp" {
				testutil.OK(t, os.Remove(filepath.Join(root, kitinfo.StampPath)))
			}
			out := invoke(t, root, args...)
			testutil.Equal(t, strings.Contains(out, "Extra: "+kitinfo.StampPath), false)
			if state == "no-without-stamp" {
				_, err := os.Stat(filepath.Join(root, kitinfo.StampPath))
				testutil.Equal(t, os.IsNotExist(err), true)
			} else if state == "no" || state == "dry" {
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, kitinfo.StampPath)))), old)
			} else {
				stamp := testutil.Must(kitinfo.ReadStamp(filepath.Join(root, kitinfo.Workflows)))
				testutil.Equal(t, stamp, kitinfo.Stamp{DevkitVersion: "test", KitDigest: kitinfo.EmbeddedDigest()})
				if state == "fresh" || state == "same" || state == "same-without-stamp" {
					testutil.Equal(t, strings.Contains(out, "[y/N]"), false)
				}
			}
			if state == "no" || state == "no-without-stamp" {
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, kitinfo.Workflows, "requirements/requirements.yaml")))), "hand edit")
				_, err := os.Stat(filepath.Join(root, kitinfo.Workflows, "plan/plan.yaml"))
				testutil.OK(t, err)
			}
		})
	}
}

func TestInstallStampStaysOldWhenAWorkflowLinkIsDeclined(t *testing.T) {
	root := installed(t)
	old := `{"devkit_version":"older","kit_digest":"old"}`
	testutil.Put(t, root, kitinfo.StampPath, old)
	path := kitinfo.Workflows + "/requirements/requirements.yaml"
	testutil.OK(t, os.Remove(filepath.Join(root, path)))
	target := filepath.Join(t.TempDir(), "workflow.yaml")
	testutil.OK(t, os.WriteFile(target, []byte("hand edit"), 0644))
	testutil.OK(t, os.Symlink(target, filepath.Join(root, path)))
	var out bytes.Buffer
	testutil.OK(t, run(nil, cli.Env{Cwd: root, Stdin: strings.NewReader("n\ny\n"), Stdout: &out}, true))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, kitinfo.StampPath)))), old)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(target))), "hand edit")
}
