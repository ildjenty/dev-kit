// fsstore は記録と生成物の保存形式・公開・破損の検出と作業の排他制御を担う。
// 渡された記録ルートの配下だけを扱い、記録の意味論は ledger、ルートの解決は cmd/devkit に置く。
// 他のモジュールに依存しない。
// 正本: docs/architecture/modules.md#ledgerfsstore
package fsstore
