package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func TestStartTaskFileAndInstructCommands(t *testing.T) {
	root, _ := startWork(t, event.ClaudeCode)
	command(t, "", "abort")
	inputRoot := t.TempDir()
	taskPath := filepath.Join(inputRoot, "task.txt")
	task := "  Agreed requirements and constraints.\nRejected option: preserve the reason.\nHuman wording: {{verbatim}}\n"
	if err := os.WriteFile(taskPath, []byte(task), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"start"},
		{"start", "--task-file", taskPath, "Extra task"},
		{"start", "--task-file", filepath.Join(inputRoot, "missing.txt")},
		{"start", "--task-file", ""},
	} {
		if err := run(context.Background(), args, strings.NewReader(""), io.Discard); err == nil {
			t.Fatalf("accepted invalid task input: %v", args)
		}
	}
	var started engine.Result
	if err := json.Unmarshal([]byte(command(t, "", "start", "--plan", "docs", "--task-file", taskPath)), &started); err != nil {
		t.Fatal(err)
	}
	workRoot := filepath.Join(root, ".devkit/run/work", started.WorkID)
	stored, err := os.ReadFile(filepath.Join(workRoot, "inbox/task.txt"))
	if err != nil || string(stored) != task {
		t.Fatalf("task file was not stored verbatim: %q, %v", stored, err)
	}
	body, err := os.ReadFile(started.Instruction.Brief)
	if err != nil || !strings.Contains(string(body), task) {
		t.Fatalf("planner omitted the task file text: %v", err)
	}
	instructionPath := filepath.Join(inputRoot, "instruction.json")
	instruction := `{"text":"Keep the new human instruction verbatim.\n","stage_id":"implement","audience":"worker"}`
	if err := os.WriteFile(instructionPath, []byte(instruction), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, "", "instruct", instructionPath)
	w, err := workspace.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	validator, err := workValidator(context.Background(), w, workRoot)
	if err != nil {
		t.Fatal(err)
	}
	records, err := ledger.List(fsstore.New(workRoot), validator)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range records {
		if p, ok := record.Payload.(*ledger.MainInstruction); ok {
			found = p.Text == "Keep the new human instruction verbatim.\n" && p.StageID == "implement" && p.Audience == "worker"
			command(t, "", "status", "--record", record.ID)
		}
	}
	if !found {
		t.Fatal("instruct did not record the instruction in the current work history")
	}
	command(t, "", "status", "--records")
	command(t, "", "view")
	command(t, "", "abort")
	if err := run(context.Background(), []string{"instruct", instructionPath}, strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("instruct accepted input after abort")
	}
}
