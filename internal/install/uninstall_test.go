package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallRemovesReadOnlySnapshots(t *testing.T) {
	root := repository(t)
	ctx := context.Background()
	if _, err := Init(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	const snapshot = ".devkit/run/work/one/snap/tree"
	writeFile(t, root, snapshot+"/nested/file.txt", "Snapshot content.\n")
	if err := os.Chmod(filepath.Join(root, snapshot, "nested/file.txt"), 0o444); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	before, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, snapshot, "external")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{snapshot + "/nested", snapshot} {
		if err := os.Chmod(filepath.Join(root, name), 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range []string{snapshot, snapshot + "/nested"} {
			if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Error(err)
			}
		}
	})
	declined := errors.New("uninstall declined")
	if _, err := Uninstall(ctx, root, func([]Change) error { return declined }); !errors.Is(err, declined) {
		t.Fatalf("confirmation error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, snapshot)); err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("snapshot permissions changed before confirmation: %v, %v", info, err)
	}
	if _, err := Uninstall(ctx, root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".devkit/run")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime state remains: %v", err)
	}
	if info, err := os.Stat(outside); err != nil || info.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("symlink target permissions changed: %v, %v", info, err)
	}
}
