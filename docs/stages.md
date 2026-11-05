# ステージ

計画のステージは実体 ID と定義 ID を持ち、同じ定義を増分ごとに使える。[計画](architecture/data.md#計画-plan)に従って入力・報告・check・遷移を組み、[ループの決まり](architecture/data.md#ループの決まり)を満たす。定義は `stages` ソースから選ぶ。

報告と確認要求の形式は[進捗](architecture/data.md#進捗-progress)、入力・経緯・採用された成果は[依頼の入力](architecture/data.md#依頼の入力)、後の作業へ残す決定は[資産の更新と共有](architecture/data.md#資産の更新と共有)に従う。

## 0 目的

`purpose` のヒアリングと要件の合意は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/purpose.md`](../assets/stages/purpose.md) に置く。

## 1 スコープ

`scope` の外部設計と範囲の合意は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/scope.md`](../assets/stages/scope.md) に置く。

## 2 アーキテクチャ

`architecture` の方式と技術選定は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/architecture.md`](../assets/stages/architecture.md) に置く。

## 3 増分

`increments` のリリースの分割と再計画は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/increments.md`](../assets/stages/increments.md) に置く。増分を組むことと、[1ステージ内の単位への分割](architecture/examples.md#分割の雛形と計画)は別の指定である。

## 4 詳細設計

`design` の増分の詳細化と判断の確認は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/design.md`](../assets/stages/design.md) に置く。

## 5 実装

`implement` の設計に沿った実装は[既定の開発フロー](architecture/flow.md#既定の開発フロー)、検証コマンドの用意と登録は[作業結果の判定](architecture/flow.md#作業結果の判定)に従う。定義は [`assets/stages/implement.md`](../assets/stages/implement.md) に置く。

## 文書の編集

定義 ID は `edit` である。文書の内容と表現を編集し、wording の check で確かめる。docs のパターンは edit と retrospective を参考の並びとする。implement の定義には実装用の必須 check の決まりが適用されるため、文書編集は edit の定義を使う。

## 検証

`verification` の増分のデモと acceptance は[既定の開発フロー](architecture/flow.md#既定の開発フロー)に従う。定義は [`assets/stages/verification.md`](../assets/stages/verification.md) に置く。

## 調査

- **決めること**: 起きている事象、動作の仕組み、既存の同種の処理の書き方。
- **決めずに次のステージへ渡すこと**: 修正方法。
- **元に戻せないもの**: なし。
- **成果物**: 調査結果を、観測と推論に分けた報告。

## 試作

- **決めること**: 未確認の前提の真偽。
- **決めずに次のステージへ渡すこと**: 本番のコード。
- **元に戻せないもの**: なし。
- **成果物**: 実験の結果と決定。決定はワーカーなら終了ブロックの `notes`、セッション自身なら [`devkit note`](architecture/data.md#決定と証拠-decision--evidence) で記録する。試作は本番のコードから分離し、[`devkit submit`](architecture/flow.md#作業結果の提出)(作業結果を提出するコマンド)の実行前に削除するか、追跡対象外で ignore された場所に置いて[完了時のコミット](architecture/guarantees.md#完了条件)に含めない。

## 引き継ぎ

ステージ定義 ID は `handoff` であり、既定のパターンの候補だけに置く。作業者が既定 `.devkit/templates/pr.md.tmpl` に沿ってレビューする人向けに PR 本文を書く。テンプレートの配置は設定で変更できる。判定者が wording などの基準で確かめる。git の操作は含めない。

## 振り返り

`retrospective` で問題・見逃しと規範の働きを見直す。[教訓の反映先](architecture/guarantees.md#教訓の反映先)と[線引きの更新](architecture/guarantees.md#線引きの更新)に従う。履歴の固定と必須 check は[ループの決まり](architecture/data.md#ループの決まり)、定義は [`assets/stages/retrospective.md`](../assets/stages/retrospective.md) に置く。
