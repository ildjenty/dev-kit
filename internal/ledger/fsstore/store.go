package fsstore

import (
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

// Record holds the ID from its filename and the JSON body interpreted by ledger.
type Record struct {
	ID   string
	Data json.RawMessage
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
	if err := validateJSON(path, data); err != nil {
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
		data, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		records = append(records, Record{ID: id, Data: data})
	}
	return records, nil
}

// Get reads a record and reports JSON corruption with the filename and reason.
// The caller interpreting the body checks schema and kit compatibility.
func (s *Store) Get(id string) (json.RawMessage, error) {
	path, err := s.recordPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateJSON(path, data); err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
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

func validateJSON(path string, data []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("record %s: cannot read JSON: %w", path, err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
