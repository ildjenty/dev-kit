package initialize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestStandardsCollisionsRejectBeforeComparisonQuestionsAndWrites(t *testing.T) {
	for _, settings := range []map[string]string{
		{"standards": ".devkit"},
		{"standards": ".devkit//"},
		{"standards": ".devkit/config.json"},
		{"standards": ".devkit/config.json/nested"},
		{"standards": ".archon"},
		{"standards": ".archon/workflows"},
		{"standards": ".archon//workflows/dev/"},
		{"standards": ".archon/workflows/dev/nested"},
		{"standards": ".archon/config.yaml/nested"},
		{"standards": ".claude/skills"},
		{"standards": ".claude/skills/devkit-runs"},
		{"standards": ".agents/skills/devkit-runs", "sessions": "codex"},
		{"standards": "policy", "decisions": "policy"},
		{"standards": "policy", "decisions": "policy//decisions/"},
		{"standards": "policy/standards", "units": "policy"},
		{"standards": "policy", "topics": "policy/topics"},
		{"standards": "policy/eval/standards", "eval": "policy/eval"},
	} {
		for _, args := range [][]string{{"--yes"}, {"--no"}, {"--dry-run"}, nil} {
			t.Run(fmt.Sprint(settings, args), func(t *testing.T) {
				root := t.TempDir()
				config := map[string]any{}
				for key, value := range settings {
					config[key] = value
				}
				if session, ok := settings["sessions"]; ok {
					config["sessions"] = []string{session}
				}
				data := testutil.Must(json.Marshal(config))
				testutil.Put(t, root, ".devkit/config.json", string(data))
				testutil.Put(t, root, ".devkit/decisions/D-abcdef.md", "existing decision")
				testutil.Put(t, root, ".archon/workflows/dev/custom/custom.yaml", "existing workflow")
				testutil.Put(t, root, "docs/guide.md", "existing document")
				before := snapshot(t, root)
				var out, stderr bytes.Buffer
				input := observingReader(func([]byte) (int, error) {
					t.Fatal("asked a question before rejecting ownership collision")
					return 0, nil
				})
				env := cli.Env{Cwd: root, Stdin: input, Stdout: &out, Stderr: &stderr}
				if args == nil {
					testutil.Fails(t, run(args, env, true), "conflicts with")
				} else {
					testutil.Equal(t, cli.New("test", Commands()).Execute(append([]string{"init"}, args...), env), 1)
					testutil.Contains(t, stderr.String(), "conflicts with")
				}
				testutil.Equal(t, out.String(), "")
				testutil.Equal(t, snapshot(t, root), before)
			})
		}
	}
}

func TestNonConflictingStandardsIncludingDocsAreAllowed(t *testing.T) {
	for _, standards := range []string{".devkit/standards", "docs/standards", "docs", "policy", ".devkit/decisions-extra"} {
		t.Run(standards, func(t *testing.T) {
			root := t.TempDir()
			data := testutil.Must(json.Marshal(map[string]string{"standards": standards}))
			testutil.Put(t, root, ".devkit/config.json", string(data))
			invoke(t, root, "--yes")
			_, err := os.Stat(filepath.Join(root, standards, "authority.md"))
			testutil.OK(t, err)
		})
	}
}
