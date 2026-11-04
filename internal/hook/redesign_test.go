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
