package initialize

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
	"github.com/ildjenty/dev-kit/kit"
)

func TestEveryKitFileIsEmbeddedExceptGoSource(t *testing.T) {
	diskRoot := filepath.Join("..", "..", "kit")
	disk := []string{}
	testutil.OK(t, filepath.WalkDir(diskRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() != "embed.go" {
			disk = append(disk, testutil.Must(filepath.Rel(diskRoot, path)))
		}
		return nil
	}))
	embedded := []string{}
	testutil.OK(t, fs.WalkDir(kit.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			embedded = append(embedded, path)
		}
		return nil
	}))
	testutil.Equal(t, embedded, disk)
}
