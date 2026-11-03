// form は入出力を伴わない固定フォーマットの定義、受け取ったスキーマ・テンプレートの適用、Markdown の描画を担う。
// 固定物の読み込みは kit、計画と判定の意味論は plan と check、HTML の生成は view に置く。
// 他のモジュールに依存せず、スキーマとテンプレート本体は kit から供給された値を使う。
// 正本: docs/architecture/modules.md#form
package form
