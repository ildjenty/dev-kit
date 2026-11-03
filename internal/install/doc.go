// install は init による導入先の構成と設定の追記・登録、doctor による照合を担う。
// 既存の設定・skill を上書きせず冪等に扱い、固定物の供給・コピー・一致の照合は kit に置く。
// config・kit・ledger・workspace に依存する。
// 正本: docs/architecture/modules.md#install
package install
