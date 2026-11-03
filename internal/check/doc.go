// check は入出力を伴わない観点ファイルの解析、判定依頼の内容の決定、結果の受理・集約を担う。
// 適用除外・必須 check・検出試験の照合を扱い、コマンド実行は engine、保存は ledger、次の判断は router に置く。
// 観点ファイルの形式と判定結果のスキーマを扱う form に依存する。
// 正本: docs/architecture/modules.md#check
package check
