brief の組み立てに渡すステージ定義を置く。
計画の意味論は internal/plan、brief の組み立ては internal/brief、進行処理は internal/engine に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、組み立ては [brief](../../docs/architecture/modules.md#brief) に従う。

ステージ定義は `<ステージID>.md` とし、先頭の `# <名前>` と、`## 入力`、`## 決めること`、`## 決めずに次のステージへ渡すこと`、`## 元に戻せないもの`、`## 成果物`、`## 終わる条件` をこの順で持つ Markdown とする。ファイル名が ID を表し、各節は次の第2階層見出しまでを値として機械で読み取る。内容の正本は `docs/stages.md` の同名のステージであり、変更時は対応する定義も更新する。`docs/`・`principles/` で始まるコード表記の参照パスは、開発時はリポジトリのルート、導入後は kit のコピーのルートを基準に解決する。入力は `docs/parts/brief.md`、終了の共通条件は `docs/architecture/guarantees.md#完了条件` に従い、採用するステージ・順序・check・runner・ゲート・上限・遷移はループの雛形と計画で決める。

- `purpose.md`: 0 目的。
- `scope.md`: 1 スコープ。
- `structure.md`: 2 構造。
- `contract.md`: 3 契約。
- `steps.md`: 4 手順。
- `implement.md`: 5 実装。
- `research.md`: 調査。
- `prototype.md`: 試作。
- `initialize.md`: 初期化。
