package kit_test

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/assets"
	"github.com/ildjenty/dev-kit/internal/kit"
)

func TestNormsLayoutAndContents(t *testing.T) {
	norms, err := kit.Norms()
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"principles", "stages", "checks", "plans", "templates", "examples", "rules"} {
		if err := fs.WalkDir(assets.FS(), root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || name == "examples/README.md" || name == "rules/README.md" {
				return err
			}
			target := name
			if root == "examples" {
				target = "checks/" + strings.TrimPrefix(name, "examples/")
			} else if root == "rules" {
				target = "rules.md"
			}
			want, err := fs.ReadFile(assets.FS(), name)
			if err != nil {
				return err
			}
			if !bytes.Equal(norms[target], want) {
				t.Errorf("norm contents differ: %s", target)
			}
			delete(norms, target)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(norms) != 0 {
		t.Fatalf("unexpected distributable norms: %v", norms)
	}
}

func TestBinaryAssets(t *testing.T) {
	if kit.Version != "v2" {
		t.Fatalf("kit version = %q; want v2", kit.Version)
	}
	for _, pair := range []struct {
		path string
		read func() ([]byte, error)
	}{
		{"schemas/plan.json", func() ([]byte, error) { return kit.Schema("plan") }},
		{"router/default.toml", kit.RouterDefaults},
		{"hosts/claude/settings.json", func() ([]byte, error) { return kit.HostFragment("claude/settings.json") }},
		{"skills/devkit/SKILL.md", func() ([]byte, error) { return fs.ReadFile(kit.FS(), "skills/devkit/SKILL.md") }},
	} {
		want, err := fs.ReadFile(assets.FS(), pair.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pair.read()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("embedded asset differs: %s: %v", pair.path, err)
		}
	}
	if _, err := kit.Schema("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing schema was accepted: %v", err)
	}
	for _, name := range []string{"", ".", "../plan", "nested/plan"} {
		if _, err := kit.Schema(name); err == nil {
			t.Errorf("invalid schema name was accepted: %q", name)
		}
	}
	if _, err := kit.HostFragment("../router/default.toml"); err == nil {
		t.Fatal("invalid host fragment path was accepted")
	}
}
