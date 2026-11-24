package judgeeval

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/config"
)

func ValidateCase(dir string, settings config.Config, standards fs.FS) (Case, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Case{}, err
	}
	c, err := loadCase(dir)
	if err != nil {
		return Case{}, err
	}
	root, err := os.MkdirTemp("", "devkit-validate-case-*")
	if err != nil {
		return Case{}, err
	}
	defer os.RemoveAll(root)
	_, err = repository(c, root, settings, standards)
	return c, err
}
