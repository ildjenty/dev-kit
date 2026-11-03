package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/engine"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
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
	w, err := workspace.Open(ctx, cwd)
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
	h.ReadConfig = func(ref string) (config.Config, error) {
		data, err := w.ReadFile(ctx, ref, "devkit.toml")
		if err != nil {
			return config.Config{}, err
		}
		return config.Parse(data)
	}
	h.ReadAgent = func(ref string, host event.Host, name string) (hook.AgentDefinition, error) {
		return readAgent(ctx, w, ref, host, name)
	}
	h.InstructionRunner = func(instruction ledger.Instruction) (string, error) {
		runtime, err := engine.New(ctx, w.Root(), e.SessionID)
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
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		switch strings.TrimSpace(key) {
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
