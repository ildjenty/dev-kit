package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func TestMalformedFinalMessageIsObserved(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		for _, message := range []string{"Unparseable final message", `{"summary":"Missing instruction identity"}`} {
			t.Run(string(host)+"/"+message, func(t *testing.T) {
				h := newHandler(t)
				appendPayload(t, h, &ledger.Instruction{ID: "issued", Kind: form.InstructionPlanCheck, TargetKey: "target", SessionID: "session"})
				e := event.Event{Host: host, Name: event.SubagentStop, SessionID: "session", AgentID: "child", AgentType: "devkit-judge", LastAssistantMessage: message}
				process(t, h, e)
				data, err := os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-child.json"))
				if err != nil {
					t.Fatal(err)
				}
				var result SubagentResult
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatal(err)
				}
				if result.Failure == "" || result.FinalMessage != message || result.InstructionID != "issued" || result.Runner != e.AgentType || result.AgentID != e.AgentID {
					t.Fatalf("malformed message observation incomplete: %+v", result)
				}
				e.LastAssistantMessage = `{"instruction_id":"issued"}`
				process(t, h, e)
				repeated, err := os.ReadFile(filepath.Join(h.workRoot(), "results", "issued-child.json"))
				if err != nil || string(data) != string(repeated) {
					t.Fatalf("failure observation was replaced: %v", err)
				}
			})
		}
	}
}
