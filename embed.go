// devkit embeds the distributed assets, principles and kit, as an fs.FS.
// The design documents in docs are for developing dev-kit and are not distributed.
// It does not depend on implementation packages; internal/kit supplies and verifies assets.
// Source of truth: the root package section in docs/architecture/modules.md.
package devkit

import (
	"embed"
	"io/fs"
)

//go:embed principles kit
var assets embed.FS

// FS returns the embedded assets.
func FS() fs.FS {
	return assets
}
