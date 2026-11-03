// hook はホストの5つのイベントを決定的に処理し、観測と結果を生成物にする。
// モデルによる判定は置かず、ルート・作業 ID・規範の設定を読む関数値は cmd/devkit から受け取る。
// ledger・config・host/event・form・ledger/fsstore に依存する。
// 正本: docs/architecture/modules.md#hook
package hook
