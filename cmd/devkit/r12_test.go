package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/host/event"
)

func TestWorkerWorktreeCommandsUseParentWork(t *testing.T) {
	root, work := startWork(t, event.ClaudeCode)
	worker := filepath.Join(root, ".devkit/worktrees/worker-instruction")
	gitCommand(t, root, "worktree", "add", "--detach", worker, "HEAD")
	nested := filepath.Join(worker, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if err := os.WriteFile("lesson.json", []byte(`{"content":"Record intermediate evidence on the parent work.","evidence":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	var recorded engine.Result
	if err := json.Unmarshal([]byte(command(t, "", "note", "lesson", "lesson.json")), &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded.WorkID != work.WorkID {
		t.Fatalf("note used the wrong work: %+v", recorded)
	}
	for _, session := range []string{"", sessionID()} {
		runtime, err := newEngine(context.Background(), nested, session)
		if err != nil {
			t.Fatal(err)
		}
		status, err := runtime.Status(context.Background())
		if err != nil || status.WorkID != work.WorkID || runtime.Root() != root {
			t.Fatalf("worker resolution escaped parent: %+v, %v", status, err)
		}
	}
	if records := command(t, "", "status", "--records"); !strings.Contains(records, "Record intermediate evidence") {
		// Large status output is written to the parent runtime directory.
		path := filepath.Join(root, ".devkit/run/work", work.WorkID, "records.json")
		body, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(body), "Record intermediate evidence") {
			t.Fatalf("parent note not visible: %s, %v", records, err)
		}
	}
	if _, err := os.Stat(filepath.Join(worker, ".devkit/run/work")); !os.IsNotExist(err) {
		t.Fatalf("worker created independent work records: %v", err)
	}
}
