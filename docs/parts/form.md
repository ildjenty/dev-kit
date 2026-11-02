# form — 固定フォーマット

form は固定フォーマットの型と、スキーマ・テンプレートの適用、Markdown の描画を提供する。形式と意味論の境界は [form の契約](../architecture/modules.md#form)に従う。

## 入力と出力

- **入力**: kit が供給するスキーマとテンプレート、および適用・描画する値を受け取る。スキーマとテンプレート本体の正本と供給は [form](../architecture/modules.md#form)と [kit](../architecture/modules.md#kit)に従う。
- **出力**: 固定フォーマットの定義、スキーマ・テンプレートを適用した結果、人向けの Markdown を返す。

## 固定フォーマット

- **終了ブロック**: `devkit submit` に渡すラウンドの進捗を扱う。項目と終了の種別は[進捗](../architecture/data.md#進捗-progress)に従う。
- **判定者の結果**: 最終メッセージの JSON の形式を扱う。判定と指摘の項目は [Judgment](../architecture/data.md#check-の結果-judgment)と [Finding](../architecture/data.md#指摘-finding)、受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **計画**: ループの雛形と計画の JSON Schema を扱う。内容と例は[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)・[計画](../architecture/data.md#計画-plan)と[雛形と計画の例](../architecture/examples.md#雛形と計画)を参照する。
- **指示**: `next` が返す指示の JSON Schema を扱う。識別と種別は[指示](../architecture/data.md#指示-instruction)に従う。
- **決定と証拠**: `note` で渡す決定と証拠の JSON Schema を扱う。項目は[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)に従う。

### 人への確認事項

問いは、判断に必要な選択肢、推奨案と理由、停止理由、回答後のステージ、範囲、根拠をまとめて示す。項目と回答の定型の案内は[問い](../architecture/data.md#問い-question)、選択肢と文章の回答・訂正期限は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

「停止する範囲」と「進められる範囲」の表示と実際の停止の関係は、[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

## テンプレート

- **skill の構成とコマンドの使い方**: 供給されたテンプレートを適用し、devkit のコマンドの使い方は kit の skill に置く。playbook の skill への統合と配置は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。
- **観点ファイル**: 供給されたテンプレートを適用する。扱う項目は [check の契約](../architecture/modules.md#check)、例は[観点と検出試験](../architecture/examples.md#観点と検出試験)を参照する。
- **PR 本文**: 供給されたテンプレートに計画の要約と共有に必要な決定を含める。共有する情報は[資産の更新と共有](../architecture/data.md#資産の更新と共有)、記載内容は [`principles/writing.md`](../../principles/writing.md) の原則1に従う。
- **設計案**: 供給されたテンプレートを適用し、不可逆な決定の一覧、または「該当なし」を含める。
- **原則**: [`principles/README.md`](../../principles/README.md) の「原則のフォーマット」に従う。

## 保証

form は入出力を行わず、計画や判定の意味論、HTML の生成を担当しない。責務の境界は [form の契約](../architecture/modules.md#form)に従う。

終了ブロック・計画・note の入力 JSON の場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)、成果物の記録は[作業結果の提出](../architecture/flow.md#作業結果の提出)と[成果物](../architecture/data.md#成果物-artifact-record)に従う。

## 試験項目

- 供給されたスキーマ・テンプレートの適用と Markdown の描画。
- 終了ブロック、計画、判定者の結果、指示、問い、決定と証拠の形式との整合。
- 問いの回答案内と、表示する範囲の整合。
