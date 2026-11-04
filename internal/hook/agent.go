package hook

import (
	"errors"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// ParseAgentDefinition reads the name, model, effort, and tools of a Claude agent definition
// (front matter) or, when codex is true, a Codex agent definition (TOML).
func ParseAgentDefinition(codex bool, data []byte) (AgentDefinition, error) {
	if codex {
		var definition struct {
			Name   string `toml:"name"`
			Model  string `toml:"model"`
			Effort string `toml:"model_reasoning_effort"`
		}
		if err := toml.Unmarshal(data, &definition); err != nil {
			return AgentDefinition{}, err
		}
		return AgentDefinition{Name: definition.Name, Model: definition.Model, Effort: definition.Effort}, nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	front, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return AgentDefinition{}, errors.New("agent definition has no front matter")
	}
	front, _, ok = strings.Cut(front, "\n---")
	if !ok {
		return AgentDefinition{}, errors.New("agent front matter is not terminated")
	}
	var definition AgentDefinition
	for _, line := range strings.Split(front, "\n") {
		if err := definition.frontmatterLine(line); err != nil {
			return AgentDefinition{}, err
		}
	}
	return definition, nil
}

func (definition *AgentDefinition) frontmatterLine(line string) error {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return nil
	}
	switch strings.TrimSpace(key) {
	case "tools":
		tools, err := agentTools(value)
		definition.Tools = tools
		return err
	case "name":
		return frontmatterInto(&definition.Name, value)
	case "model":
		return frontmatterInto(&definition.Model, value)
	case "effort":
		return frontmatterInto(&definition.Effort, value)
	}
	return nil
}

func frontmatterInto(target *string, value string) error {
	parsed, err := frontmatterValue(value)
	*target = parsed
	return err
}

func frontmatterValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if value[0] == '\'' {
		var result strings.Builder
		for i := 1; i < len(value); i++ {
			if value[i] != '\'' {
				result.WriteByte(value[i])
				continue
			}
			if i+1 < len(value) && value[i+1] == '\'' {
				result.WriteByte('\'')
				i++
				continue
			}
			return result.String(), nil
		}
		return "", errors.New("agent front matter contains an unterminated quoted value")
	}
	if value[0] == '"' {
		for i := 1; i < len(value); i++ {
			if value[i] == '\\' {
				i++
				continue
			}
			if value[i] == '"' {
				return strconv.Unquote(value[:i+1])
			}
		}
		return "", errors.New("agent front matter contains an unterminated quoted value")
	}
	for i := range value {
		if value[i] == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
			value = value[:i]
			break
		}
	}
	return strings.TrimSpace(value), nil
}

func agentTools(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("agent tools must explicitly list read-only tools")
	}
	if strings.HasPrefix(value, "[") {
		if !strings.HasSuffix(value, "]") {
			return nil, errors.New("agent tools list is not terminated")
		}
		value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	} else if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
		parsed, err := toolValue(value)
		if err != nil {
			return nil, err
		}
		value = parsed
	}
	tools := []string{}
	for _, item := range strings.Split(value, ",") {
		tool, err := toolValue(strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		if tool != "" {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func toolValue(value string) (string, error) {
	if strings.HasPrefix(value, "\"") {
		return strconv.Unquote(value)
	}
	if strings.HasPrefix(value, "'") {
		if len(value) < 2 || !strings.HasSuffix(value, "'") {
			return "", errors.New("agent tool contains an unterminated quoted value")
		}
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'"), nil
	}
	return value, nil
}
