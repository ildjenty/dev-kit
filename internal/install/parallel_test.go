package install

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
)

func TestInitInstallsWorkersWithoutJudgeWriteDeny(t *testing.T) {
	ctx := context.Background()
	root := repository(t)
	if _, err := Init(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".claude/agents/devkit-worker.md", ".codex/agents/devkit-worker.toml"} {
		body := string(readFile(t, root, path))
		if !strings.Contains(body, "devkit-worker") || !strings.Contains(body, "schemas/progress.json") || !strings.Contains(body, "absolute worktree path") {
			t.Fatalf("worker definition is incomplete: %s", path)
		}
	}
	settings, err := config.Parse(readFile(t, root, "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	validator, err := workVersionValidator()
	if err != nil {
		t.Fatal(err)
	}
	store := fsstore.New(filepath.Join(root, ".devkit/run/work/test-work"))
	record, err := ledger.NewRecord(store, &ledger.Work{ID: "test-work", TaskRef: "task.txt", Branch: "main", BaseCommit: "base", KitVersion: kit.Version})
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Append(store, record, validator); err != nil {
		t.Fatal(err)
	}
	h := hook.Handler{Root: root, WorkID: "test-work", Validator: validator, ReadConfig: func(string) (config.Config, error) { return settings, nil }}
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		for _, agent := range []string{"devkit-worker", "devkit-judge", "devkit-planner"} {
			for _, tool := range settings.Deny.Judge {
				result, err := h.Process(event.Event{Host: host, Name: event.PreToolUse, SessionID: "test-session", AgentID: "test-agent", AgentType: agent, ToolName: tool})
				if err != nil {
					t.Fatal(err)
				}
				if (result.Decision == event.Deny) != (agent != "devkit-worker") {
					t.Fatalf("incorrect write deny for %s on %s: %+v", agent, host, result)
				}
			}
		}
	}
	if ignored, err := pathIgnored(ctx, root, ".devkit/worktrees/test-unit/file"); err != nil || !ignored {
		t.Fatalf("unit worktrees are not ignored: %v: %v", ignored, err)
	}
}
