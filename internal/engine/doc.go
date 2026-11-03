// engine は状態機械の進行、指示の計算、判定・検証コマンドの実行、結果の受理と記録、brief の書き出しを担う。
// git 操作は workspace、外部書き込みは external、保存は ledger/fsstore に置き、サブエージェントは起動しない。
// plan・brief・check・router・ledger・form・workspace・external・ledger/fsstore に依存する。
// 正本: docs/architecture/modules.md#engine
package engine
