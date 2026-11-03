package event

import (
	"encoding/json"
	"fmt"
)

// Host identifies the host that sent the event.
type Host string

const (
	ClaudeCode Host = "claude-code"
	Codex      Host = "codex"
)

// Name identifies an event handled by devkit.
type Name string

const (
	SessionStart     Name = "SessionStart"
	Stop             Name = "Stop"
	PreToolUse       Name = "PreToolUse"
	SubagentStop     Name = "SubagentStop"
	UserPromptSubmit Name = "UserPromptSubmit"
)

// Event is a common representation of events observable from both hosts.
// Effort holds only values reported by Claude Code; Model holds only values reported by Codex.
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

// AgentInvocation holds observable arguments for a subagent invocation.
// It does not include the encrypted Codex message.
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

// Parse converts host JSON and rejects unknown hosts and events.
// Unknown input fields are ignored to allow host extensions.
func Parse(host Host, data []byte) (Event, error) {
	if err := validHost(host); err != nil {
		return Event{}, err
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return Event{}, fmt.Errorf("%s event input: %w", host, err)
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
			return Event{}, fmt.Errorf("%s effort: %w", host, err)
		}
		e.Effort = observed.Effort.Level
	} else {
		var observed struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(data, &observed); err != nil {
			return Event{}, fmt.Errorf("%s model: %w", host, err)
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
		return Event{}, fmt.Errorf("%s tool input: %w", host, err)
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
			return Event{}, fmt.Errorf("Claude Code Agent input: %w", err)
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
			return Event{}, fmt.Errorf("Codex spawn_agent input: %w", err)
		}
		e.Agent = &AgentInvocation{Type: args.Type, ForkTurns: args.ForkTurns, TaskName: args.TaskName}
	}
	return e, nil
}

func validHost(host Host) error {
	if host != ClaudeCode && host != Codex {
		return fmt.Errorf("unsupported host: %q", host)
	}
	return nil
}

func validName(name Name) error {
	switch name {
	case SessionStart, Stop, PreToolUse, SubagentStop, UserPromptSubmit:
		return nil
	default:
		return fmt.Errorf("unsupported event: %q", name)
	}
}
