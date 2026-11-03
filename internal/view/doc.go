// view は ledger と解析済みの構造の値から、1作業を人に表示する静的 HTML を生成する。
// サーバや HTML からの状態変更は置かず、構造の履歴は cmd/devkit が workspace で取得して渡す。
// ledger・問いを描画する form・ステージ名を持つ plan に依存する。
// 正本: docs/architecture/modules.md#view
package view
