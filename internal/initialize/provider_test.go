package initialize

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/doctor"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestProviderRenderingAndSessions(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			out := invoke(t, root, "--provider", provider, "--sessions", "claude,codex", "--skill", "devkit-runs=run-it")
			testutil.Contains(t, out, "[required] archon:")
			settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
			testutil.Equal(t, settings.Provider, provider)
			testutil.Equal(t, settings.Sessions, []string{"claude", "codex"})
			judging := string(testutil.Must(os.ReadFile(filepath.Join(root, ".archon/workflows/dev/.shared/judge/judging.md"))))
			for _, text := range []string{
				"Read `$ARTIFACTS_DIR/judge/launched.json` and `concerns.json` in `criteria_folder`",
				"only when the violated principle belongs to your quality, or its owning quality is absent from `launched`",
				"Do not apply `every_quality`.",
				"When the owner was launched, leave that failure to its judge.",
				"A consequence that violates your own quality's principle remains yours",
				"a missing test for the behaviour belongs to test",
				"a changed caller contract belongs to contract",
			} {
				testutil.Contains(t, judging, text)
			}
			quality := string(testutil.Must(os.ReadFile(filepath.Join(root, ".archon/workflows/dev/judge/commands/quality.md"))))
			testutil.Contains(t, quality, "$PACK_DIR/.shared/judge/judging.md")
			for _, session := range settings.Sessions {
				folder := skillFolder(session)
				skill := string(testutil.Must(os.ReadFile(filepath.Join(root, folder, "run-it/SKILL.md"))))
				testutil.Contains(t, skill, folder+"run-it/scripts/watch.sh")
				testutil.Equal(t, strings.Contains(skill, skillFolder(map[string]string{"claude": "codex", "codex": "claude"}[session])), false)
			}
			count := 0
			testutil.OK(t, filepath.WalkDir(filepath.Join(root, ".archon/workflows"), func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() || !strings.HasSuffix(path, ".yaml") || strings.Contains(path, "/fixtures/") {
					return err
				}
				var document any
				if err := yaml.Unmarshal(testutil.Must(os.ReadFile(path)), &document); err != nil {
					return err
				}
				var visit func(any)
				visit = func(value any) {
					switch n := value.(type) {
					case map[string]any:
						if _, ok := n["provider"]; ok {
							count++
							testutil.Equal(t, n["provider"], provider)
							if provider == "codex" {
								for _, key := range []string{"allowed_tools", "denied_tools", "settingSources"} {
									if _, ok := n[key]; ok {
										t.Errorf("%s retained %s", path, key)
									}
								}
							}
							if strings.HasPrefix(n["id"].(string), "judge") || n["command"] == "quality" {
								testutil.Equal(t, n["mutates_checkout"], false)
							}
						}
						for _, child := range n {
							visit(child)
						}
					case []any:
						for _, child := range n {
							visit(child)
						}
					}
				}
				visit(document)
				return nil
			}))
			if count == 0 {
				t.Fatal("no rendered AI nodes")
			}
			testutil.Equal(t, strings.Split(invoke(t, root), "[required]")[0], "")
		})
	}
}

func TestProviderSessionsFlagsPersistWhenConfigReplacementDeclined(t *testing.T) {
	root := installed(t)
	before := "{\n  \"language\" : \"fr\",\n  \"checks\": []\n}\n"
	testutil.Put(t, root, ".devkit/config.json", before)
	out := invoke(t, root, "--no", "--provider=codex", "--sessions=claude,codex")
	testutil.Contains(t, out, ".archon/workflows/dev: replace")
	testutil.Contains(t, out, "Left alone: .archon/workflows/dev/")
	raw := testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))
	testutil.Contains(t, string(raw), `"language" : "fr"`)
	settings := testutil.Must(config.Parse(raw))
	testutil.Equal(t, settings.Provider, "codex")
	testutil.Equal(t, settings.Sessions, []string{"claude", "codex"})
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(root, ".archon/workflows/dev/fix/fix.yaml")))), "provider: claude")
}

func TestSessionDefaultAndPerLocationDeletionOwnership(t *testing.T) {
	root := t.TempDir()
	invoke(t, root, "--provider", "codex")
	settings := testutil.Must(config.Parse(testutil.Must(os.ReadFile(filepath.Join(root, ".devkit/config.json")))))
	testutil.Equal(t, settings.Sessions, []string{"codex"})
	_, err := os.Stat(filepath.Join(root, ".claude/skills"))
	testutil.Equal(t, os.IsNotExist(err), true)
	invoke(t, root, "--yes", "--sessions", "claude,codex")
	testutil.Put(t, root, ".agents/skills/devkit-runs/SKILL.md", "my own skill")
	out := invoke(t, root, "--yes", "--skill", "devkit-runs=renamed")
	testutil.Contains(t, out, "Deleted: .claude/skills/devkit-runs/SKILL.md")
	testutil.Contains(t, out, "Left alone: .agents/skills/devkit-runs holds files devkit did not install")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(root, ".agents/skills/devkit-runs/SKILL.md")))), "my own skill")
	invoke(t, root, "--yes", "--sessions", "codex")
	_, err = os.Stat(filepath.Join(root, ".claude/skills/renamed"))
	testutil.Equal(t, os.IsNotExist(err), true)
}

func TestInitDoctorReportIsSharedAndNeverFailsInit(t *testing.T) {
	root := installed(t)
	out := invoke(t, root)
	var report bytes.Buffer
	testutil.Equal(t, doctor.Report(cli.Env{Registry: cli.New("test", Commands()), Cwd: root, Stdout: &report}), false)
	testutil.Equal(t, out, report.String())
}

func TestInvalidProviderSessionFlagsWriteNothing(t *testing.T) {
	for _, args := range [][]string{{"--provider"}, {"--provider", "other"}, {"--sessions"}, {"--sessions", ""}, {"--sessions", "codex,codex"}, {"--sessions", "claude,other"}} {
		root := t.TempDir()
		var out bytes.Buffer
		if err := run(args, cli.Env{Cwd: root, Stdout: &out}, false); err == nil {
			t.Errorf("accepted %v", args)
		}
		testutil.Equal(t, snapshot(t, root), map[string]string{".": "directory"})
	}
}
