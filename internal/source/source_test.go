package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
)

func TestSelectSnapshotDocuments(t *testing.T) {
	root := t.TempDir()
	snapshot := fstest.MapFS{
		"norms/top.md":                {Data: []byte("Frozen top")},
		"norms/review/test.md":        {Data: []byte("Frozen review")},
		"norms/review/deep/extra.txt": {Data: []byte("Extra")},
		"rules.md":                    {Data: []byte("Frozen rules")},
	}
	adapter, err := NewFiles(root, "norms", snapshot, "commit-id", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		selector string
		ids      []string
	}{
		{"", []string{"review/deep/extra.txt", "review/test.md", "top.md"}},
		{"*.md", []string{"top.md"}},
		{"**/*.md", []string{"review/test.md", "top.md"}},
		{"review/**", []string{"review/deep/extra.txt", "review/test.md"}},
		{"review/test.md", []string{"review/test.md"}},
		{"missing", []string{}},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			documents, err := adapter.Select(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, document := range documents {
				ids = append(ids, document.ID)
				if document.Origin != "commit-id" || document.Body != string(snapshot["norms/"+document.ID].Data) {
					t.Fatalf("document differs from snapshot: %+v", document)
				}
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("IDs = %v; want %v", ids, tc.ids)
			}
		})
	}
	for _, selector := range []string{"../rules.md", "/rules.md", "[", "review/[", `review\test.md`} {
		if _, err := adapter.Select(selector); err == nil {
			t.Fatalf("accepted invalid selector: %q", selector)
		}
	}
	if _, err := NewFiles(root, "norms", nil, "", true); err == nil {
		t.Fatal("repository source accepted a live filesystem fallback")
	}
	single, err := NewFiles(root, "rules.md", snapshot, "commit-id", false)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := single.Select("")
	if err != nil || !reflect.DeepEqual(documents, []Document{{ID: "rules.md", Body: "Frozen rules", Origin: "commit-id"}}) {
		t.Fatalf("single-file source = %+v, %v", documents, err)
	}
	missing, err := NewFiles(root, "requirements.md", snapshot, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if documents, err := missing.Select(""); err != nil || len(documents) != 0 {
		t.Fatalf("missing product = %+v, %v", documents, err)
	}
}

func TestExternalSourceAndCheckedUpdates(t *testing.T) {
	repository := t.TempDir()
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "rules.md"), []byte("Original"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewFiles(repository, vault, nil, "", true)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := adapter.Select("")
	if err != nil || len(documents) != 1 || documents[0].Body != "Original" {
		t.Fatalf("external source = %+v, %v", documents, err)
	}
	updated := "Updated"
	if err := adapter.Write("rules.md", &updated, ContentHash("Stale")); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update = %v", err)
	}
	if err := adapter.Write("rules.md", &updated, ContentHash("Original")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(vault, "rules.md"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("updated permissions = %+v, %v", info, err)
	}
	if err := adapter.Write("nested/new.md", &updated, ""); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Write("nested/new.md", nil, ContentHash(updated)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(vault, "nested/new.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted document remains: %v", err)
	}
	if err := adapter.Write("rules.md", nil, ContentHash("Stale")); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale deletion = %v", err)
	}
	if err := adapter.Write("rules.md", &updated, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("addition replaced an existing file: %v", err)
	}
	for _, id := range []string{"../outside.md", "/outside.md", ".", `nested\bad.md`} {
		if err := adapter.Write(id, &updated, ""); err == nil {
			t.Fatalf("accepted invalid document ID: %q", id)
		}
	}
	if err := os.Symlink(repository, filepath.Join(vault, "outside")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Write("outside/escape.md", &updated, ""); err == nil {
		t.Fatal("write escaped the source through a symlink")
	}
	readOnly, err := NewFiles(repository, vault, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := readOnly.Write("rules.md", &updated, ContentHash(updated)); err == nil {
		t.Fatal("read-only source accepted a write")
	}
}

func TestSingleFileDeleteAndCreate(t *testing.T) {
	repository := t.TempDir()
	vault := t.TempDir()
	location := filepath.Join(vault, "rules.md")
	if err := os.WriteFile(location, []byte("Original"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewFiles(repository, location, nil, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Write("other.md", nil, ""); err == nil {
		t.Fatal("single-file source accepted another document ID")
	}
	if err := adapter.Write("rules.md", nil, ContentHash("Original")); err != nil {
		t.Fatal(err)
	}
	body := "Recreated"
	if err := adapter.Write("rules.md", &body, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(location)
	if err != nil || string(data) != body {
		t.Fatalf("recreated file = %q, %v", data, err)
	}
}

func TestConcurrentUpdatesDoNotOverwrite(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "rules.md"), []byte("Original"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewFiles(t.TempDir(), vault, nil, "", true)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for _, body := range []string{"First", "Second"} {
		group.Go(func() { results <- adapter.Write("rules.md", &body, ContentHash("Original")) })
	}
	group.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful updates = %d; want 1", succeeded)
	}
}
