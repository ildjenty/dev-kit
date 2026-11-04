// Package assets embeds everything devkit distributes: the kit assets and the principles.
// init copies this tree as is into .devkit/kit/<version>/ in the using repository.
// It does not depend on implementation packages; internal/kit supplies and verifies assets.
// Source of truth: the assets section in docs/architecture/modules.md.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed checks examples hosts plans principles router rules schemas skills stages templates
var files embed.FS

// FS returns the embedded assets.
func FS() fs.FS {
	return files
}
