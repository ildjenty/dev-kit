// router は渡された分類表と判定の集約・進展・上限などから次の行動を選び、完了条件と人への確認を扱う。
// 入出力を伴わない分類と判断を置き、判断に従うループの進行は engine に置く。
// 上限と遷移の語彙を持つ plan と判定の型を持つ check に依存する。
// 正本: docs/architecture/modules.md#router
package router
