package event

import (
	"encoding/json"
	"fmt"
)

// Host はイベントを送ったホストを表す。
type Host string

const (
	ClaudeCode Host = "claude-code"
	Codex      Host = "codex"
)

// Name は devkit が処理するイベント名を表す。
type Name string

const (
	SessionStart     Name = "SessionStart"
	Stop             Name = "Stop"
	PreToolUse       Name = "PreToolUse"
	SubagentStop     Name = "SubagentStop"
	UserPromptSubmit Name = "UserPromptSubmit"
)

// Event は両ホストから観測できるイベントの共通表現である。
// Effort は Claude Code、Model は Codex が報告した値だけを持つ。
type Event struct {
	Host                 Host
	Name                 Name
	SessionID            string
	TranscriptPath       string
	CWD                  string
	Source               string
	Prompt               string
	AgentID              string
	AgentType            string
	ToolName             string
	Command              string
	Agent                *AgentInvocation
	StopHookActive       bool
	LastAssistantMessage string
	AgentTranscriptPath  string
	Effort               string
	Model                string
}

// AgentInvocation はサブエージェント起動で観測できる引数を表す。
// Codex の暗号化された message は含めない。
type AgentInvocation struct {
	Type      string
	Prompt    string
	Model     string
	Effort    string
	Isolation string
	ForkTurns string
	TaskName  string
}

type input struct {
	Name                 Name            `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	TranscriptPath       string          `json:"transcript_path"`
	CWD                  string          `json:"cwd"`
	Source               string          `json:"source"`
	Prompt               string          `json:"prompt"`
	AgentID              string          `json:"agent_id"`
	AgentType            string          `json:"agent_type"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	StopHookActive       bool            `json:"stop_hook_active"`
	LastAssistantMessage string          `json:"last_assistant_message"`
	AgentTranscriptPath  string          `json:"agent_transcript_path"`
}

// Parse はホストの JSON を変換する。未知のホストとイベントはエラーにする。
// 未知の入力項目はホストの拡張を妨げないよう読み飛ばす。
func Parse(host Host, data []byte) (Event, error) {
	if err := validHost(host); err != nil {
		return Event{}, err
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return Event{}, fmt.Errorf("%s のイベント入力: %w", host, err)
	}
	if err := validName(in.Name); err != nil {
		return Event{}, err
	}
	e := Event{
		Host: host, Name: in.Name, SessionID: in.SessionID,
		TranscriptPath: in.TranscriptPath, CWD: in.CWD, Source: in.Source,
		Prompt: in.Prompt, AgentID: in.AgentID, AgentType: in.AgentType,
		ToolName: in.ToolName, StopHookActive: in.StopHookActive,
		LastAssistantMessage: in.LastAssistantMessage, AgentTranscriptPath: in.AgentTranscriptPath,
	}
	if host == ClaudeCode {
		var observed struct {
			Effort struct {
				Level string `json:"level"`
			} `json:"effort"`
		}
		if err := json.Unmarshal(data, &observed); err != nil {
			return Event{}, fmt.Errorf("%s の effort: %w", host, err)
		}
		e.Effort = observed.Effort.Level
	} else {
		var observed struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(data, &observed); err != nil {
			return Event{}, fmt.Errorf("%s の model: %w", host, err)
		}
		e.Model = observed.Model
	}
	if in.Name != PreToolUse || len(in.ToolInput) == 0 {
		return e, nil
	}
	var tool struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(in.ToolInput, &tool); err != nil {
		return Event{}, fmt.Errorf("%s のツール入力: %w", host, err)
	}
	e.Command = tool.Command
	if host == ClaudeCode && in.ToolName == "Agent" {
		var args struct {
			Type      string `json:"subagent_type"`
			Prompt    string `json:"prompt"`
			Model     string `json:"model"`
			Effort    string `json:"effort"`
			Isolation string `json:"isolation"`
		}
		if err := json.Unmarshal(in.ToolInput, &args); err != nil {
			return Event{}, fmt.Errorf("Claude Code の Agent 入力: %w", err)
		}
		e.Agent = &AgentInvocation{
			Type: args.Type, Prompt: args.Prompt, Model: args.Model,
			Effort: args.Effort, Isolation: args.Isolation,
		}
	} else if host == Codex && in.ToolName == "collaborationspawn_agent" {
		var args struct {
			Type      string `json:"agent_type"`
			ForkTurns string `json:"fork_turns"`
			TaskName  string `json:"task_name"`
		}
		if err := json.Unmarshal(in.ToolInput, &args); err != nil {
			return Event{}, fmt.Errorf("Codex の spawn_agent 入力: %w", err)
		}
		e.Agent = &AgentInvocation{Type: args.Type, ForkTurns: args.ForkTurns, TaskName: args.TaskName}
	}
	return e, nil
}

func validHost(host Host) error {
	if host != ClaudeCode && host != Codex {
		return fmt.Errorf("未対応のホストです: %q", host)
	}
	return nil
}

func validName(name Name) error {
	switch name {
	case SessionStart, Stop, PreToolUse, SubagentStop, UserPromptSubmit:
		return nil
	default:
		return fmt.Errorf("未対応のイベントです: %q", name)
	}
}
