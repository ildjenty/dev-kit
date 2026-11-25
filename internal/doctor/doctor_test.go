package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func executable(t *testing.T, root, name, body string) {
	t.Helper()
	testutil.Put(t, root, name, "#!/bin/sh\n"+body+"\n")
	testutil.OK(t, os.Chmod(filepath.Join(root, name), 0700))
}

func environment(t *testing.T, provider string) (cli.Env, string, *bytes.Buffer) {
	t.Helper()
	root, bin := t.TempDir(), t.TempDir()
	testutil.OK(t, fs.WalkDir(kit.Files, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		content := string(testutil.Must(kit.Files.ReadFile(path)))
		if strings.HasSuffix(path, ".yaml") {
			content = strings.ReplaceAll(content, "provider: claude", "provider: "+provider)
		}
		testutil.Put(t, root, path, content)
		return nil
	}))
	testutil.Put(t, root, ".devkit/config.json", fmt.Sprintf(`{"provider":%q,"sessions":["claude","codex"],"skills":{"devkit-optimize":"fit-checks"}}`, provider))
	testutil.Put(t, root, ".agents/skills/devkit-runs/SKILL.md", "skill")
	testutil.Put(t, root, ".agents/skills/devkit-runs/scripts/watch.sh", "watch")
	testutil.Put(t, root, ".agents/skills/devkit-discuss/SKILL.md", "skill")
	for _, folder := range []string{".agents/skills/fit-checks", ".claude/skills/fit-checks"} {
		testutil.Put(t, root, folder+"/SKILL.md", "skill")
	}
	executable(t, bin, "git", `case "$*" in
 '--version') printf 'git version test' ;;
 'rev-parse --show-toplevel') printf '%s' "$PWD" ;;
 'rev-parse --verify HEAD^{commit}') printf 'commit' ;;
 'remote get-url origin') printf 'https://example.invalid/repo.git' ;;
 *) exit 19 ;;
esac`)
	for _, name := range []string{"devkit", "gh", "claude", "codex"} {
		executable(t, bin, name, "printf 'test-version\\n'")
	}
	executable(t, bin, "archon", `case "$*" in
 '--version') printf 'test-version\n' ;;
 'doctor') printf '✓ Installed providers: claude (0.14.0), codex (0.14.0)\n' ;;
 *) exit 19 ;;
esac`)
	executable(t, bin, "plugins/archon-provider-"+provider, "exit 17")
	t.Setenv("PATH", bin)
	t.Setenv("ARCHON_HOME", bin)
	t.Setenv("HOME", bin)
	t.Setenv("TYPESAFE_API_KEY", "")
	var out bytes.Buffer
	calls := testutil.Must(kitinfo.Invocations(kit.Files, ".archon/workflows", false))
	skillCalls := testutil.Must(kitinfo.Invocations(kit.Files, ".claude/skills", true))
	commands := []cli.Command{}
	for _, call := range append(calls, skillCalls...) {
		area, name, _ := strings.Cut(call, " ")
		commands = append(commands, cli.Command{Area: area, Name: name})
	}
	return cli.Env{Registry: cli.New("test", commands), Cwd: root, Stdout: &out, Stderr: &out}, bin, &out
}

func TestPresentCommandsBothProvidersAndOptionalChecks(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			env, bin, out := environment(t, provider)
			testutil.Equal(t, Report(env), true)
			for _, part := range []string{"[required] repository: OK", "[required] archon: OK — test-version", "[required] Archon provider " + provider + ": OK", "listed by archon doctor", "[needed for a part] claude sessions: OK", "[needed for a part] codex sessions: OK", provider + " Jev fallback: OK", "[optional] checks: MISSING — No verification runs; run the fit-checks skill"} {
				testutil.Contains(t, out.String(), part)
			}
			out.Reset()
			t.Setenv("TYPESAFE_API_KEY", "never-print-this-key")
			executable(t, bin, "archon", "printf never-print-this-key")
			testutil.Equal(t, Report(env), true)
			testutil.Contains(t, out.String(), "[optional] TYPESAFE_API_KEY: OK — set; TypeSafe receives changed-file diffs")
			testutil.Equal(t, strings.Contains(out.String(), "never-print-this-key"), false)
			testutil.Equal(t, strings.Contains(out.String(), "Jev fallback"), false)
		})
	}
}

func TestEveryProviderUsedByAnInstalledWorkflowIsRequired(t *testing.T) {
	env, bin, out := environment(t, "claude")
	executable(t, bin, "archon", `case "$*" in
 '--version') printf 'test-version\n' ;;
 'doctor') printf '✓ Installed providers: claude (0.14.0)\n' ;;
 *) exit 19 ;;
esac`)
	testutil.Equal(t, Report(env), true)
	testutil.Equal(t, strings.Contains(out.String(), "Archon provider codex"), false)
	file := filepath.Join(env.Cwd, ".archon/workflows/dev/requirements/requirements.yaml")
	edited := strings.Replace(string(testutil.Must(os.ReadFile(file))), "provider: claude", "provider: codex", 1)
	testutil.OK(t, os.WriteFile(file, []byte(edited), 0644))
	out.Reset()
	testutil.Equal(t, Report(env), false)
	testutil.Contains(t, out.String(), "[required] Archon provider claude: OK")
	testutil.Contains(t, out.String(), "[required] Archon provider codex: MISSING — Provider is not installed; run archon provider add codex, then archon doctor.")
}

func TestProviderInstallationFromDoctor(t *testing.T) {
	env, bin, out := environment(t, "claude")
	testutil.OK(t, os.Remove(filepath.Join(bin, "plugins/archon-provider-claude")))
	executable(t, bin, "archon", `case "$*" in
 '--version') printf 'test-version\n' ;;
 'doctor') printf '✓ Installed providers: claude (0.14.0), codex (0.14.0)\n' ;;
 *) exit 19 ;;
esac`)
	testutil.Equal(t, Report(env), true)
	testutil.Contains(t, out.String(), "[required] Archon provider claude: OK — listed by archon doctor")
}

func TestMissingFailingAndMalformedRequiredItems(t *testing.T) {
	for _, item := range []string{"git", "archon", "devkit", "provider", "config", "concerns", "kit", "commit"} {
		t.Run(item, func(t *testing.T) {
			env, bin, out := environment(t, "claude")
			switch item {
			case "provider":
				testutil.OK(t, os.Remove(filepath.Join(bin, "plugins/archon-provider-claude")))
				executable(t, bin, "archon", "printf 'test-version\\n'")
			case "config":
				testutil.Put(t, env.Cwd, ".devkit/config.json", `{"provider":"other"}`)
			case "concerns":
				testutil.OK(t, os.Remove(filepath.Join(env.Cwd, ".devkit/standards/concerns.json")))
			case "kit":
				testutil.OK(t, os.Remove(filepath.Join(env.Cwd, ".archon/workflows/dev/fix/fix.yaml")))
			case "commit":
				executable(t, bin, "git", `case "$*" in *'HEAD^{commit}'*) exit 1 ;; *'show-toplevel'*) printf '%s' "$PWD" ;; *) printf 'git' ;; esac`)
			default:
				testutil.OK(t, os.Remove(filepath.Join(bin, item)))
			}
			testutil.Equal(t, Report(env), false)
			if item == "provider" {
				testutil.Contains(t, out.String(), "[required] Archon provider claude: MISSING — Provider is not installed; run archon provider add claude, then archon doctor.")
			}
		})
	}
}

func TestPartCommandsDoNotFailDoctorAndNoCommandOutputLeaks(t *testing.T) {
	for _, name := range []string{"gh", "claude", "codex"} {
		for _, body := range []string{"", "printf 'secret diagnostic' >&2; exit 4"} {
			t.Run(name+body, func(t *testing.T) {
				env, bin, out := environment(t, "codex")
				if body == "" {
					testutil.OK(t, os.Remove(filepath.Join(bin, name)))
				} else {
					executable(t, bin, name, body)
				}
				testutil.Equal(t, Report(env), true)
				testutil.Contains(t, out.String(), "[needed for a part] "+name)
				testutil.Contains(t, out.String(), "MISSING")
				testutil.Equal(t, strings.Contains(out.String(), "secret diagnostic"), false)
			})
		}
	}
}

func TestGhAuthenticationCoversIssuesAndPrWithExecutableOverride(t *testing.T) {
	env, bin, out := environment(t, "claude")
	testutil.OK(t, os.Remove(filepath.Join(bin, "gh")))
	path := filepath.Join(bin, "fake-gh")
	t.Setenv("DEVKIT_GH", path)
	executable(t, bin, "fake-gh", `case "$*" in
 '--version') printf 'gh test' ;;
 'auth status') exit 0 ;;
 *) exit 19 ;;
esac`)
	testutil.Equal(t, Report(env), true)
	testutil.Contains(t, out.String(), "gh (issues, pr): OK")
	testutil.Contains(t, out.String(), "gh authentication (issues, pr): OK")
	executable(t, bin, "fake-gh", `case "$*" in
 '--version') printf 'gh test' ;;
 'auth status') printf 'private diagnostic' >&2; exit 4 ;;
 *) exit 19 ;;
esac`)
	out.Reset()
	testutil.Equal(t, Report(env), true)
	testutil.Contains(t, out.String(), "gh authentication (issues, pr): MISSING")
	testutil.Contains(t, out.String(), "Issue operations and pr publishing need authentication; run gh auth login.")
	testutil.Equal(t, strings.Contains(out.String(), "private diagnostic"), false)
}

func TestIssueLabelsArePartSpecificAndApprovalCreatesMissingLabels(t *testing.T) {
	for _, response := range []string{`[]`, `[{"name":"work"},{"name":"questions"}]`, `[{"name":"work"}]`, `invalid JSON`} {
		t.Run(response, func(t *testing.T) {
			env, bin, out := environment(t, "claude")
			testutil.Put(t, env.Cwd, ".devkit/config.json", `{"issues":{"labels":{"unit":"work","topic":"questions"}}}`)
			path := filepath.Join(bin, "fake-gh")
			t.Setenv("DEVKIT_GH", path)
			executable(t, bin, "fake-gh", `case "$1 $2" in
 '--version '| 'auth status') exit 0 ;;
 'label list') printf '%s' '`+response+`' ;;
 *) exit 19 ;;
esac`)
			testutil.Equal(t, Report(env), true)
			for _, name := range []string{"work", "questions"} {
				status := "MISSING"
				if strings.Contains(response, `"`+name+`"`) {
					status = "OK"
				}
				testutil.Contains(t, out.String(), "[needed for a part] issue label "+name+": "+status)
				if status == "MISSING" {
					testutil.Contains(t, out.String(), "approval creates it before creating Issues")
				}
			}
		})
	}
}

func TestShortTimeoutsAndExitCodes(t *testing.T) {
	env, bin, out := environment(t, "claude")
	executable(t, bin, "archon", "exec /bin/sleep 5")
	started := time.Now()
	testutil.Equal(t, report(env, 250*time.Millisecond, 250*time.Millisecond), false)
	if time.Since(started) > 2*time.Second {
		t.Fatal("command timeout was not enforced")
	}
	executable(t, bin, "archon", `case "$*" in
 '--version') printf version ;;
 'doctor') exec /bin/sleep 5 ;;
 *) exit 19 ;;
esac`)
	started = time.Now()
	out.Reset()
	testutil.Equal(t, report(env, time.Second, 250*time.Millisecond), true)
	testutil.Contains(t, out.String(), "[required] Archon provider claude: OK — plugin binary at "+filepath.Join(bin, "plugins/archon-provider-claude"))
	if time.Since(started) > 2*time.Second {
		t.Fatal("provider timeout was not enforced")
	}
	executable(t, bin, "archon", "printf version")
	testutil.OK(t, os.Remove(filepath.Join(bin, "plugins/archon-provider-claude")))
	testutil.Equal(t, cli.New("test", append(Commands(), env.Registry.Commands()...)).Execute([]string{"doctor"}, env), 1)
	out.Reset()
	testutil.Equal(t, cli.New("test", append(Commands(), env.Registry.Commands()...)).Execute([]string{"doctor", "one", "two"}, env), 2)
}

func TestConfiguredChecks(t *testing.T) {
	env, _, out := environment(t, "codex")
	testutil.Put(t, env.Cwd, ".devkit/config.json", `{"provider":"codex","skills":{"devkit-optimize":"fit-checks"},"checks":[{"name":"test","command":"do not execute this","timeout_seconds":1}]}`)
	out.Reset()
	testutil.Equal(t, cli.New("test", append(Commands(), env.Registry.Commands()...)).Execute([]string{"doctor", env.Cwd}, env), 0)
	testutil.Contains(t, out.String(), "[optional] checks: OK — 1 configured")
}

func TestOriginIsPartSpecificAndHomeProviderDiscovery(t *testing.T) {
	env, bin, out := environment(t, "claude")
	executable(t, bin, "git", `case "$*" in
 '--version') printf git ;;
 'rev-parse --show-toplevel') printf '%s' "$PWD" ;;
 'rev-parse --verify HEAD^{commit}') printf commit ;;
 *) exit 1 ;;
esac`)
	testutil.Put(t, bin, ".archon/plugins/archon-provider-claude", string(testutil.Must(os.ReadFile(filepath.Join(bin, "plugins/archon-provider-claude")))))
	testutil.OK(t, os.Chmod(filepath.Join(bin, ".archon/plugins/archon-provider-claude"), 0700))
	t.Setenv("ARCHON_HOME", "")
	executable(t, bin, "archon", "printf version")
	testutil.Equal(t, Report(env), true)
	testutil.Contains(t, out.String(), "[needed for a part] origin (pr): MISSING")
	testutil.Contains(t, out.String(), "plugin binary at "+filepath.Join(bin, ".archon/plugins/archon-provider-claude"))
	testutil.OK(t, os.Remove(filepath.Join(env.Cwd, ".agents/skills/devkit-runs/scripts/watch.sh")))
	out.Reset()
	testutil.Equal(t, Report(env), false)
	testutil.Contains(t, out.String(), "devkit-runs/scripts/watch.sh")
}

func TestProviderDoctorReports(t *testing.T) {
	for _, tt := range []struct {
		name, report, warning string
		installed             bool
	}{
		{"versioned", "✓ Installed providers: codex (0.14.0), claude (0.14.0)", "", true},
		{"unversioned", "✓ Installed providers: codex,  claude  ", "", true},
		{"other provider", "✓ Installed providers: codex (0.14.0)", "", false},
		{"similar ids", "✓ Installed providers: claude-extra (1.0), preclaude (1.0)", "", false},
		{"empty list", "✓ Installed providers:", "", false},
		{"failure mark", "✓ Installed providers: claude (0.14.0)\n  ✗ claude: CLI unavailable", "✗ claude: CLI unavailable", true},
		{"ascii failure mark", "✓ Installed providers: claude\nx claude: CLI unavailable", "x claude: CLI unavailable", true},
		{"warning mark", "✓ Installed providers: claude\n! claude: CLI unavailable", "! claude: CLI unavailable", true},
		{"other diagnostics", "✓ Installed providers: claude\n✗ codex: CLI unavailable\n! claude-extra: unavailable\n✓ claude: spawns OK", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			t.Setenv("PATH", bin)
			t.Setenv("ARCHON_HOME", bin)
			executable(t, bin, "archon", "test \"$*\" = doctor || exit 19\nprintf '%s\\n' '"+tt.report+"'")
			executable(t, bin, "plugins/archon-provider-claude", "exit 17")
			ok, detail := providerInstalled(cli.Env{Cwd: bin}, time.Second, "claude")
			testutil.Equal(t, ok, tt.installed)
			want := "listed by archon doctor"
			if tt.warning != "" {
				want += "; warning: " + tt.warning
			}
			testutil.Equal(t, detail, want)
		})
	}
}

func TestProviderPluginFallback(t *testing.T) {
	for _, customHome := range []bool{true, false} {
		for _, doctor := range []string{"no list", "failure", "timeout"} {
			for _, file := range []string{"executable", "missing", "not executable", "directory"} {
				t.Run(fmt.Sprintf("ARCHON_HOME=%t/%s/%s", customHome, doctor, file), func(t *testing.T) {
					bin, home := t.TempDir(), t.TempDir()
					t.Setenv("PATH", bin)
					t.Setenv("HOME", home)
					t.Setenv("ARCHON_HOME", "")
					pluginHome := filepath.Join(home, ".archon")
					if customHome {
						pluginHome = bin
						t.Setenv("ARCHON_HOME", pluginHome)
					}
					body := "printf 'no provider report\\n'"
					limit := time.Second
					switch doctor {
					case "failure":
						body = "printf '✓ Installed providers: claude (0.14.0)\\n✗ claude: CLI unavailable\\n'; exit 1"
					case "timeout":
						body = "printf '✓ Installed providers: claude (0.14.0)\\n'; exec /bin/sleep 5"
						limit = 250 * time.Millisecond
					}
					executable(t, bin, "archon", "test \"$*\" = doctor || exit 19\n"+body)
					plugin := filepath.Join(pluginHome, "plugins/archon-provider-claude")
					switch file {
					case "executable", "not executable":
						executable(t, pluginHome, "plugins/archon-provider-claude", "printf started > \"$HOME/plugin-started\"; exit 17")
						if file == "not executable" {
							testutil.OK(t, os.Chmod(plugin, 0600))
						}
					case "directory":
						testutil.OK(t, os.MkdirAll(plugin, 0700))
					}
					started := time.Now()
					ok, detail := providerInstalled(cli.Env{Cwd: bin}, limit, "claude")
					if time.Since(started) > time.Second {
						t.Fatal("provider timeout was not enforced")
					}
					testutil.Equal(t, ok, file == "executable")
					want := "plugin binary at " + plugin
					if doctor == "failure" {
						want += "; warning: ✗ claude: CLI unavailable"
					}
					testutil.Equal(t, detail, want)
					_, err := os.Stat(filepath.Join(home, "plugin-started"))
					testutil.Equal(t, os.IsNotExist(err), true)
				})
			}
		}
	}
}

func TestProviderWarningDoesNotFailDoctor(t *testing.T) {
	env, bin, out := environment(t, "claude")
	executable(t, bin, "archon", `case "$*" in
 '--version') printf 'test-version\n' ;;
 'doctor') printf '✓ Installed providers: claude (0.14.0)\n✗ claude: CLI unavailable\n' ;;
 *) exit 19 ;;
esac`)
	testutil.Equal(t, cli.New("test", append(Commands(), env.Registry.Commands()...)).Execute([]string{"doctor"}, env), 0)
	testutil.Contains(t, out.String(), "[required] Archon provider claude: OK — listed by archon doctor; warning: ✗ claude: CLI unavailable")
}

func TestKitCompatibilityAndAdvisoryVersion(t *testing.T) {
	for _, state := range []string{"match", "different", "no-stamp", "workflow", "prompt", "skill", "script", "invalid-stamp"} {
		t.Run(state, func(t *testing.T) {
			env, _, out := environment(t, "claude")
			stamp := kitinfo.Stamp{DevkitVersion: "older", KitDigest: kitinfo.EmbeddedDigest()}
			if state == "different" {
				stamp.KitDigest = "different"
			}
			data := testutil.Must(json.Marshal(stamp))
			testutil.Put(t, env.Cwd, kitinfo.StampPath, string(data))
			switch state {
			case "no-stamp":
				testutil.OK(t, os.Remove(filepath.Join(env.Cwd, kitinfo.StampPath)))
			case "invalid-stamp":
				testutil.Put(t, env.Cwd, kitinfo.StampPath, "invalid")
			case "workflow":
				testutil.Put(t, env.Cwd, kitinfo.Workflows+"/custom/custom.yml", "bash: devkit future start")
			case "prompt":
				testutil.Put(t, env.Cwd, kitinfo.Workflows+"/plan/commands/custom.md", "Run `devkit future start`.")
			case "skill":
				testutil.Put(t, env.Cwd, ".claude/skills/devkit-runs/SKILL.md", "Run `devkit future start`.")
			case "script":
				testutil.Put(t, env.Cwd, ".agents/skills/devkit-runs/scripts/watch.sh", "devkit future start")
			}
			incompatible := state == "workflow" || state == "prompt" || state == "skill" || state == "script"
			testutil.Equal(t, Report(env), !incompatible)
			if incompatible {
				testutil.Contains(t, out.String(), "[required] kit and binary: MISSING — missing commands: devkit future start")
			} else {
				testutil.Contains(t, out.String(), "[required] kit and binary: OK")
			}
			if state == "no-stamp" {
				testutil.Contains(t, out.String(), "[needed for a part] kit version: MISSING — no install stamp")
			} else if state == "different" {
				testutil.Contains(t, out.String(), "[needed for a part] kit version: MISSING — the installed kit was written by devkit older; this binary carries a different kit (test); `devkit init` shows the differences and asks before changing anything")
			} else if state != "invalid-stamp" {
				testutil.Contains(t, out.String(), "[needed for a part] kit version: OK")
			}
		})
	}
}
