package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
)

// SessionObservation preserves the session's work binding across hook invocations.
type SessionObservation struct {
	Host       event.Host `json:"host"`
	SessionID  string     `json:"session_id"`
	ObservedAt time.Time  `json:"observed_at"`
	WorkID     string     `json:"work_id,omitempty"`
}

// SubagentResult preserves final JSON and provenance verified at collection time.
// Schema and target validation belong to the CLI's result acceptance boundary.
type SubagentResult struct {
	InstructionID string          `json:"instruction_id"`
	Runner        string          `json:"runner"`
	AgentID       string          `json:"agent_id"`
	Host          event.Host      `json:"host"`
	ObservedAt    time.Time       `json:"observed_at"`
	Effort        string          `json:"effort,omitempty"`
	Model         string          `json:"model,omitempty"`
	Content       json.RawMessage `json:"content"`
}

func publishSubagentResult(workRoot string, result SubagentResult) error {
	dir := filepath.Join(workRoot, "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	publication, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer publication.Close()
	// Keep the existence check and rename atomic across result collectors.
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := syscall.Flock(int(publication.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			if err != nil {
				return fmt.Errorf("result publication lock %s: %w", dir, err)
			}
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("result publication lock %s: %w", dir, os.ErrDeadlineExceeded)
		}
		time.Sleep(min(100*time.Millisecond, remaining))
	}
	name := filepath.Join("results", result.InstructionID+"-"+result.AgentID+".json")
	if _, err := os.Lstat(filepath.Join(workRoot, name)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return publishJSON(fsstore.New(workRoot), name, result)
}

// PublishSession publishes a hook observation or a CLI binding operation under runRoot.
// The caller preserves the existing binding when publishing later observations.
func PublishSession(runRoot string, observation SessionObservation, operation string) error {
	name := filepath.Join("sessions", observation.SessionID, timestamp(observation.ObservedAt)+"-"+operation+".json")
	return publishJSON(fsstore.New(runRoot), name, observation)
}

// ReadSession returns the latest observation for one session, or nil when absent.
func ReadSession(runRoot, sessionID string) (*SessionObservation, error) {
	observations, err := readJSONFiles[SessionObservation](filepath.Join(runRoot, "sessions", sessionID))
	if err != nil || len(observations) == 0 {
		return nil, err
	}
	slices.SortStableFunc(observations, func(a, b SessionObservation) int { return a.ObservedAt.Compare(b.ObservedAt) })
	return &observations[len(observations)-1], nil
}

// ReadStops returns all retained stop observations in observation order.
// The latest observation is the work-wide counter, including across sessions.
func ReadStops(workRoot string) ([]ledger.StopObservation, error) {
	stops, err := readJSONFiles[ledger.StopObservation](filepath.Join(workRoot, "stops"))
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(stops, func(a, b ledger.StopObservation) int {
		if order := a.ObservedAt.Compare(b.ObservedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return stops, nil
}

func readJSONFiles[T any](dir string) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []T{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := []T{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var value T
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, fmt.Errorf("observation %s: %w", name, err)
		}
		values = append(values, value)
	}
	return values, nil
}

// ReadOriginalRequest returns the latest unbound human prompt, or nil when absent.
func ReadOriginalRequest(runRoot, sessionID string) (*string, error) {
	data, err := os.ReadFile(filepath.Join(runRoot, "sessions", sessionID, "latest-prompt.txt"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := string(data)
	return &text, nil
}
