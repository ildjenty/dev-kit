// event はホストのイベント JSON を共通イベント型に変換し、ホストの上限と対応表に合わせて出力 JSON を整形する。
// ホスト形式の変換を置き、イベントごとの判断は hook に置く。
// 他のモジュールに依存しない。
// 正本: docs/architecture/modules.md#hostevent
package event
