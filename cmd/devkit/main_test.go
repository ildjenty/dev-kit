package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
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

func TestCommandsAndHookBindings(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		t.Run(string(host), func(t *testing.T) {
			root := checkout(t)
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
			gitCommand(t, root, "add", ".")
			gitCommand(t, root, "commit", "--quiet", "-m", "Initialize devkit")
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
			var status engine.Status
			if err := json.Unmarshal([]byte(command(t, "", "status")), &status); err != nil {
				t.Fatal(err)
			}
			if status.WorkID != started.WorkID {
				t.Fatal("work binding was lost")
			}
			message, _ := json.Marshal(map[string]string{"instruction_id": started.Instruction.ID})
			subagent, _ := json.Marshal(map[string]string{"hook_event_name": "SubagentStop", "session_id": "test-session", "cwd": nested, "agent_id": "planner", "agent_type": "devkit-planner", "last_assistant_message": string(message)})
			command(t, string(subagent), "hook", "SubagentStop")
			resultPath := filepath.Join(root, ".devkit/run/work", started.WorkID, "results", started.Instruction.ID+"-planner.json")
			if _, err := os.Stat(resultPath); err != nil {
				t.Fatal(err)
			}
			denied, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": nested, "agent_id": "judge", "agent_type": "devkit-judge", "tool_name": "apply_patch", "tool_input": map[string]string{}})
			if got := command(t, string(denied), "hook", "PreToolUse"); !strings.Contains(got, `"permissionDecision":"deny"`) {
				t.Fatalf("judge write was not denied: %s", got)
			}
			allowed, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": nested, "agent_id": "judge", "agent_type": "devkit-judge", "tool_name": "Read", "tool_input": map[string]string{"file_path": filepath.Join(root, "devkit.toml")}})
			if got := command(t, string(allowed), "hook", "PreToolUse"); strings.TrimSpace(got) != "{}" {
				t.Fatalf("judge read was not allowed: %s", got)
			}
			command(t, "", "pause")
			var paused engine.Result
			if err := json.Unmarshal([]byte(command(t, "", "next")), &paused); err != nil {
				t.Fatal(err)
			}
			if !paused.Paused || paused.Instruction != nil {
				t.Fatalf("paused next advanced work: %+v", paused)
			}
			command(t, "", "resume")
			if got := command(t, "", "view"); !strings.Contains(got, "view.html") {
				t.Fatalf("missing view path: %s", got)
			}
			command(t, "", "status", "--records")
			command(t, "", "abort")
			var ended engine.Status
			if err := json.Unmarshal([]byte(command(t, "", "status")), &ended); err != nil {
				t.Fatal(err)
			}
			if ended.Position.Status != ledger.WorkAborted {
				t.Fatalf("work did not abort: %+v", ended.Position)
			}
			observation, err := hook.ReadSession(filepath.Join(root, ".devkit/run"), "test-session")
			if err != nil || observation.Host != host {
				t.Fatalf("wrong host observation: %+v, %v", observation, err)
			}
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
			if !strings.Contains(got, "--- .claude/settings.json (before)\n+++ .claude/settings.json (after)\n-"+before) || !strings.Contains(got, `+      "Bash(devkit *)"`) {
				t.Fatalf("missing host settings diff: %s", got)
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
