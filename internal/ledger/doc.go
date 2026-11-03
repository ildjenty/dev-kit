// ledger は記録の型と意味論、問い合わせ、派生値、決定と承認の解決を担う。
// ファイル形式は ledger/fsstore、表示は view に置く。
// ペイロードの型を持つ plan・check・router・form と、保存を担う ledger/fsstore に依存する。
// 正本: docs/architecture/modules.md#ledger
package ledger
