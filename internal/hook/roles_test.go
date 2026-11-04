package hook

import (
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/host/event"
)

func TestRoleKindsEnforceReadOnlyToolsAndLaunches(t *testing.T) {
	h := newHandler(t)
	settings, err := config.Parse([]byte("kit_version='v3'\n[roles.reviewer]\nkind='judge'\nagent='devkit-custom-judge'\n[roles.designer]\nkind='plan'\nagent='devkit-custom-plan'\n[roles.editor]\nkind='work'\nagent='devkit-editor'\n[deny]\njudge=['Write','apply_patch','Edit']"))
	if err != nil {
		t.Fatal(err)
	}
	h.ReadConfig = func(string) (config.Config, error) { return settings, nil }
	for _, agent := range []string{"devkit-custom-judge", "devkit-custom-plan", "devkit-editor"} {
		for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
			for _, tool := range []string{"Read", "Write", "apply_patch"} {
				result := process(t, h, event.Event{Name: event.PreToolUse, Host: host, SessionID: "session", AgentID: "child", AgentType: agent, ToolName: tool})
				want := agent != "devkit-editor" && tool != "Read"
				if (result.Decision == event.Deny) != want {
					t.Fatalf("%s %s %s: %+v", host, agent, tool, result)
				}
			}
		}
	}
	for _, agent := range []string{"devkit-custom-judge", "devkit-custom-plan"} {
		for _, tools := range [][]string{nil, {"Read", "Bash"}, {"Read", "Glob", "Grep"}, {}} {
			h.ReadAgent = func(_ string, _ event.Host, name string) (AgentDefinition, error) {
				return AgentDefinition{Name: name, Tools: tools}, nil
			}
			result := process(t, h, event.Event{Name: event.PreToolUse, Host: event.ClaudeCode, SessionID: "session", ToolName: "Agent", Agent: &event.AgentInvocation{Type: agent, Prompt: "devkit-instruction issued"}})
			want := tools == nil || len(tools) == 2
			if (result.Decision == event.Deny) != want {
				t.Fatalf("read-only launch %s %v: %+v", agent, tools, result)
			}
		}
	}
	unknown := process(t, h, event.Event{Name: event.PreToolUse, Host: event.ClaudeCode, SessionID: "session", ToolName: "Agent", Agent: &event.AgentInvocation{Type: "devkit-unknown", Prompt: "devkit-instruction issued"}})
	if unknown.Decision != event.Deny || !strings.Contains(unknown.Reason, "not registered") {
		t.Fatalf("unknown agent launch was allowed: %+v", unknown)
	}
}

func TestDefaultRolesPreserveDenyDecisions(t *testing.T) {
	h := newHandler(t)
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		for _, agent := range []string{"", "devkit-worker", "devkit-judge", "devkit-planner"} {
			for _, tool := range []string{"Read", "apply_patch", "mcp_write"} {
				result := process(t, h, event.Event{Name: event.PreToolUse, Host: host, SessionID: "session", AgentID: "child", AgentType: agent, ToolName: tool})
				want := (agent == "devkit-judge" || agent == "devkit-planner") && tool != "Read"
				if (result.Decision == event.Deny) != want {
					t.Fatalf("default deny changed for %s %s %s: %+v", host, agent, tool, result)
				}
			}
		}
	}
}

func TestCustomRoleSubagentResultCollection(t *testing.T) {
	h := newHandler(t)
	settings, err := config.Parse([]byte("kit_version='v3'\n[roles.reviewer]\nkind='judge'\nagent='devkit-custom-judge'"))
	if err != nil {
		t.Fatal(err)
	}
	h.ReadConfig = func(string) (config.Config, error) { return settings, nil }
	h.InstructionRunner = func(ledger.Instruction) (string, error) { return "devkit-custom-judge", nil }
	appendPayload(t, h, &ledger.Instruction{ID: "issued", Kind: form.InstructionJudgment, TargetKey: "target", SessionID: "session"})
	process(t, h, event.Event{Name: event.SubagentStop, Host: event.ClaudeCode, SessionID: "session", AgentID: "custom", AgentType: "devkit-custom-judge", LastAssistantMessage: `{"instruction_id":"issued"}`})
	if _, err := os.Stat(filepath.Join(h.workRoot(), "results", "issued-custom.json")); err != nil {
		t.Fatalf("custom role result was not collected: %v", err)
	}
}
