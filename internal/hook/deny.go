package hook

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func (h Handler) preToolUse(e event.Event, state *ledger.State) (event.Result, error) {
	if h.ReadConfig == nil {
		return event.Result{}, fmt.Errorf("normative configuration reader is required")
	}
	work, ok := state.Work()
	if !ok {
		return event.Result{}, fmt.Errorf("work record is required")
	}
	reference := work.Branch
	if round, ok := state.OpenRound(); ok {
		reference = round.StartTarget.Tree
	}
	settings, err := h.ReadConfig(reference)
	if err != nil {
		return event.Result{}, err
	}
	for _, rule := range settings.Deny.Rules {
		if matchesRule(rule, e) {
			return denied("Operation matches deny rule: " + rule), nil
		}
	}
	if e.AgentID != "" && strings.HasPrefix(e.AgentType, "devkit-") {
		for _, tool := range settings.Deny.Judge {
			if e.ToolName == tool {
				return denied("Operation matches judge deny tool: " + tool), nil
			}
		}
	}
	if e.Agent == nil || !strings.HasPrefix(e.Agent.Type, "devkit-") {
		return event.Result{}, nil
	}
	if e.Host == event.Codex {
		if e.Agent.ForkTurns != "none" {
			return denied("Devkit subagents require fork_turns: none."), nil
		}
		return event.Result{}, nil
	}
	const prefix = "devkit-instruction "
	id, ok := strings.CutPrefix(e.Agent.Prompt, prefix)
	if !ok || id == "" || strings.IndexFunc(id, unicode.IsSpace) >= 0 {
		return denied("Devkit subagent prompts must be exactly one line: devkit-instruction <instruction_id>."), nil
	}
	if h.ReadAgent == nil {
		return event.Result{}, fmt.Errorf("normative agent definition reader is required")
	}
	definition, err := h.ReadAgent(reference, e.Host, e.Agent.Type)
	if err != nil {
		return event.Result{}, err
	}
	if definition.Name != e.Agent.Type {
		return denied("The subagent name does not match its normative definition."), nil
	}
	if e.Agent.Model != "" && e.Agent.Model != definition.Model {
		return denied("The model override changes the normative subagent definition."), nil
	}
	if e.Agent.Effort != "" && e.Agent.Effort != definition.Effort {
		return denied("The effort override changes the normative subagent definition."), nil
	}
	return event.Result{}, nil
}

func denied(reason string) event.Result { return event.Result{Decision: event.Deny, Reason: reason} }

func pauseCommand(e event.Event) bool {
	if e.Name != event.PreToolUse {
		return false
	}
	fields := strings.Fields(e.Command)
	return len(fields) >= 2 && fields[0] == "devkit" && fields[1] == "pause" && len(commandParts(strings.TrimSpace(e.Command))) == 1
}

func matchesRule(rule string, e event.Event) bool {
	if prefix, ok := strings.CutPrefix(rule, "Bash("); ok && strings.HasSuffix(prefix, " *)") {
		prefix = strings.TrimSuffix(prefix, " *)")
		for _, command := range commandParts(e.Command) {
			rest, ok := strings.CutPrefix(strings.TrimSpace(command), prefix)
			if ok && (rest == "" || strings.IndexFunc(rest[:1], unicode.IsSpace) == 0) {
				return true
			}
		}
		return false
	}
	if attribute, ok := strings.CutPrefix(rule, "Agent("); ok && strings.HasSuffix(attribute, ")") {
		return e.Agent != nil && e.Agent.Type == strings.TrimSuffix(attribute, ")")
	}
	return e.ToolName == rule
}

// commandParts splits linear shell connectors, preserving quoted and escaped text.
// It does not interpret substitutions or implement host permission-rule semantics.
func commandParts(command string) []string {
	parts := []string{}
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if c == '\\' && quote != '\'' {
			i++
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == ';' || c == '|' || c == '&' || c == '\n' {
			parts = append(parts, command[start:i])
			if (c == '|' || c == '&') && i+1 < len(command) && command[i+1] == c {
				i++
			}
			start = i + 1
		}
	}
	return append(parts, command[start:])
}
