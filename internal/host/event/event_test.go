package event

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseAgent(t *testing.T) {
	claude := []byte(`{
		"hook_event_name":"PreToolUse","session_id":"session","cwd":"/repo",
		"agent_id":"caller","agent_type":"devkit-judge","effort":{"level":"high"},
		"tool_name":"Agent","tool_input":{"subagent_type":"devkit-plan","prompt":"request", "model":"model-override","effort":"medium","isolation":"worktree"}
	}`)
	event, err := Parse(ClaudeCode, claude)
	if err != nil {
		t.Fatal(err)
	}
	want := &AgentInvocation{Type: "devkit-plan", Prompt: "request", Model: "model-override", Effort: "medium", Isolation: "worktree"}
	if !reflect.DeepEqual(event.Agent, want) || event.Effort != "high" || event.AgentID != "caller" || event.AgentType != "devkit-judge" {
		t.Fatalf("Claude Code invocation: %+v", event)
	}
	codex := []byte(`{
		"hook_event_name":"PreToolUse","session_id":"session","cwd":"/repo",
		"agent_id":"caller","agent_type":"devkit-judge","model":"observed-model",
		"tool_name":"collaborationspawn_agent","tool_input":{"agent_type":"devkit-plan","fork_turns":"none","task_name":"plan","message":{"encrypted":"opaque"}}
	}`)
	event, err = Parse(Codex, codex)
	if err != nil {
		t.Fatal(err)
	}
	want = &AgentInvocation{Type: "devkit-plan", ForkTurns: "none", TaskName: "plan"}
	if !reflect.DeepEqual(event.Agent, want) || event.Model != "observed-model" || event.AgentType != "devkit-judge" {
		t.Fatalf("Codex invocation: %+v", event)
	}
}

func TestParseObservations(t *testing.T) {
	input := []byte(`{
		"hook_event_name":"SubagentStop","session_id":"session","agent_id":"child",
		"agent_type":"devkit-judge","agent_transcript_path":"/transcript.jsonl",
		"last_assistant_message":"final result","effort":{"level":"high"},"model":"observed-model"
	}`)
	for _, host := range []Host{ClaudeCode, Codex} {
		event, err := Parse(host, input)
		if err != nil {
			t.Fatal(err)
		}
		if event.AgentID != "child" || event.AgentTranscriptPath != "/transcript.jsonl" || event.LastAssistantMessage != "final result" {
			t.Fatalf("stop observation: %+v", event)
		}
		if host == ClaudeCode && (event.Effort != "high" || event.Model != "") {
			t.Fatalf("Claude Code provenance: %+v", event)
		}
		if host == Codex && (event.Model != "observed-model" || event.Effort != "") {
			t.Fatalf("Codex provenance: %+v", event)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"UserPromptSubmit","prompt":"  original text\n"}`))
		if err != nil || event.Prompt != "  original text\n" {
			t.Fatalf("original input: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && go test ./..."}}`))
		if err != nil || event.Command != "git status && go test ./..." {
			t.Fatalf("command: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"SessionStart","source":"compact","transcript_path":"/session.jsonl","cwd":"/repo"}`))
		if err != nil || event.Source != "compact" || event.TranscriptPath != "/session.jsonl" || event.CWD != "/repo" {
			t.Fatalf("start: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"Stop","stop_hook_active":true}`))
		if err != nil || !event.StopHookActive {
			t.Fatalf("stop: %+v, %v", event, err)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name   Name
		result Result
		want   string
	}{
		{Stop, Result{}, `{}`},
		{Stop, Result{Decision: Block, Reason: "run devkit next"}, `{"decision":"block","reason":"run devkit next"}`},
		{Stop, Result{SystemMessage: "Continue?"}, `{"systemMessage":"Continue?"}`},
		{PreToolUse, Result{Decision: Deny, Reason: "forbidden operation"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"forbidden operation"}}`},
		{PreToolUse, Result{Decision: Allow}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`},
		{SessionStart, Result{AdditionalContext: "current state"}, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"current state"}}`},
		{UserPromptSubmit, Result{AdditionalContext: "context"}, `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"context"}}`},
		{SubagentStop, Result{}, `{}`},
	}
	for _, host := range []Host{ClaudeCode, Codex} {
		for _, test := range tests {
			data, err := Format(host, test.name, test.result)
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s: %s, %v", host, test.name, data, err)
			}
		}
	}
}

func TestContextLimit(t *testing.T) {
	for _, host := range []Host{ClaudeCode, Codex} {
		body := strings.Repeat("\U0001F30D", 10001)
		data, err := Format(host, SessionStart, Result{AdditionalContext: body, Summary: "current state summary"})
		if err != nil {
			t.Fatal(err)
		}
		var out output
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		if out.Specific.Context != "current state summary\nRun `devkit status` for the full text" {
			t.Fatalf("guidance when limit is exceeded: %+v", out)
		}
		data, err = Format(host, SessionStart, Result{AdditionalContext: body, Summary: body})
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		if (host == ClaudeCode && utf8.RuneCount(data) > 10000) || (host == Codex && len(data) > 10000) || !utf8.Valid(data) || !strings.HasSuffix(out.Specific.Context, "Run `devkit status` for the full text") {
			t.Fatalf("limit or character boundary: %+v", out)
		}
	}
}

func TestOutputLimit(t *testing.T) {
	for _, host := range []Host{ClaudeCode, Codex} {
		for _, test := range []struct {
			name   Name
			result Result
		}{
			{SessionStart, Result{SystemMessage: strings.Repeat("s", 9000), AdditionalContext: strings.Repeat("c", 9000), Summary: "checkpoint summary"}},
			{PreToolUse, Result{Decision: Deny, SystemMessage: strings.Repeat("s", 9000), Reason: strings.Repeat("r", 9000), AdditionalContext: strings.Repeat("c", 9000), Summary: strings.Repeat("\U0001F30D", 10001)}},
			{Stop, Result{Decision: Block, SystemMessage: strings.Repeat("s", 9000), Reason: strings.Repeat("r", 9000)}},
			{SessionStart, Result{AdditionalContext: strings.Repeat("\"\n<", 2000), Summary: "escaped text summary"}},
		} {
			data, err := Format(host, test.name, test.result)
			if err != nil {
				t.Fatal(err)
			}
			if (host == ClaudeCode && utf8.RuneCount(data) > 10000) || (host == Codex && len(data) > 10000) || !utf8.Valid(data) {
				t.Fatalf("%s output exceeds limit or contains invalid UTF-8: %d bytes", host, len(data))
			}
			var out output
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatal(err)
			}
			if test.result.AdditionalContext != "" && (out.Specific == nil || !strings.HasSuffix(out.Specific.Context, "Run `devkit status` for the full text")) {
				t.Fatalf("%s missing full-text guidance: %+v", host, out)
			}
			if test.result.Decision == Block && out.Decision != "block" {
				t.Fatalf("%s lost block decision: %+v", host, out)
			}
			if test.result.Decision == Deny && (out.Specific == nil || out.Specific.Permission != "deny") {
				t.Fatalf("%s lost deny decision: %+v", host, out)
			}
		}
	}
}

func TestContextLimitBoundary(t *testing.T) {
	const overhead = len(`{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":""}}`)
	for _, host := range []Host{ClaudeCode, Codex} {
		for _, length := range []int{9000, 10000 - overhead, 10001 - overhead} {
			body := strings.Repeat("x", length)
			data, err := Format(host, SessionStart, Result{AdditionalContext: body, Summary: "summary"})
			if err != nil {
				t.Fatal(err)
			}
			var out output
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatal(err)
			}
			want := body
			if length+overhead > 10000 {
				want = "summary\nRun `devkit status` for the full text"
			}
			if out.Specific == nil || out.Specific.Context != want || len(data) > 10000 {
				t.Fatalf("%s unexpected context at %d bytes: %s", host, length, data)
			}
		}
		body := strings.Repeat("\U0001F30D", 3000)
		data, err := Format(host, SessionStart, Result{AdditionalContext: body, Summary: "summary"})
		if err != nil {
			t.Fatal(err)
		}
		var out output
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		want := body
		if host == Codex {
			want = "summary\nRun `devkit status` for the full text"
		}
		if out.Specific == nil || out.Specific.Context != want {
			t.Fatalf("%s unexpected multibyte context: %s", host, data)
		}
	}
}

func TestUnsupportedInputAndOutput(t *testing.T) {
	if _, err := Parse("unknown", []byte(`{}`)); err == nil {
		t.Error("unknown host was accepted")
	}
	for _, input := range []string{`null`, `{`, `{"hook_event_name":"Unknown"}`, `{"hook_event_name":"PreToolUse","tool_input":"invalid"}`} {
		if _, err := Parse(Codex, []byte(input)); err == nil {
			t.Errorf("invalid input was accepted: %s", input)
		}
	}
	if _, err := Format(ClaudeCode, SessionStart, Result{Decision: Block}); err == nil {
		t.Error("block was accepted for SessionStart")
	}
	if _, err := Format(Codex, Stop, Result{Decision: Deny}); err == nil {
		t.Error("deny was accepted for Stop")
	}
}
