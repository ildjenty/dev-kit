package engine

import (
	"encoding/json"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
)

func decodeNote(kind ledger.Kind, data []byte) (ledger.Payload, error) {
	var payload ledger.Payload
	switch kind {
	case ledger.KindDecision:
		payload = &ledger.Decision{}
	case ledger.KindEvidence:
		payload = &ledger.Evidence{}
	case ledger.KindLesson:
		payload = &ledger.Lesson{}
	default:
		return nil, fmt.Errorf("unsupported note kind: %q", kind)
	}
	if err := fsstore.DecodePayload(data, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// workerNotes validates every note of one termination block before any is published. The key
// covers the instruction target and the note's content, so acceptance that resumes after a
// partial write, or a reissue returning the same note, skips notes already published, while a
// reissue returning a different note records it.
func (e *Engine) workerNotes(targetKey string, notes []form.Note) ([]ledger.Record, error) {
	if len(notes) == 0 {
		return nil, nil
	}
	saved, err := e.records()
	if err != nil {
		return nil, err
	}
	published := map[string]bool{}
	for _, record := range saved {
		if record.NoteKey != "" {
			published[record.NoteKey] = true
		}
	}
	pending := []ledger.Record{}
	for index, note := range notes {
		payload, err := decodeNote(ledger.Kind(note.Kind), note.Payload)
		if err != nil {
			return nil, fmt.Errorf("notes[%d]: %w", index, err)
		}
		record, err := ledger.NewRecord(e.store(), payload)
		if err != nil {
			return nil, err
		}
		if err := ledger.Validate(e.store(), record, e.validator); err != nil {
			return nil, fmt.Errorf("notes[%d]: %w", index, err)
		}
		content, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		record.NoteKey = hash([]byte(targetKey + "\x00" + note.Kind + "\x00" + string(content)))
		if !published[record.NoteKey] {
			pending = append(pending, record)
		}
	}
	return pending, nil
}

// publishNotes appends notes already validated by workerNotes.
func (e *Engine) publishNotes(notes []ledger.Record) error {
	for _, note := range notes {
		if err := ledger.Append(e.store(), note, e.validator); err != nil {
			return err
		}
	}
	return nil
}
