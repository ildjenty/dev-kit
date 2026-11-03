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
)

const (
	firstID  = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	secondID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
)

func TestRecords(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	if records, err := store.List(); err != nil || len(records) != 0 {
		t.Fatalf("空の記録: %v, %v", records, err)
	}
	for _, id := range []string{secondID, firstID} {
		data := json.RawMessage(`{"id":"` + id + `","kind":"work"}`)
		if err := store.Append(id, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Append(firstID, json.RawMessage(`{"changed":true}`)); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("上書きを拒否しませんでした: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "records", ".unfinished.tmp"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != firstID || records[1].ID != secondID {
		t.Fatalf("記録の順序と件数: %v", records)
	}
	if !strings.Contains(string(records[0].Data), `"kind":"work"`) {
		t.Fatalf("既存の記録が変わりました: %s", records[0].Data)
	}
	path := filepath.Join(root, "records", secondID+".json")
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("破損の対象が報告されませんでした: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != `{"broken":` {
		t.Fatalf("破損した記録が保存されていません: %s, %v", data, err)
	}
}

func TestPublishAndInvalidInput(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	for _, body := range []string{"最初の本文", "再生成した本文"} {
		if err := store.Publish("brief/instruction.md", []byte(body)); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "brief", "instruction.md"))
		if err != nil || string(data) != body {
			t.Fatalf("生成物: %s, %v", data, err)
		}
	}
	if err := store.Publish("view.html", []byte("<html>現在地</html>")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "view.html")); err != nil || string(data) != "<html>現在地</html>" {
		t.Fatalf("view の保存: %s, %v", data, err)
	}
	for _, name := range []string{"../outside", filepath.Join(root, "absolute")} {
		if err := store.Publish(name, []byte("本文")); err == nil {
			t.Fatalf("ルート外のパスが受理されました: %s", name)
		}
	}
	if err := store.Append("../outside", json.RawMessage(`{}`)); err == nil {
		t.Fatal("不正な記録 ID が受理されました")
	}
	if err := store.Append(firstID, json.RawMessage(`{`)); err == nil {
		t.Fatal("壊れた JSON が受理されました")
	}
	if _, err := store.Get(firstID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("壊れた記録が公開されました: %v", err)
	}
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
			t.Fatalf("別プロセスからのロック: %v、競合の期待値: %v", err, wantBlocked)
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
			t.Fatalf("子プロセス: %s, %v", output, err)
		}
	}
	check("true")
	// pause の書き込みは、作業ロックを保持したままでも独立して公開できる。
	if err := store.Append(firstID, json.RawMessage(`{"kind":"pause"}`)); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	check("false")
	if _, err := os.Stat(filepath.Join(root, "run.lock")); err != nil {
		t.Fatalf("ロックファイルが残っていません: %v", err)
	}
}
