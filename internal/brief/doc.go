// brief は渡されたステージ定義・計画・前の出力・リポジトリの事実から brief と依頼文を組み立てる。
// 判定依頼の内容は check が決め、ファイルへの書き出しは engine に置く。自ら入出力は行わない。
// ステージの型を持つ plan とテンプレートを扱う form に依存する。
// 正本: docs/architecture/modules.md#brief
package brief
