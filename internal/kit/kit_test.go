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
		t.Fatalf("コピーの場所またはバージョンが異なる: %s, %s", k.Dir(), k.Version())
	}
	for _, pair := range [][2]string{
		{"docs/parts/runtime.md", "docs/parts/runtime.md"},
		{"principles/harness.md", "principles/harness.md"},
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
			t.Errorf("コピーの内容が異なる: %s", pair[1])
		}
	}
	if _, err := fs.Stat(k, "kit"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("kit の内容が直下に配置されていない: %v", err)
	}
	if err := kit.Copy(repo); err != nil {
		t.Fatalf("同内容の再コピーに失敗: %v", err)
	}
	if err := k.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckVersion(kit.Version); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckVersion("unknown"); err == nil {
		t.Fatal("Work とコピーの不一致を拒否していない")
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
	name := "plans/README.md"
	changed := []byte("変更したコピー\n")
	if err := os.WriteFile(filepath.Join(k.Dir(), filepath.FromSlash(name)), changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := k.Verify(); err == nil {
		t.Fatal("コピーの内容差分を検出していない")
	}
	if err := kit.Copy(repo); err == nil {
		t.Fatal("異なる既存ファイルの上書きを拒否していない")
	}
	got, err := k.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, changed) {
		t.Fatal("読み出し元がコピーではない、またはコピーを上書きした")
	}
	if err := os.Remove(filepath.Join(k.Dir(), filepath.FromSlash(name))); err != nil {
		t.Fatal(err)
	}
	if _, err := k.ReadFile(name); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("欠落したコピーを埋め込みから補っている: %v", err)
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
			target := filepath.Join(k.Dir(), "plans", "README.md")
			switch change {
			case "missing":
				err = os.Remove(target)
			case "extra":
				err = os.WriteFile(filepath.Join(k.Dir(), "extra.md"), []byte("追加\n"), 0o644)
			case "extra_directory":
				err = os.Mkdir(filepath.Join(k.Dir(), "extra"), 0o755)
			case "different_type":
				if err := os.Rename(target, target+".original"); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink("README.md.original", target)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := k.Verify(); err == nil {
				t.Fatal("コピーの配置差分を検出していない")
			}
		})
	}
}

func TestOpenRejectsUnavailableCopy(t *testing.T) {
	repo := t.TempDir()
	if _, err := kit.Open(repo, kit.Version); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("コピーがないときにエラーにならない: %v", err)
	}
	if _, err := kit.Open(repo, "unknown"); err == nil {
		t.Fatal("未知のバージョンを拒否していない")
	}
}
