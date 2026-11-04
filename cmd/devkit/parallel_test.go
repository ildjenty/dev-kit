package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func TestWorkerHooksUseOwningCheckout(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		t.Run(string(host), func(t *testing.T) {
			root, started := startWork(t, host)
			ctx := context.Background()
			w, err := workspace.Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			workRoot := filepath.Join(root, ".devkit/run/work", started.WorkID)
			validator, err := workValidator(ctx, w, workRoot)
			if err != nil {
				t.Fatal(err)
			}
			id := started.WorkID + "-unit-work-test"
			store := fsstore.New(workRoot)
			record, err := ledger.NewRecord(store, &ledger.Instruction{ID: id, Kind: form.InstructionUnitWork, TargetKey: "unit", SessionID: "test-session", Runner: "devkit-worker", Unit: &plan.Unit{ID: "unit", Description: "Test worker hook routing", Scope: "document.md"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := ledger.Append(store, record, validator); err != nil {
				t.Fatal(err)
			}
			unit := filepath.Join(root, ".devkit/worktrees", id)
			gitCommand(t, root, "worktree", "add", "--detach", unit, "HEAD")
			tool, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": unit, "agent_id": "worker", "agent_type": "devkit-worker", "tool_name": "apply_patch", "tool_input": map[string]string{}})
			if err != nil {
				t.Fatal(err)
			}
			if got := command(t, string(tool), "hook", "PreToolUse"); strings.TrimSpace(got) != "{}" {
				t.Fatalf("worker write was denied: %s", got)
			}
			tool, err = json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": unit, "agent_id": "judge", "agent_type": "devkit-judge", "tool_name": "apply_patch", "tool_input": map[string]string{}})
			if err != nil {
				t.Fatal(err)
			}
			if got := command(t, string(tool), "hook", "PreToolUse"); !strings.Contains(got, `"permissionDecision":"deny"`) {
				t.Fatalf("unit checkout bypassed the judge deny list: %s", got)
			}
			message, err := json.Marshal(form.Progress{InstructionID: id, TerminationKind: form.TerminationCompleted})
			if err != nil {
				t.Fatal(err)
			}
			stop, err := json.Marshal(map[string]string{"hook_event_name": "SubagentStop", "session_id": "test-session", "cwd": unit, "agent_id": "worker", "agent_type": "devkit-worker", "last_assistant_message": string(message)})
			if err != nil {
				t.Fatal(err)
			}
			command(t, string(stop), "hook", "SubagentStop")
			if _, err := os.Stat(filepath.Join(workRoot, "results", id+"-worker.json")); err != nil {
				t.Fatalf("worker result was not collected in its owning checkout: %v", err)
			}
			if _, err := os.Stat(filepath.Join(unit, ".devkit/run")); !os.IsNotExist(err) {
				t.Fatalf("hook created an independent runtime in the worker checkout: %v", err)
			}
		})
	}
}
