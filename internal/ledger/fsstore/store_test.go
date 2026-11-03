package fsstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	firstID  = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	secondID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
)

func TestRecords(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	if records, err := store.List(); err != nil || len(records) != 0 {
		t.Fatalf("empty records: %v, %v", records, err)
	}
	for _, id := range []string{secondID, firstID} {
		data := json.RawMessage(`{"id":"` + id + `","kind":"work"}`)
		if err := store.Append(id, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Append(firstID, json.RawMessage(`{"changed":true}`)); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("overwrite was not rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "records", ".unfinished.tmp"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != firstID || records[1].ID != secondID {
		t.Fatalf("record order and count: %v", records)
	}
	if !strings.Contains(string(records[0].Data), `"kind":"work"`) {
		t.Fatalf("existing record changed: %s", records[0].Data)
	}
	path := filepath.Join(root, "records", secondID+".json")
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("corrupt file was not reported: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != `{"broken":` {
		t.Fatalf("corrupt record was not preserved: %s, %v", data, err)
	}
}

func TestPublishAndInvalidInput(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	for _, body := range []string{"initial body", "regenerated body"} {
		if err := store.Publish("brief/instruction.md", []byte(body)); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "brief", "instruction.md"))
		if err != nil || string(data) != body {
			t.Fatalf("artifact: %s, %v", data, err)
		}
	}
	if err := store.Publish("view.html", []byte("<html>current state</html>")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "view.html")); err != nil || string(data) != "<html>current state</html>" {
		t.Fatalf("saved view: %s, %v", data, err)
	}
	for _, name := range []string{"../outside", filepath.Join(root, "absolute")} {
		if err := store.Publish(name, []byte("body")); err == nil {
			t.Fatalf("path outside root was accepted: %s", name)
		}
	}
	if err := store.Append("../outside", json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid record ID was accepted")
	}
	if err := store.Append(firstID, json.RawMessage(`{`)); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if _, err := store.Get(firstID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("invalid record was published: %v", err)
	}
}

func TestAppendWithStaleTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "records"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"." + firstID + ".tmp", "." + firstID + "-stale.tmp"} {
		if err := os.WriteFile(filepath.Join(root, "records", name), []byte(`{`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := New(root)
	if err := store.Append(firstID, json.RawMessage(`{"kind":"pause"}`)); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil || len(records) != 1 || records[0].ID != firstID {
		t.Fatalf("records after stale temporary files: %v, %v", records, err)
	}
}

func TestConcurrentAppend(t *testing.T) {
	store := New(t.TempDir())
	const writers = 8
	start := make(chan struct{})
	results := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func() {
			<-start
			results <- store.Append(firstID, json.RawMessage(`{"kind":"pause"}`))
		}()
	}
	close(start)
	succeeded := 0
	for i := 0; i < writers; i++ {
		if err := <-results; err == nil {
			succeeded++
		} else if !errors.Is(err, fs.ErrExist) {
			t.Fatal(err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful appends: got %d, want 1", succeeded)
	}
}

func TestWorkLockTimeout(t *testing.T) {
	store := New(t.TempDir())
	lock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	started := time.Now()
	other, err := store.Lock()
	if other != nil {
		other.Close()
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) || time.Since(started) < 10*time.Second {
		t.Fatalf("lock timeout after %v: %v", time.Since(started), err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	other, err = store.Lock()
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	other.Close()
}

func TestWorkLock(t *testing.T) {
	if root := os.Getenv("DEVKIT_FSSTORE_LOCK_TEST_ROOT"); root != "" {
		f, err := os.OpenFile(filepath.Join(root, "run.lock"), os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		blocked := errors.Is(err, syscall.EWOULDBLOCK)
		wantBlocked := os.Getenv("DEVKIT_FSSTORE_LOCK_TEST_BLOCKED") == "true"
		if blocked != wantBlocked || (err != nil && !blocked) {
			t.Fatalf("lock from another process: %v, expected contention: %v", err, wantBlocked)
		}
		return
	}
	root := t.TempDir()
	store := New(root)
	lock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	check := func(blocked string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestWorkLock$")
		cmd.Env = append(os.Environ(), "DEVKIT_FSSTORE_LOCK_TEST_ROOT="+root, "DEVKIT_FSSTORE_LOCK_TEST_BLOCKED="+blocked)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("child process: %s, %v", output, err)
		}
	}
	check("true")
	// A pause record can be published independently while the work lock is held.
	if err := store.Append(firstID, json.RawMessage(`{"kind":"pause"}`)); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	check("false")
	if _, err := os.Stat(filepath.Join(root, "run.lock")); err != nil {
		t.Fatalf("lock file was not preserved: %v", err)
	}
}
