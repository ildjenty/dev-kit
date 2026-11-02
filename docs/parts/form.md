# form — 固定フォーマット

## 責務

form は固定フォーマットの型と、[スキーマ・テンプレート](../architecture/modules.md#form)(入力形式の定義と文面の雛形)の適用、Markdown の描画を提供する。

## 入力と出力

[kit](../architecture/modules.md#kit)(固定物の供給)が渡すスキーマとテンプレート、および適用・描画する値を受け取り、固定フォーマットの定義、適用結果、人向けの Markdown を返す。

### 固定フォーマット

- **終了ブロック**: `devkit submit` に渡す[終了ブロック](../architecture/data.md#進捗-progress)(ラウンドの進捗)の JSON Schema を適用する。
- **判定者の結果**: [判定者](../architecture/data.md#計画-plan)(評価を担う実行主体)のサブエージェントが最終メッセージで返す JSON にスキーマを適用する。形式は [check の出力](check.md#出力)、項目は[判定結果](../architecture/data.md#check-の結果-judgment)(観点から対象を判定した記録)と[指摘](../architecture/data.md#指摘-finding)に従う。
- **問い**: [問い](../architecture/data.md#問い-question)(人への確認事項)の JSON Schema を適用する。
- **指示**: `devkit next` が返す[指示](../architecture/data.md#指示-instruction)(エージェントへの依頼)の JSON Schema を適用する。
- **決定と証拠**: `devkit note` で渡す[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)(判断とその根拠)の JSON Schema を適用する。

#### 問い

問いの文面に添える回答の案内は[問いの定義](../architecture/data.md#問い-question)に従う。選択肢による回答と文章による回答の受理、および訂正期限は[回答の受理](../architecture/guarantees.md#回答の受理)、停止する範囲と進められる範囲の表示は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

画面のデザインの承認対象は、画面 ID(画面の識別子)と表示内容を含む[ツリーのスナップショット](../architecture/data.md#対象のバージョン-targetversion)(対象ファイルの固定時点の内容)とし、画面 ID は問いの説明に示す。

### テンプレート

- **skill の構成**: [skill](../architecture/layout.md#指示ファイルとホスト設定)(devkit の操作手順)の構成のテンプレートを適用する。コマンドの使い方の配置は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。
- **観点ファイル**: [観点ファイル](../architecture/modules.md#check)(判定の確認事項)のテンプレートを適用する。項目はリンク先の定義、例は[観点と検出試験](../architecture/examples.md#観点と検出試験)を参照する。
- **PR 本文**: 検証の節を持つテンプレートを適用する。節の構成は [form の契約](../architecture/modules.md#form)、共有する情報は[資産の更新と共有](../architecture/data.md#資産の更新と共有)、記載内容は [`principles/writing.md`](../../principles/writing.md) の原則1に従う。
- **設計案**: 供給されたテンプレートを適用し、元に戻せないもの(変更困難な決定)の一覧、または「該当なし」を含める。判断は [`principles/designing.md` の原則1](../../principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。

## 保証

形式と意味論の境界、および供給された定義の扱いは [form の契約](../architecture/modules.md#form)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

雛形と計画のスキーマは form の対象に含めず、[plan](../architecture/modules.md#plan)(計画の意味と検証)で扱う。

## 試験項目

試験の範囲は[正本の試験項目](../architecture/contract-changes.md#試験項目)に従う。

- 雛形・計画のスキーマを form の適用対象に含めないこと。
