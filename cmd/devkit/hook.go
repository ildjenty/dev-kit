package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func runHook(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("hook requires an event name")
	}
	name, args := event.Name(args[0]), args[1:]
	f := flags("hook")
	hostName := f.String("host", "", "Host sending the event")
	if err := parse(f, args, 0, 0); err != nil {
		return err
	}
	if *hostName == "" {
		if os.Getenv("CLAUDE_CODE_SESSION_ID") != "" {
			*hostName = string(event.ClaudeCode)
		} else if os.Getenv("CODEX_THREAD_ID") != "" {
			*hostName = string(event.Codex)
		} else {
			return errors.New("host is unknown; supply --host claude-code or --host codex")
		}
	}
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	e, err := event.Parse(event.Host(*hostName), data)
	if err != nil {
		return err
	}
	if e.Name != name {
		return errors.New("hook event name does not match its command argument")
	}
	if e.SessionID == "" {
		e.SessionID = sessionID()
	}
	if e.SessionID == "" || !localName(e.SessionID) {
		return errors.New("hook requires a valid session ID")
	}
	result, err := processHook(ctx, e)
	if err != nil {
		if e.Name != event.SessionStart {
			return err
		}
		result = event.Result{AdditionalContext: "Warning: " + err.Error()}
	}
	data, err = event.Format(e.Host, e.Name, result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(data))
	return err
}

func processHook(ctx context.Context, e event.Event) (event.Result, error) {
	if e.AgentID != "" && (e.Name == event.SessionStart || e.Name == event.Stop) {
		return event.Result{}, nil
	}
	cwd := e.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return event.Result{}, err
		}
	}
	w, err := hookWorkspace(ctx, cwd)
	if err != nil {
		return event.Result{}, err
	}
	observation, err := hook.ReadSession(filepath.Join(w.Root(), ".devkit/run"), e.SessionID)
	if err != nil {
		return event.Result{}, err
	}
	h := hook.Handler{Root: w.Root()}
	if observation != nil {
		h.WorkID = observation.WorkID
	}
	if h.WorkID == "" || e.Name == event.UserPromptSubmit || e.Name == event.PreToolUse && strings.TrimSpace(e.Command) == "devkit pause" {
		return h.Process(e)
	}
	if !localName(h.WorkID) {
		return event.Result{}, errors.New("invalid bound work ID")
	}
	h.Validator, err = workValidator(ctx, w, filepath.Join(w.Root(), ".devkit/run/work", h.WorkID))
	if err != nil {
		if e.Name == event.SessionStart {
			result, processErr := h.Process(e)
			if processErr != nil {
				return result, processErr
			}
			result.AdditionalContext = "Warning: " + err.Error()
			return result, nil
		}
		return event.Result{}, err
	}
	state, err := ledger.Load(fsstore.New(filepath.Join(w.Root(), ".devkit/run/work", h.WorkID)), nil, h.Validator)
	if err != nil {
		return event.Result{}, err
	}
	normativeRef := func(ref string) string {
		ref, _, _ = strings.Cut(ref, "@")
		if round, ok := state.OpenRound(); ok && round.StartTarget.Tree == ref && round.NormReference != "" {
			git, _, _ := strings.Cut(round.NormReference, "@")
			return git
		}
		return ref
	}
	h.ReadConfig = func(ref string) (config.Config, error) {
		data, err := w.ReadFile(ctx, normativeRef(ref), "devkit.toml")
		if err != nil {
			return config.Config{}, err
		}
		return config.Parse(data)
	}
	h.ReadAgent = func(ref string, host event.Host, name string) (hook.AgentDefinition, error) {
		return readAgent(ctx, w, normativeRef(ref), host, name)
	}
	h.InstructionRunner = func(instruction ledger.Instruction) (string, error) {
		runtime, err := newEngine(ctx, w.Root(), e.SessionID)
		if err != nil {
			return "", err
		}
		if _, err := runtime.Status(ctx); err != nil {
			return "", err
		}
		return runtime.InstructionRunner(instruction)
	}
	return h.Process(e)
}

func hookWorkspace(ctx context.Context, cwd string) (*workspace.Workspace, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return nil, err
	}
	if filepath.Base(filepath.Dir(w.Root())) == "worktrees" && filepath.Base(filepath.Dir(filepath.Dir(w.Root()))) == ".devkit" {
		return workspace.Open(ctx, filepath.Dir(filepath.Dir(filepath.Dir(w.Root()))))
	}
	return w, nil
}

func localName(name string) bool {
	return name != "." && filepath.IsLocal(name) && !strings.ContainsAny(name, "/\\")
}

func readAgent(ctx context.Context, w *workspace.Workspace, ref string, host event.Host, name string) (hook.AgentDefinition, error) {
	if !localName(name) {
		return hook.AgentDefinition{}, errors.New("invalid agent name")
	}
	if host == event.Codex {
		data, err := w.ReadFile(ctx, ref, ".codex/agents/"+name+".toml")
		if err != nil {
			return hook.AgentDefinition{}, err
		}
		var definition struct {
			Name   string `toml:"name"`
			Model  string `toml:"model"`
			Effort string `toml:"model_reasoning_effort"`
		}
		if err := toml.Unmarshal(data, &definition); err != nil {
			return hook.AgentDefinition{}, err
		}
		return hook.AgentDefinition{Name: definition.Name, Model: definition.Model, Effort: definition.Effort}, nil
	}
	data, err := w.ReadFile(ctx, ref, ".claude/agents/"+name+".md")
	if err != nil {
		return hook.AgentDefinition{}, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	front, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return hook.AgentDefinition{}, errors.New("agent definition has no front matter")
	}
	front, _, ok = strings.Cut(front, "\n---")
	if !ok {
		return hook.AgentDefinition{}, errors.New("agent front matter is not terminated")
	}
	var definition hook.AgentDefinition
	for _, line := range strings.Split(front, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key != "name" && key != "model" && key != "effort" && key != "tools" {
			continue
		}
		if key == "tools" {
			tools, err := agentTools(value)
			if err != nil {
				return hook.AgentDefinition{}, err
			}
			definition.Tools = tools
			continue
		}
		value, err = frontmatterValue(value)
		if err != nil {
			return hook.AgentDefinition{}, err
		}
		switch key {
		case "name":
			definition.Name = value
		case "model":
			definition.Model = value
		case "effort":
			definition.Effort = value
		}
	}
	return definition, nil
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
