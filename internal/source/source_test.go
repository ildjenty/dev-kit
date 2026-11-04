package source

import (
	"os"
	"path/filepath"
	"reflect"
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
	adapter, err := NewFiles(root, "norms", snapshot, "commit-id")
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
	if _, err := NewFiles(root, "norms", nil, ""); err == nil {
		t.Fatal("repository source accepted a live filesystem fallback")
	}
	single, err := NewFiles(root, "rules.md", snapshot, "commit-id")
	if err != nil {
		t.Fatal(err)
	}
	documents, err := single.Select("")
	if err != nil || !reflect.DeepEqual(documents, []Document{{ID: "rules.md", Body: "Frozen rules", Origin: "commit-id"}}) {
		t.Fatalf("single-file source = %+v, %v", documents, err)
	}
	missing, err := NewFiles(root, "requirements.md", snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	if documents, err := missing.Select(""); err != nil || len(documents) != 0 {
		t.Fatalf("missing product = %+v, %v", documents, err)
	}
}

func TestExternalSource(t *testing.T) {
	repository := t.TempDir()
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "rules.md"), []byte("Original"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewFiles(repository, vault, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	documents, err := adapter.Select("")
	if err != nil || len(documents) != 1 || documents[0].Body != "Original" {
		t.Fatalf("external source = %+v, %v", documents, err)
	}
}
