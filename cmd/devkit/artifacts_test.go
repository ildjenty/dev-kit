package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/claudecli"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/codexcli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestCommandsRejectMissingArtifactsBeforeCheckoutAccess(t *testing.T) {
	for _, command := range []string{
		"judge prepare", "judge changes", "judge route", "judge launch", "judge completion", "judge collect", "judge finish",
		"pr answers", "pr facts", "pr assemble", "pr judgment", "pr state", "pr open", "pr finish",
		"sync start", "sync continue", "sync state", "sync finish",
	} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			for _, folder := range []string{"judge", "pr", "sync"} {
				testutil.Put(t, root, folder+"/target.json", "stale input")
			}
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr, LookupEnv: func(string) (string, bool) { return "", false }}
			testutil.Fails(t, newRegistry().Run(strings.Fields(command), env), "ARTIFACTS_DIR must be set")
			testutil.Equal(t, out.Len(), 0)
			for _, folder := range []string{"judge", "pr", "sync"} {
				entries := testutil.Must(os.ReadDir(filepath.Join(root, folder)))
				testutil.Equal(t, len(entries), 1)
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, folder, "target.json")))), "stale input")
			}
		})
	}
}

func TestProviderCLIsRequireArtifactAndPackFolders(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, missing := range []string{"ARTIFACTS_DIR", "PACK_DIR"} {
			t.Run(provider+"/"+missing, func(t *testing.T) {
				root := t.TempDir()
				testutil.Put(t, root, "prompt.md", "stale prompt")
				values := map[string]string{"ARTIFACTS_DIR": "artifacts", "PACK_DIR": "."}
				delete(values, missing)
				var out, stderr bytes.Buffer
				env := cli.Env{Cwd: root, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := values[key]; return v, ok }}
				run := claudecli.Run
				if provider == "codex" {
					run = codexcli.Run
				}
				_, err := run(env, "judge", "prompt.md", "{}", time.Second)
				testutil.Fails(t, err, missing+" must be set")
				testutil.Equal(t, len(testutil.Must(os.ReadDir(root))), 1)
				testutil.Equal(t, stderr.Len(), 0)
			})
		}
	}
}
