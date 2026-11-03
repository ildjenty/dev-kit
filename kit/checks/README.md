kit が提供する観点ファイルを置く。
解析・判定の意味論は internal/check、検出試験の例は kit/examples/、利用側の観点ファイルは .devkit/checks/ に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、観点の扱いは [check](../../docs/architecture/modules.md#check) に従う。

観点ファイルは `<観点ID>.md` とし、先頭行の `# <観点ID>` に続けて、`- 確認事項: `、`- blocking: `、`- 報告しない事項: `、`- 出力形式: ` をこの順で各1行に記す。値は行末までの Markdown とし、項目名と先頭行から機械で読み取る。内容の正本は `docs/checks.md` の同名の観点であり、変更時は対応する観点と `kit/examples/<観点ID>/` の例を併せて更新する。`docs/`・`principles/` で始まるコード表記の参照パスは、開発時はリポジトリのルート、導入後は kit のコピーのルートを基準に解決する。検出試験の形式は [検出試験の例](../examples/README.md) に従う。

- `purpose.md`: 目的。
- `irreversible.md`: 元に戻せないもの。
- `structure.md`: 構造。
- `consistency.md`: 整合。
- `wording.md`: 校閲。
- `contract.md`: 契約。
- `steps.md`: 手順。
- `diff.md`: 差分。
- `behavior.md`: 動作検証。
- `completion.md`: 終了時の状態。
- `plan.md`: 計画。
- `premise.md`: 前提の批評。
- `harness.md`: harness。
- `skill-test.md`: skill の試験。
