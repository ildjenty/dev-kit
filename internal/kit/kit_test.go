package kit_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	devkit "github.com/ildjenty/dev-kit"
	"github.com/ildjenty/dev-kit/internal/kit"
)

func TestCopyLayoutAndRead(t *testing.T) {
	repo := t.TempDir()
	if err := kit.Copy(repo); err != nil {
		t.Fatal(err)
	}
	k, err := kit.Open(repo, kit.Version)
	if err != nil {
		t.Fatal(err)
	}
	if k.Dir() != filepath.Join(repo, ".devkit", "kit", kit.Version) || k.Version() != kit.Version {
		t.Fatalf("copy directory or version differs: %s, %s", k.Dir(), k.Version())
	}
	for _, pair := range [][2]string{
		{"principles/harness.md", "principles/harness.md"},
		{"kit/plans/default.json", "plans/default.json"},
		{"kit/plans/README.md", "plans/README.md"},
	} {
		want, err := fs.ReadFile(devkit.FS(), pair[0])
		if err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(k, pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("copy contents differ: %s", pair[1])
		}
	}
	for _, name := range []string{"kit", "docs", "README.md"} {
		if _, err := fs.Stat(k, name); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("unexpected copied path: %s: %v", name, err)
		}
	}
	if err := kit.Copy(repo); err != nil {
		t.Fatalf("recopying identical contents failed: %v", err)
	}
	if err := k.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckVersion(kit.Version); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckVersion("unknown"); err == nil {
		t.Fatal("Work and copy version mismatch was not rejected")
	}
}

func TestCopyWithUnpublishedFile(t *testing.T) {
	repo := t.TempDir()
	parent := filepath.Join(repo, ".devkit", "kit")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, ".kit-interrupted"), []byte("partial copy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := kit.Copy(repo); err != nil {
		t.Fatal(err)
	}
	k, err := kit.Open(repo, kit.Version)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Verify(); err != nil {
		t.Fatal(err)
	}
	if _, err := k.ReadFile(".kit-interrupted"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unpublished file is readable from the copy: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("temporary files remain after copying: %v", entries)
	}
}

func TestCopyReadAndConflict(t *testing.T) {
	repo := t.TempDir()
	if err := kit.Copy(repo); err != nil {
		t.Fatal(err)
	}
	k, err := kit.Open(repo, kit.Version)
	if err != nil {
		t.Fatal(err)
	}
	name := "plans/default.json"
	changed := []byte("modified copy\n")
	if err := os.WriteFile(filepath.Join(k.Dir(), filepath.FromSlash(name)), changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := k.Verify(); err == nil {
		t.Fatal("copy content mismatch was not detected")
	}
	if err := kit.Copy(repo); err == nil {
		t.Fatal("overwriting a different existing file was not rejected")
	}
	got, err := k.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, changed) {
		t.Fatal("read did not use the copy, or the copy was overwritten")
	}
	if err := os.Remove(filepath.Join(k.Dir(), filepath.FromSlash(name))); err != nil {
		t.Fatal(err)
	}
	if _, err := k.ReadFile(name); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing copy was supplied from embedded assets: %v", err)
	}
}

func TestVerifyLayoutMismatch(t *testing.T) {
	for _, change := range []string{"missing", "extra", "extra_directory", "different_type"} {
		t.Run(change, func(t *testing.T) {
			repo := t.TempDir()
			if err := kit.Copy(repo); err != nil {
				t.Fatal(err)
			}
			k, err := kit.Open(repo, kit.Version)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(k.Dir(), "plans", "default.json")
			switch change {
			case "missing":
				err = os.Remove(target)
			case "extra":
				err = os.WriteFile(filepath.Join(k.Dir(), "extra.md"), []byte("extra\n"), 0o644)
			case "extra_directory":
				err = os.Mkdir(filepath.Join(k.Dir(), "extra"), 0o755)
			case "different_type":
				original := filepath.Join(t.TempDir(), "default.json")
				if err := os.Rename(target, original); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(original, target)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := k.Verify(); err == nil {
				t.Fatal("copy layout mismatch was not detected")
			}
		})
	}
}

func TestOpenRejectsUnavailableCopy(t *testing.T) {
	repo := t.TempDir()
	if _, err := kit.Open(repo, kit.Version); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing copy did not cause an error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".devkit", "kit", "unknown"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := kit.Open(repo, "unknown"); err == nil {
		t.Fatal("unknown version was not rejected")
	}
}
