package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func TestViewUsesStructureSchemaBeforeDrawing(t *testing.T) {
	root, _ := startWork(t, event.ClaudeCode)
	const valid = `{"modules":[{"id":"core","name":"Core","responsibility":"Compute","path":"internal/core"}],"dependencies":[]}`
	for _, tc := range []struct {
		name, body string
		diagram    bool
	}{
		{"schema conforming", valid, true},
		{"unknown field", strings.TrimSuffix(valid, "}") + `,"notes":"Preserve this important structure context"}`, false},
		{"missing module fields", `{"modules":[{"id":"core"}],"dependencies":[]}`, false},
		{"missing dependencies", `{"modules":[{"id":"core","name":"Core","responsibility":"Compute","path":"internal/core"}]}`, false},
		{"free text", "Structure notes that do not use the optional diagram format.", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, ".devkit/structure.json"), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			name := strings.TrimSpace(command(t, "", "view"))
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "Current dependency structure") != tc.diagram {
				t.Fatalf("structure schema was not enforced: %s", tc.body)
			}
			if !tc.diagram && !strings.Contains(html.UnescapeString(string(data)), tc.body) {
				t.Fatal("view discarded part of the structure body")
			}
		})
	}
	// Historical unstructured documents must also retain their complete body.
	const previous = `{"modules":[{"id":"core"}],"notes":"Historical structure context"}`
	path := filepath.Join(root, ".devkit/structure.json")
	if err := os.WriteFile(path, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", ".devkit/structure.json")
	gitCommand(t, root, "commit", "-qm", "Record free-form structure")
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(strings.TrimSpace(command(t, "", "view")))
	if err != nil || !strings.Contains(html.UnescapeString(string(data)), previous) {
		t.Fatalf("view omitted the historical full body: %v", err)
	}
}

func TestV3InitializationAndV2WorkRefusal(t *testing.T) {
	root, started := startWork(t, event.ClaudeCode)
	data, err := os.ReadFile(filepath.Join(root, "devkit.toml"))
	if err != nil || !bytes.Contains(data, []byte(`kit_version = "v3"`)) {
		t.Fatalf("init did not write v3: %s, %v", data, err)
	}
	store := fsstore.New(filepath.Join(root, ".devkit/run/work", started.WorkID))
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Kind != string(ledger.KindWork) {
			continue
		}
		var work ledger.Work
		if err := json.Unmarshal(record.Payload, &work); err != nil {
			t.Fatal(err)
		}
		if work.KitVersion != "v3" {
			t.Fatalf("start did not record v3: %+v", work)
		}
		work.KitVersion = "v2"
		payload, err := json.Marshal(work)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]any{"id": record.ID, "kind": record.Kind, "payload": json.RawMessage(payload)})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".devkit/run/work", started.WorkID, "records", record.ID+".json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"status"}, strings.NewReader(""), &output); err == nil || !strings.Contains(err.Error(), "versions do not match") {
		t.Fatalf("v2 work passed the compatibility boundary: %v", err)
	}
}

func TestHookReadsGitNormsWithOutsidePin(t *testing.T) {
	root, started := startWork(t, event.ClaudeCode)
	ctx := t.Context()
	w, err := workspace.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := w.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := w.CommitTree(ctx, head)
	if err != nil {
		t.Fatal(err)
	}
	store := fsstore.New(filepath.Join(root, ".devkit/run/work", started.WorkID))
	runID, err := store.NextID()
	if err != nil {
		t.Fatal(err)
	}
	stage := ledger.StageRun{ID: runID, StageID: "implement", PlanVersion: 1, State: ledger.StageWorking}
	if err := store.AppendRecord(runID, string(ledger.KindStageRun), stage); err != nil {
		t.Fatal(err)
	}
	roundID, err := store.NextID()
	if err != nil {
		t.Fatal(err)
	}
	round := ledger.Round{ID: roundID, StageRunID: runID, SessionID: "test-session", StartTarget: form.TargetVersion{Kind: form.TargetTree, Tree: tree}, NormReference: tree + "@" + strings.Repeat("a", 64), Norms: []form.NormReference{}}
	if err := store.AppendRecord(roundID, string(ledger.KindRound), round); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tool string
		input      map[string]string
	}{
		{"configuration", "Read", map[string]string{"file_path": filepath.Join(root, "devkit.toml")}},
		{"agent definition", "Agent", map[string]string{"subagent_type": "devkit-judge", "prompt": "devkit-instruction pinned-judgment"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "test-session", "cwd": root, "tool_name": tc.tool, "tool_input": tc.input})
			if err != nil {
				t.Fatal(err)
			}
			if result := strings.TrimSpace(command(t, string(data), "hook", "PreToolUse")); result != "{}" {
				t.Fatalf("outside pin prevented reading git norms: %s", result)
			}
		})
	}
}
