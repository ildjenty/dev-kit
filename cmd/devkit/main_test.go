package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/install"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func command(t *testing.T, input string, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	if err := run(context.Background(), args, strings.NewReader(input), &output); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return output.String()
}

func gitCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func checkout(t *testing.T) string {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	gitEnvironment := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(gitEnvironment, "config"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(gitEnvironment, "config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TEMPLATE_DIR", gitEnvironment)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", gitEnvironment)
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "init", "--quiet")
	gitCommand(t, root, "config", "user.name", "Devkit test")
	gitCommand(t, root, "config", "user.email", "devkit@example.test")
	gitCommand(t, root, "config", "commit.gpgsign", "false")
	t.Chdir(root)
	return root
}

func TestVersionOutsideCheckout(t *testing.T) {
	t.Chdir(t.TempDir())
	if got := command(t, "", "version"); got != "devkit "+kit.Version+"\n" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func startWork(t *testing.T, host event.Host) (string, engine.Result) {
	t.Helper()
	root := checkout(t)
	t.Cleanup(func() {
		err := filepath.WalkDir(filepath.Join(root, ".devkit/run/work"), func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return os.Chmod(path, 0o755)
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	if host == event.ClaudeCode {
		t.Setenv("CLAUDE_CODE_SESSION_ID", "test-session")
	} else {
		t.Setenv("CODEX_THREAD_ID", "test-session")
	}
	if got := command(t, "", "init", "--yes"); !strings.Contains(got, "Initialized") {
		t.Fatalf("missing init summary: %s", got)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	startEvent := fmt.Sprintf(`{"hook_event_name":"SessionStart","session_id":"test-session","cwd":%q}`, nested)
	command(t, startEvent, "hook", "SessionStart")
	command(t, "", "doctor")
	var started engine.Result
	if err := json.Unmarshal([]byte(command(t, "", "start", "--plan", "docs", "Update documentation")), &started); err != nil {
		t.Fatal(err)
	}
	if started.Instruction == nil || started.Instruction.Runner != "devkit-planner" {
		t.Fatalf("missing planner: %+v", started)
	}
	if _, err := os.Stat(started.Instruction.Brief); err != nil {
		t.Fatal(err)
	}
	return root, started
}

func TestCommandsAndHookBindings(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		t.Run(string(host), func(t *testing.T) {
			t.Run("session binding", func(t *testing.T) {
				root, started := startWork(t, host)
				var status engine.Status
				if err := json.Unmarshal([]byte(command(t, "", "status")), &status); err != nil {
					t.Fatal(err)
				}
				if status.WorkID != started.WorkID {
					t.Fatal("work binding was lost")
				}
				observation, err := hook.ReadSession(filepath.Join(root, ".devkit/run"), "test-session")
				if err != nil || observation == nil || observation.Host != host {
					t.Fatalf("wrong host observation: %+v, %v", observation, err)
				}
			})
			t.Run("subagent result collection", func(t *testing.T) {
				root, started := startWork(t, host)
				message, _ := json.Marshal(map[string]string{"instruction_id": started.Instruction.ID})
				subagent, _ := json.Marshal(map[string]string{"hook_event_name": "SubagentStop", "session_id": "test-session", "cwd": filepath.Join(root, "nested"), "agent_id": "planner", "agent_type": "devkit-planner", "last_assistant_message": string(message)})
				command(t, string(subagent), "hook", "SubagentStop")
				resultPath := filepath.Join(root, ".devkit/run/work", started.WorkID, "results", started.Instruction.ID+"-planner.json")
				if _, err := os.Stat(resultPath); err != nil {
					t.Fatal(err)
				}
			})
			t.Run("judge tool permissions", func(t *testing.T) {
				root, _ := startWork(t, host)
				denied, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": filepath.Join(root, "nested"), "agent_id": "judge", "agent_type": "devkit-judge", "tool_name": "apply_patch", "tool_input": map[string]string{}})
				if got := command(t, string(denied), "hook", "PreToolUse"); !strings.Contains(got, `"permissionDecision":"deny"`) {
					t.Fatalf("judge write was not denied: %s", got)
				}
				allowed, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": filepath.Join(root, "nested"), "agent_id": "judge", "agent_type": "devkit-judge", "tool_name": "Read", "tool_input": map[string]string{"file_path": filepath.Join(root, "devkit.toml")}})
				if got := command(t, string(allowed), "hook", "PreToolUse"); strings.TrimSpace(got) != "{}" {
					t.Fatalf("judge read was not allowed: %s", got)
				}
			})
			t.Run("pause and resume", func(t *testing.T) {
				startWork(t, host)
				command(t, "", "pause")
				var paused engine.Result
				if err := json.Unmarshal([]byte(command(t, "", "next")), &paused); err != nil {
					t.Fatal(err)
				}
				if !paused.Paused || paused.Instruction != nil {
					t.Fatalf("paused next advanced work: %+v", paused)
				}
				command(t, "", "resume")
				var status engine.Status
				if err := json.Unmarshal([]byte(command(t, "", "status")), &status); err != nil {
					t.Fatal(err)
				}
				if status.Position.Paused {
					t.Fatal("work remained paused after resume")
				}
			})
			t.Run("view and record queries", func(t *testing.T) {
				startWork(t, host)
				if got := command(t, "", "view"); !strings.Contains(got, "view.html") {
					t.Fatalf("missing view path: %s", got)
				}
				command(t, "", "status", "--records")
			})
			t.Run("abort", func(t *testing.T) {
				startWork(t, host)
				command(t, "", "abort")
				var ended engine.Status
				if err := json.Unmarshal([]byte(command(t, "", "status")), &ended); err != nil {
					t.Fatal(err)
				}
				if ended.Position.Status != ledger.WorkAborted {
					t.Fatalf("work did not abort: %+v", ended.Position)
				}
			})
		})
	}
}

func TestInitConfirmation(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    string
		yes      bool
		accepted bool
	}{
		{name: "accept", input: "y\n", accepted: true},
		{name: "accept full answer", input: "YES\n", accepted: true},
		{name: "skip confirmation", yes: true, accepted: true},
		{name: "decline", input: "n\n"},
		{name: "empty answer", input: "\n"},
		{name: "end of input"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := checkout(t)
			settings := filepath.Join(root, ".claude/settings.json")
			if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
				t.Fatal(err)
			}
			const before = "{\"permissions\":{\"allow\":[\"Read\"]}}\n"
			if err := os.WriteFile(settings, []byte(before), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"init"}
			if test.yes {
				args = append(args, "--yes")
			}
			var output bytes.Buffer
			err := run(context.Background(), args, strings.NewReader(test.input), &output)
			if test.accepted && err != nil || !test.accepted && (err == nil || err.Error() != "initialization canceled") {
				t.Fatalf("unexpected confirmation result: %v", err)
			}
			got := output.String()
			diffPath := filepath.Join(root, ".devkit/run/init.diff")
			if len(got) > 4000 || !strings.Contains(got, "Full diff: "+diffPath) {
				t.Fatalf("missing bounded diff summary: %s", got)
			}
			diff, err := os.ReadFile(diffPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(diff), "--- .claude/settings.json (before)\n+++ .claude/settings.json (after)\n-"+before) || !strings.Contains(string(diff), `+      "Bash(devkit *)"`) {
				t.Fatalf("missing host settings diff: %s", diff)
			}
			if strings.Contains(got, "Apply initialization changes? [y/N]") == test.yes {
				t.Fatalf("unexpected confirmation prompt: %s", got)
			}
			after, err := os.ReadFile(settings)
			if err != nil {
				t.Fatal(err)
			}
			if !test.accepted {
				if string(after) != before {
					t.Fatal("declined initialization changed host settings")
				}
				if _, err := os.Stat(filepath.Join(root, ".codex/hooks.json")); !os.IsNotExist(err) {
					t.Fatalf("declined initialization wrote hooks: %v", err)
				}
			} else if got := command(t, "", "init"); strings.Contains(got, "Apply initialization changes?") {
				t.Fatalf("unchanged initialization requested confirmation: %s", got)
			}
		})
	}
}

func TestLongOutputUsesFile(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	value := strings.Repeat("Long output. ", 1000)
	if err := writeOutput(&output, root, "response.json", value); err != nil {
		t.Fatal(err)
	}
	if output.Len() > 1000 || !strings.Contains(output.String(), filepath.Join(root, "response.json")) {
		t.Fatalf("unexpected output: %s", output.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "response.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil || decoded != value {
		t.Fatal("full output was not preserved")
	}
}

func TestPreToolUseReadFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		host event.Host
		path string
		data string
	}{
		{name: "missing configuration", host: event.ClaudeCode, path: "devkit.toml"},
		{name: "invalid configuration", host: event.Codex, path: "devkit.toml", data: "kit_version = ["},
		{name: "missing agent definition", host: event.ClaudeCode, path: ".claude/agents/devkit-planner.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, started := startWork(t, test.host)
			path := filepath.Join(root, test.path)
			var err error
			if test.data == "" {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, []byte(test.data), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			gitCommand(t, root, "add", "-A")
			gitCommand(t, root, "commit", "--quiet", "-m", "Make normative input unavailable")
			tool, toolInput := "Read", map[string]string{"file_path": "devkit.toml"}
			if test.path == ".claude/agents/devkit-planner.md" {
				tool, toolInput = "Agent", map[string]string{"subagent_type": "devkit-planner", "prompt": "devkit-instruction " + started.Instruction.ID}
			}
			data, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": root, "tool_name": tool, "tool_input": toolInput})
			if err != nil {
				t.Fatal(err)
			}
			if got := command(t, string(data), "hook", "PreToolUse"); !strings.Contains(got, `"permissionDecision":"deny"`) || !strings.Contains(got, "Cannot evaluate tool use:") {
				t.Fatalf("unreadable policy was not denied: %s", got)
			}
			pause := fmt.Sprintf(`{"hook_event_name":"PreToolUse","session_id":"test-session","cwd":%q,"tool_name":"Bash","tool_input":{"command":"devkit pause"}}`, root)
			if got := command(t, pause, "hook", "PreToolUse"); !strings.Contains(got, `"permissionDecision":"allow"`) {
				t.Fatalf("pause was not allowed: %s", got)
			}
		})
	}
}

type failingWriter struct {
	writes int
	failAt int
}

func (w *failingWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(data), nil
}

func TestOutputWriteFailures(t *testing.T) {
	long := strings.Repeat("Long output. ", 1000)
	for _, test := range []struct {
		name  string
		value any
		lines int
	}{
		{name: "result", value: engine.Result{Message: long}, lines: 2},
		{name: "status", value: engine.Status{Warning: long}, lines: 2},
		{name: "initialization", value: install.Result{Preserved: []string{long}, Guidance: []string{"Review host trust settings."}}, lines: 3},
		{name: "doctor", value: install.Report{Checks: []install.Check{{Message: long}}}, lines: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			for line := 1; line <= test.lines; line++ {
				output := &failingWriter{failAt: line}
				if err := writeOutput(output, t.TempDir(), "response.json", test.value); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("write failure on line %d was lost: %v", line, err)
				}
			}
		})
	}
}

func TestShortInitDiffUsesOutput(t *testing.T) {
	var output bytes.Buffer
	change := install.Change{Path: ".claude/agents/devkit-test.md", Before: []byte("before\n"), After: []byte("after\n")}
	if err := confirmInit([]install.Change{change}, t.TempDir(), strings.NewReader("y\n"), &output, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "-before\n+after\nApply initialization changes? [y/N]") {
		t.Fatalf("short diff was not shown before confirmation: %s", output.String())
	}
}

func TestViewWarningWriteFailure(t *testing.T) {
	root, _ := startWork(t, event.ClaudeCode)
	gitCommand(t, root, "checkout", "--quiet", "-b", "other-work")
	var status engine.Status
	if err := json.Unmarshal([]byte(command(t, "", "status")), &status); err != nil {
		t.Fatal(err)
	}
	if status.Warning == "" {
		t.Fatal("branch mismatch did not produce a warning")
	}
	output := &failingWriter{failAt: 1}
	if err := run(context.Background(), []string{"view"}, strings.NewReader(""), output); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("warning write failure was lost: %v", err)
	}
}

func TestInstallerPathEntries(t *testing.T) {
	data, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	_, script, ok := strings.Cut(string(data), "on_path=false\n")
	if !ok {
		t.Fatal("installer PATH check is missing")
	}
	script, _, ok = strings.Cut(script, "\nif [ \"$on_path\" != true ]; then")
	if !ok {
		t.Fatal("installer PATH check end is missing")
	}
	script = "bin_dir=$1\non_path=false\n" + script + "\n[ \"$on_path\" = true ]\n"
	root := t.TempDir()
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		path string
		want bool
	}{
		{name: "trailing empty entry", path: "/usr/bin:", want: true},
		{name: "leading empty entry", path: ":/usr/bin", want: true},
		{name: "middle empty entry", path: "/usr/bin::/bin", want: true},
		{name: "empty PATH", path: "", want: true},
		{name: "explicit directory", path: root, want: true},
		{name: "directory absent", path: "/usr/bin:/bin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command("/bin/sh", "-c", script, "install-path-check", root)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+test.path)
			output, err := cmd.CombinedOutput()
			if (err == nil) != test.want {
				t.Fatalf("unexpected PATH result: %v, %s", err, output)
			}
		})
	}
}

func TestInitNoCommitFlag(t *testing.T) {
	root := checkout(t)
	command(t, "", "init", "--yes", "--no-commit")
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD").Run(); err == nil {
		t.Fatal("init --no-commit created a commit")
	}
	if _, err := os.Stat(filepath.Join(root, "devkit.toml")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"doctor"}, strings.NewReader(""), &output); err == nil {
		t.Fatal("doctor accepted initialization without HEAD")
	}
}
