# check の観点ファイル

## 目的

- **[ステージ](stages.md)**: 0
- **確認事項**: 目的を1文で説明できるか。完了を判定できるか。
- **blocking**: 完了を判定できない。

## 不可逆な決定

- **ステージ**: 1
- **確認事項**: 元に戻せないものを「後で直せる」と扱っていないか。一覧に漏れがないか。
- **blocking**: 一覧に漏れがある。

## 構造

- **ステージ**: 2
- **確認事項**: 各配置を責務に基づいて正当化できるか。同じ目的をより単純に満たす構造を1つ作り、元の案の余分な部分を示す。[`principles/design.md`](../principles/design.md) の各原則の違反・兆候（浅いモジュール、実行順で分割したモジュール、同じ状態への複数の書き手、同じ処理を行う2つの経路）を確認する。
- **blocking**: 正当化できない配置がある。または、より単純な構造がある。

## 整合

- **ステージ**: 2〜4、文書
- **確認事項**: 文書や設計案の中の矛盾、未定義の事項、用語の揺れ。
- **blocking**: 矛盾がある。未定義の事項と用語の揺れは記録のみとする。

## 校閲

- **ステージ**: 文書
- **確認事項**: [`principles/writing.md`](../principles/writing.md) の原則2・3の違反・兆候と blocking に従う。
- **blocking**: 読み手が意味を取り違える箇所。

## 契約

- **ステージ**: 3
- **確認事項**: 本来あり得ない状態を表現できてしまわないか。失敗の種類を区別できるか。
- **blocking**: あり得ない状態を表現できる。または、失敗の種類を区別できない。

## 手順

- **ステージ**: 4
- **確認事項**: 各ステップを単独で検証できるか。人が一度で読める大きさか。
- **blocking**: 検証できないステップがある。

## 差分

- **ステージ**: 5
- **確認事項**: [`principles/engineering.md`](../principles/engineering.md) の原則1・5 と、[`principles/working.md`](../principles/working.md) の原則12、および各原則の blocking に従う。PR 本文と報告は、[form](parts/form.md) の PR 本文の契約と [規則と強制手段](rules.md#文章と記録)、および [`principles/writing.md`](../principles/writing.md) の原則1・4・9の blocking に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 動作検証

- **ステージ**: 5
- **確認事項**: 起動、操作、証拠の取得、後片付けを確認し、[規則と強制手段](rules.md#画面) の画面の規則に従う。画面はスクリーンショットを画像として見る。コンソールエラー、ページ全体の意図しない横スクロール（要素内の `overflow-x: auto` は許す）、読めない文字、プレースホルダ（[principles/ui.md](../principles/ui.md) の原則10の違反・兆候を参照）を停止条件とする。長い名前、0件、大量のデータ、多言語など、問題が起きやすいデータも使う。
- **blocking**: 停止条件に該当する。確認できなかった場合は `NOT ASSESSED` とする。

## 終了時の状態

- **ステージ**: 5（Stop）
- **確認事項**: [runtime](parts/runtime.md) の終了条件と [`principles/working.md`](../principles/working.md) の原則11に従う。
- **blocking**: 1つでも満たさない。

## 計画

- **ステージ**: 計画
- **確認事項**: 人によるレビューゲートの理由が [router](parts/router.md) の分類に含まれるか。ステージを減らせないか。**各完了条件に対応する [check](parts/check.md) があるか。変更の種類（画面・データ・外部との通信・構造）に必要な検査が含まれるか。**
- **blocking**: 対応する check がない完了条件がある。

## 前提の批評

- **ステージ**: 設計、harness
- **確認事項**: 部品・原則・記録・観点の数と形式を疑い、対案を作って比較する。
- **blocking**: 採用すると設計の形が変わる。

## harness

- **ステージ**: メタ
- **確認事項**: [`principles/harness.md`](../principles/harness.md) の各原則の blocking と、[check](parts/check.md)・[ledger](parts/ledger.md)・[router](parts/router.md)・[runtime](parts/runtime.md) の契約に該当するか。skill に description があるか。参照先が存在するか。check が `NOT ASSESSED` を返せるか。原則が所定の形式を満たすか。skill・playbook・checkpoint に同じ処理の実装が重複していないか（`principles/working.md` の原則5）。Fresh Session Test として、新しいセッションがリポジトリだけを読み、「何のシステムか」「どう動かすか」「どう確かめるか」「今どこか」に答えられるかを確かめる。部品間の接続は runtime の組み立ての試験に従い、保存した checkpoint を別セッションで読み戻し、保存先・読み込み先と版・内容を照合する。
- **blocking**: 答えられない問いがある。harness.md の blocking または部品の契約に違反する。

## skill の試験

- **ステージ**: メタ
- **確認事項**: 依頼と入力だけを渡し、結果を期待結果と照合する。テストケースには「判定できない」場合も含める。評価中だと分かる表現は、試験対象に見せない。
- **blocking**: 期待結果と異なる。

各リポジトリでは、そのリポジトリで繰り返し発生する失敗を、新たな観点として追加できる。
