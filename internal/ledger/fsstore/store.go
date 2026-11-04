package fsstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"
)

// Store handles files under the root supplied by the caller.
// Use the work root for records and locks, or the run root for session artifacts.
type Store struct {
	root string
}

// Record is the storage envelope. Ledger interprets Kind and Payload.
type Record struct {
	ID      string          `json:"id"`
	NoteKey string          `json:"note_key,omitempty"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// New sets the storage root without resolving it or creating files.
func New(root string) *Store {
	return &Store{root: root}
}

// Append adds records/<ID>.json without overwriting existing records.
// The caller acquires the work lock for normal operations; Append does not acquire it.
func (s *Store) Append(id string, data json.RawMessage) error {
	path, err := s.recordPath(id)
	if err != nil {
		return err
	}
	if _, err := validateRecord(path, id, data); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+id+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := writeAndClose(f, data); err != nil {
		return err
	}
	// Serialize the existence check and rename without taking the work lock.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := lockFile(dir); err != nil {
		return fmt.Errorf("record publication lock %s: %w", dir.Name(), err)
	}
	if _, err := os.Lstat(path); err == nil {
		return &fs.PathError{Op: "append", Path: path, Err: fs.ErrExist}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(f.Name(), path)
}

// List returns all records in ID order and ignores temporary files.
// It returns an error for corrupt records and leaves their files intact.
func (s *Store) List() ([]Record, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "records"))
	if os.IsNotExist(err) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		record, err := s.GetRecord(id)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// Get reads a record and reports JSON corruption with the filename and reason.
// The caller interpreting the body checks schema and kit compatibility.
func (s *Store) Get(id string) (json.RawMessage, error) {
	data, _, err := s.readRecord(id)
	return data, err
}

func (s *Store) readRecord(id string) (json.RawMessage, Record, error) {
	path, err := s.recordPath(id)
	if err != nil {
		return nil, Record{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Record{}, err
	}
	record, err := validateRecord(path, id, data)
	if err != nil {
		return nil, Record{}, err
	}
	return json.RawMessage(data), record, nil
}

// Publish renames a temporary file to publish an artifact at a root-relative path.
// The caller assigns unique names to appended observations and results.
// Only the regenerable view.html is saved with a normal write.
func (s *Store) Publish(name string, data []byte) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("artifact path must be relative to the root: %q", name)
	}
	path := filepath.Join(s.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if filepath.Clean(name) == "view.html" {
		return os.WriteFile(path, data, 0o600)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".publish-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := writeAndClose(f, data); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Lock waits up to 10 seconds for the interprocess exclusive lock on run.lock.
// The caller must Close it when finished. The lock file is not removed.
func (s *Store) Lock() (*Lock, error) {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.root, "run.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("work lock %s: %w", f.Name(), err)
	}
	return &Lock{file: f}, nil
}

// Lock represents the lifetime of a work lock.
type Lock struct {
	file *os.File
}

// Close closes the file and releases the work lock.
func (l *Lock) Close() error {
	return l.file.Close()
}

func lockFile(f *os.File) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return os.ErrDeadlineExceeded
		}
		time.Sleep(min(100*time.Millisecond, remaining))
	}
}

func (s *Store) recordPath(id string) (string, error) {
	path := filepath.Join(s.root, "records", id+".json")
	if _, err := ulid.ParseStrict(id); err != nil {
		return "", fmt.Errorf("record %s: record ID is not a ULID: %w", path, err)
	}
	return path, nil
}

// DecodePayload decodes a stored payload without accepting unknown fields.
func DecodePayload(data []byte, dst any) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid record JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// DecodeRecord decodes the envelope and checks its ID against the filename.
func DecodeRecord(id string, data []byte) (Record, error) {
	var record Record
	if err := DecodePayload(data, &record); err != nil {
		return Record{}, err
	}
	if record.ID != id {
		return Record{}, fmt.Errorf("body ID %q does not match filename %q", record.ID, id)
	}
	if record.Kind == "" {
		return Record{}, fmt.Errorf("record kind is required")
	}
	payload := bytes.TrimSpace(record.Payload)
	if len(payload) == 0 || payload[0] != '{' {
		return Record{}, fmt.Errorf("record payload must be an object")
	}
	return record, nil
}

func validateRecord(path, id string, data []byte) (Record, error) {
	record, err := DecodeRecord(id, data)
	if err != nil {
		return Record{}, fmt.Errorf("record %s: %w", path, err)
	}
	return record, nil
}

// AppendRecord serializes and publishes a storage envelope.
func (s *Store) AppendRecord(id, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("record %s.json: %w", id, err)
	}
	body, err := json.Marshal(Record{ID: id, Kind: kind, Payload: data})
	if err != nil {
		return fmt.Errorf("record %s.json: %w", id, err)
	}
	return s.Append(id, body)
}

// GetRecord reads and decodes a storage envelope.
func (s *Store) GetRecord(id string) (Record, error) {
	_, record, err := s.readRecord(id)
	return record, err
}

// NextID returns a ULID greater than the last published record ID.
// The caller holds the work lock through publication, except for pause.
func (s *Store) NextID() (string, error) {
	records, err := s.List()
	if err != nil {
		return "", err
	}
	next := ulid.Make()
	if len(records) != 0 {
		last, err := ulid.ParseStrict(records[len(records)-1].ID)
		if err != nil {
			return "", err
		}
		if next.Compare(last) <= 0 {
			next = last
			for i := len(next) - 1; i >= 0; i-- {
				next[i]++
				if next[i] != 0 {
					return next.String(), nil
				}
			}
			return "", fmt.Errorf("record ID space exhausted")
		}
	}
	return next.String(), nil
}

func writeAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
