// workspace は git CLI によるブランチ・スナップショット・コミットの操作、展開、変更や履歴の取得、ルートの解決を担う。
// git 操作と出力の内部型への変換を置き、worktree の扱いはホスト・git と kit の責務分担に従う。
// 他のモジュールに依存せず、外部の git CLI を使う。
// 正本: docs/architecture/modules.md#workspace
package workspace
