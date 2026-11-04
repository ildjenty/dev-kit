package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
)

func TestLinkedWorktreeCommandsAndHooks(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		t.Run(string(host), func(t *testing.T) {
			mainRoot, mainWork := startWork(t, host)
			linkedRoot := filepath.Join(t.TempDir(), "linked")
			gitCommand(t, mainRoot, "worktree", "add", "-b", "linked-session", linkedRoot)
			linkedRoot, err := filepath.EvalSymlinks(linkedRoot)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := filepath.WalkDir(filepath.Join(linkedRoot, ".devkit/run"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return os.Chmod(path, 0o755)
					}
					return nil
				}); err != nil {
					t.Error(err)
				}
			})
			nested := filepath.Join(linkedRoot, "nested")
			if err := os.Mkdir(nested, 0o755); err != nil {
				t.Fatal(err)
			}
			session := "linked-session"
			if host == event.ClaudeCode {
				t.Setenv("CLAUDE_CODE_SESSION_ID", session)
			} else {
				t.Setenv("CODEX_THREAD_ID", session)
			}
			startEvent, err := json.Marshal(map[string]string{"hook_event_name": "SessionStart", "session_id": session, "cwd": nested})
			if err != nil {
				t.Fatal(err)
			}
			// Hook cwd identifies the linked checkout even while the process is in the main one.
			if got := command(t, string(startEvent), "hook", "SessionStart"); strings.Contains(got, "Warning:") {
				t.Fatalf("linked worktree hook failed: %s", got)
			}
			if observation, err := hook.ReadSession(filepath.Join(mainRoot, ".devkit/run"), session); err != nil || observation != nil {
				t.Fatalf("hook wrote into the main checkout: %+v, %v", observation, err)
			}
			t.Chdir(nested)
			command(t, "", "doctor")
			var linkedWork engine.Result
			if err := json.Unmarshal([]byte(command(t, "", "start", "--plan", "docs", "Update linked documentation")), &linkedWork); err != nil {
				t.Fatal(err)
			}
			if linkedWork.WorkID == mainWork.WorkID || linkedWork.Instruction == nil || !strings.HasPrefix(linkedWork.Instruction.Brief, linkedRoot+string(filepath.Separator)) {
				t.Fatalf("linked start shared main work or brief: %+v", linkedWork)
			}
			runtime, err := newEngine(context.Background(), nested, "")
			if err != nil {
				t.Fatal(err)
			}
			status, err := runtime.Status(context.Background())
			if err != nil || status.WorkID != linkedWork.WorkID || runtime.Root() != linkedRoot {
				t.Fatalf("branch resolution escaped linked checkout: %+v, %v", status, err)
			}
			t.Chdir(mainRoot)
			if got := command(t, string(startEvent), "hook", "SessionStart"); !strings.Contains(got, linkedWork.WorkID) || strings.Contains(got, mainWork.WorkID) {
				t.Fatalf("checkpoint identified the wrong checkout: %s", got)
			}
			message, err := json.Marshal(map[string]string{"instruction_id": linkedWork.Instruction.ID})
			if err != nil {
				t.Fatal(err)
			}
			stopEvent, err := json.Marshal(map[string]string{"hook_event_name": "SubagentStop", "session_id": session, "cwd": nested, "agent_id": "planner", "agent_type": "devkit-planner", "last_assistant_message": string(message)})
			if err != nil {
				t.Fatal(err)
			}
			command(t, string(stopEvent), "hook", "SubagentStop")
			result := filepath.Join(".devkit/run/work", linkedWork.WorkID, "results", linkedWork.Instruction.ID+"-planner.json")
			if _, err := os.Stat(filepath.Join(linkedRoot, result)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(mainRoot, result)); !os.IsNotExist(err) {
				t.Fatalf("subagent result escaped linked checkout: %v", err)
			}
		})
	}
}
