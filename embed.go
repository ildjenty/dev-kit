// devkit embeds the README and assets from principles, docs, and kit as an fs.FS.
// It does not depend on implementation packages; internal/kit supplies and verifies assets.
// Source of truth: the root package section in docs/architecture/modules.md.
package devkit

import (
	"embed"
	"io/fs"
)

//go:embed README.md principles docs kit
var assets embed.FS

// FS returns the embedded assets.
func FS() fs.FS {
	return assets
}
