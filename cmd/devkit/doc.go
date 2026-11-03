// main はサブコマンドの定義と各実装の接続を担う。
// ルート・ブランチ・セッション・作業と設定の読み出し元を解決し、設定と kit の値を渡す。
// すべての実装パッケージに依存し、計画・判定・記録の意味論は各モジュールに置く。
// 正本: docs/architecture/modules.md#cmddevkit
package main
