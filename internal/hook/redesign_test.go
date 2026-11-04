package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/host/event"
)

func TestUnboundOriginalRequestIsReplacedPerSession(t *testing.T) {
	h := Handler{Root: t.TempDir()}
	for _, prompt := range []string{"First request", "Latest human request", "<task-notification>Ignore this</task-notification>"} {
		process(t, h, event.Event{Host: event.ClaudeCode, Name: event.UserPromptSubmit, SessionID: "one", Prompt: prompt})
	}
	process(t, h, event.Event{Host: event.Codex, Name: event.UserPromptSubmit, SessionID: "two", Prompt: "Another session"})
	for session, want := range map[string]string{"one": "Latest human request", "two": "Another session"} {
		got, err := ReadOriginalRequest(h.runRoot(), session)
		if err != nil || got == nil || *got != want {
			t.Fatalf("session %s: %v, %v", session, got, err)
		}
		files, err := os.ReadDir(filepath.Join(h.runRoot(), "sessions", session))
		if err != nil || len(files) != 1 {
			t.Fatalf("prompt history retained: %v, %v", files, err)
		}
	}
	if got, err := ReadOriginalRequest(h.runRoot(), "missing"); err != nil || got != nil {
		t.Fatalf("missing prompt: %v, %v", got, err)
	}
}

func TestBoundOriginalRequestIsReplacedPerSession(t *testing.T) {
	for _, host := range []event.Host{event.ClaudeCode, event.Codex} {
		t.Run(string(host), func(t *testing.T) {
			h := Handler{Root: t.TempDir()}
			process(t, h, event.Event{Host: host, Name: event.UserPromptSubmit, SessionID: "one", Prompt: "Before binding"})
			h.WorkID = "work-one"
			for _, prompt := range []string{"First bound request", "Latest bound request"} {
				process(t, h, event.Event{Host: host, Name: event.UserPromptSubmit, SessionID: "one", Prompt: prompt})
			}
			if host == event.ClaudeCode {
				process(t, h, event.Event{Host: host, Name: event.UserPromptSubmit, SessionID: "one", Prompt: "<task-notification>Ignore this</task-notification>"})
			}
			process(t, h, event.Event{Host: host, Name: event.UserPromptSubmit, SessionID: "two", Prompt: "Another session"})
			for session, want := range map[string]string{"one": "Latest bound request", "two": "Another session"} {
				got, err := ReadOriginalRequest(h.runRoot(), session)
				if err != nil || got == nil || *got != want {
					t.Fatalf("session %s: %v, %v", session, got, err)
				}
			}
			files, err := filepath.Glob(filepath.Join(h.workRoot(), "prompts", "one-*.txt"))
			if err != nil || len(files) != 2 {
				t.Fatalf("bound prompt observations: %v, %v", files, err)
			}
			for i, want := range []string{"First bound request", "Latest bound request"} {
				data, err := os.ReadFile(files[i])
				if err != nil || string(data) != want {
					t.Fatalf("bound observation: %q, %v; want %q", data, err, want)
				}
			}
		})
	}
}

func TestConfiguredStopContinuationLimit(t *testing.T) {
	h := newHandler(t)
	h.ReadConfig = func(string) (config.Config, error) {
		return config.Config{Limits: config.Limits{StopContinuation: 1}}, nil
	}
	result := process(t, h, event.Event{Name: event.Stop, SessionID: "one"})
	stops, err := ReadStops(h.workRoot())
	if err != nil || len(stops) != 1 || !stops[0].LimitReached || result.SystemMessage == "" {
		t.Fatalf("configured limit was ignored: %+v, %+v, %v", result, stops, err)
	}
}
