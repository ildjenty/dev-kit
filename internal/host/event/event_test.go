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
		"tool_name":"Agent","tool_input":{"subagent_type":"devkit-plan","prompt":"依頼", "model":"model-override","effort":"medium","isolation":"worktree"}
	}`)
	event, err := Parse(ClaudeCode, claude)
	if err != nil {
		t.Fatal(err)
	}
	want := &AgentInvocation{Type: "devkit-plan", Prompt: "依頼", Model: "model-override", Effort: "medium", Isolation: "worktree"}
	if !reflect.DeepEqual(event.Agent, want) || event.Effort != "high" || event.AgentID != "caller" || event.AgentType != "devkit-judge" {
		t.Fatalf("Claude Code の起動: %+v", event)
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
		t.Fatalf("Codex の起動: %+v", event)
	}
}

func TestParseObservations(t *testing.T) {
	input := []byte(`{
		"hook_event_name":"SubagentStop","session_id":"session","agent_id":"child",
		"agent_type":"devkit-judge","agent_transcript_path":"/transcript.jsonl",
		"last_assistant_message":"最終結果","effort":{"level":"high"},"model":"observed-model"
	}`)
	for _, host := range []Host{ClaudeCode, Codex} {
		event, err := Parse(host, input)
		if err != nil {
			t.Fatal(err)
		}
		if event.AgentID != "child" || event.AgentTranscriptPath != "/transcript.jsonl" || event.LastAssistantMessage != "最終結果" {
			t.Fatalf("終了の観測: %+v", event)
		}
		if host == ClaudeCode && (event.Effort != "high" || event.Model != "") {
			t.Fatalf("Claude Code の来歴: %+v", event)
		}
		if host == Codex && (event.Model != "observed-model" || event.Effort != "") {
			t.Fatalf("Codex の来歴: %+v", event)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"UserPromptSubmit","prompt":"  原文\n"}`))
		if err != nil || event.Prompt != "  原文\n" {
			t.Fatalf("入力の原文: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && go test ./..."}}`))
		if err != nil || event.Command != "git status && go test ./..." {
			t.Fatalf("コマンド: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"SessionStart","source":"compact","transcript_path":"/session.jsonl","cwd":"/repo"}`))
		if err != nil || event.Source != "compact" || event.TranscriptPath != "/session.jsonl" || event.CWD != "/repo" {
			t.Fatalf("開始: %+v, %v", event, err)
		}
		event, err = Parse(host, []byte(`{"hook_event_name":"Stop","stop_hook_active":true}`))
		if err != nil || !event.StopHookActive {
			t.Fatalf("停止: %+v, %v", event, err)
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
		{Stop, Result{Decision: Block, Reason: "devkit next を実行する"}, `{"decision":"block","reason":"devkit next を実行する"}`},
		{Stop, Result{SystemMessage: "続行しますか"}, `{"systemMessage":"続行しますか"}`},
		{PreToolUse, Result{Decision: Deny, Reason: "禁止操作"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"禁止操作"}}`},
		{PreToolUse, Result{Decision: Allow}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`},
		{SessionStart, Result{AdditionalContext: "現在地"}, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"現在地"}}`},
		{UserPromptSubmit, Result{AdditionalContext: "文脈"}, `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"文脈"}}`},
		{SubagentStop, Result{}, `{}`},
	}
	for _, host := range []Host{ClaudeCode, Codex} {
		for _, test := range tests {
			data, err := Format(host, test.name, test.result)
			if err != nil || string(data) != test.want {
				t.Errorf("%s %s: %s, %v", host, test.name, data, err)
			}
		}
	}
}

func TestContextLimit(t *testing.T) {
	for _, host := range []Host{ClaudeCode, Codex} {
		body := strings.Repeat("文", 10001)
		data, err := Format(host, SessionStart, Result{AdditionalContext: body, Summary: "現在地の要約"})
		if err != nil {
			t.Fatal(err)
		}
		var out output
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		if out.Specific.Context != "現在地の要約\n`devkit status` で全文" {
			t.Fatalf("上限超過時の案内: %+v", out)
		}
		data, err = Format(host, SessionStart, Result{AdditionalContext: body, Summary: body})
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		if !fits(host, out.Specific.Context) || !utf8.ValidString(out.Specific.Context) || !strings.HasSuffix(out.Specific.Context, "`devkit status` で全文") {
			t.Fatalf("上限または文字境界: %+v", out)
		}
	}
}

func TestUnsupportedInputAndOutput(t *testing.T) {
	if _, err := Parse("unknown", []byte(`{}`)); err == nil {
		t.Error("未知のホストが受理されました")
	}
	for _, input := range []string{`null`, `{`, `{"hook_event_name":"Unknown"}`, `{"hook_event_name":"PreToolUse","tool_input":"invalid"}`} {
		if _, err := Parse(Codex, []byte(input)); err == nil {
			t.Errorf("不正な入力が受理されました: %s", input)
		}
	}
	if _, err := Format(ClaudeCode, SessionStart, Result{Decision: Block}); err == nil {
		t.Error("SessionStart の block が受理されました")
	}
	if _, err := Format(Codex, Stop, Result{Decision: Deny}); err == nil {
		t.Error("Stop の deny が受理されました")
	}
}
