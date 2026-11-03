// external は kit が管理する外部書き込みの唯一の実行経路として、承認・実行前後の記録・実行・結果不明からの回復を担う。
// 種別ごとの実装は持たず、登録された定型コマンドをチェックアウトで呼ぶ。
// ledger に依存し、コマンドの登録は config が読み込んだ値として受け取る。
// 正本: docs/architecture/modules.md#external
package external
