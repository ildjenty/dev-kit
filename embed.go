// devkit は principles・docs・kit の固定物を埋め込み、fs.FS として公開する。
// 実装パッケージに依存せず、固定物の供給と照合は internal/kit に置く。
// 正本: docs/architecture/modules.md#ルートパッケージ
package devkit

import (
	"embed"
	"io/fs"
)

//go:embed principles docs kit
var assets embed.FS

// FS は埋め込んだ固定物を返す。
func FS() fs.FS {
	return assets
}
