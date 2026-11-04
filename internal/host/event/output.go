package event

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Decision is the hook's decision about stopping or tool use. An empty value does not intervene.
type Decision string

const (
	Allow Decision = "allow"
	Block Decision = "block"
	Deny  Decision = "deny"
)

// Result is the hook processing result. The hook creates decisions and summaries.
// AdditionalContext is used for SessionStart, PreToolUse, and UserPromptSubmit.
// SystemMessage carries notifications for humans, such as Stop questions.
type Result struct {
	Decision          Decision
	Reason            string
	AdditionalContext string
	Summary           string
	SystemMessage     string
}

type output struct {
	Decision      string          `json:"decision,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	SystemMessage string          `json:"systemMessage,omitempty"`
	Specific      *specificOutput `json:"hookSpecificOutput,omitempty"`
}

type specificOutput struct {
	Name       Name   `json:"hookEventName"`
	Permission string `json:"permissionDecision,omitempty"`
	Reason     string `json:"permissionDecisionReason,omitempty"`
	Context    string `json:"additionalContext,omitempty"`
}

// Format converts a processing result to host output JSON, returning {} even when no action is needed.
// If the output exceeds the host limit, it uses Summary and guidance to the full text.
func Format(host Host, name Name, result Result) ([]byte, error) {
	if err := validHost(host); err != nil {
		return nil, err
	}
	if err := validName(name); err != nil {
		return nil, err
	}
	out := output{SystemMessage: result.SystemMessage}
	var specific specificOutput
	specific.Name = name
	switch result.Decision {
	case "":
	case Block:
		if name != Stop {
			return nil, fmt.Errorf("cannot output block for %s", name)
		}
		out.Decision = "block"
		out.Reason = result.Reason
	case Allow, Deny:
		if name != PreToolUse {
			return nil, fmt.Errorf("cannot output tool permission decisions for %s", name)
		}
		specific.Permission = string(result.Decision)
		specific.Reason = result.Reason
	default:
		return nil, fmt.Errorf("unsupported result: %q", result.Decision)
	}
	if result.AdditionalContext != "" {
		switch name {
		case SessionStart, PreToolUse, UserPromptSubmit:
		default:
			return nil, fmt.Errorf("cannot output additionalContext for %s", name)
		}
		specific.Context = result.AdditionalContext
	}
	if specific.Permission != "" || specific.Context != "" {
		out.Specific = &specific
	}
	return fitOutput(host, out, &specific, result.Summary)
}

func fitOutput(host Host, out output, specific *specificOutput, summary string) ([]byte, error) {
	data, err := json.Marshal(out)
	if err != nil || fits(host, string(data)) {
		return data, err
	}
	const guide = "\nRun `devkit status` for the full text"
	if specific.Context != "" {
		specific.Context = summary + guide
		data, err = json.Marshal(out)
		if err != nil || fits(host, string(data)) {
			return data, err
		}
	}
	// Shorten text against the serialized output budget, preserving the full-text guide.
	for _, field := range []*string{&out.SystemMessage, &out.Reason, &specific.Reason, &specific.Context} {
		suffix := ""
		text := *field
		if field == &specific.Context && text != "" {
			text = summary
			suffix = guide
		}
		runes := []rune(text)
		low, high := 0, len(runes)
		for low < high {
			mid := low + (high-low+1)/2
			*field = string(runes[:mid]) + suffix
			data, err = json.Marshal(out)
			if err != nil {
				return nil, err
			}
			if fits(host, string(data)) {
				low = mid
			} else {
				high = mid - 1
			}
		}
		*field = string(runes[:low]) + suffix
		data, err = json.Marshal(out)
		if err != nil || fits(host, string(data)) {
			return data, err
		}
	}
	return data, nil
}

func fits(host Host, text string) bool {
	if host == ClaudeCode {
		return utf8.RuneCountInString(text) <= 10000
	}
	// Approximate 2,500 tokens as 10,000 UTF-8 bytes, at four bytes per token.
	return len(text) <= 10000
}
