// Package assets embeds the repository norms and binary-owned assets distributed by devkit.
// It does not depend on implementation packages; internal/kit supplies these assets.
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
