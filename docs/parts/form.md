# form — 固定フォーマット

## 責務

form は固定フォーマットの型と、[スキーマ・テンプレート](../architecture/modules.md#form)(入力形式の定義と文面の定型)の適用、Markdown の描画を提供する。

## 入力と出力

[kit](../architecture/modules.md#kit)(固定物の供給)が渡すスキーマとテンプレート、および適用・描画する値を受け取り、固定フォーマットの定義、適用結果、人向けの Markdown を返す。

### 固定フォーマット

スキーマの適用対象と形式は [form の定義](../architecture/modules.md#form)、各データの項目は[エンティティの定義](../architecture/data.md#エンティティ)に従う。[判定者](../architecture/data.md#計画-plan)(評価を担うロール)の結果の項目は、[判定結果](../architecture/data.md#check-の結果-judgment)(観点から対象を判定した記録)と[指摘](../architecture/data.md#指摘-finding)(失敗や判断を要する点)に従う。

#### 問い

問い(人への確認事項)の文面に添える回答の案内は[問いの定義](../architecture/data.md#問い-question)に従う。選択肢による回答と文章による回答の受理、および訂正期限は[回答の受理](../architecture/guarantees.md#回答の受理)、停止する範囲と進められる範囲の表示は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

画面のデザインの承認時の表示要件は[問いの定義](../architecture/data.md#問い-question)に従う。

### テンプレート

テンプレートの対象と PR 本文の節の構成は [form の定義](../architecture/modules.md#form)に従う。

skill(devkit の操作手順)の配置は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。[観点ファイル](../architecture/modules.md#check)(判定の確認事項)の例は[観点と検出試験](../architecture/examples.md#観点と検出試験)を参照する。

PR 本文で共有する情報は[資産の更新と共有](../architecture/data.md#資産の更新と共有)、記載内容は [`principles/writing.md` の原則1](../../principles/writing.md#1-結論と人の判断に必要なことを先に書く)に従う。

設計案には供給されたテンプレートを適用し、元に戻せないもの(変更困難な決定)の一覧、または「該当なし」を含める。判断は [`principles/designing.md` の原則1](../../principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。

## 保証

形式と意味論の境界、および供給された定義の扱いは [form の契約](../architecture/modules.md#form)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

計画の検証との責務の境界は [plan の契約](../architecture/modules.md#plan)(計画の意味論を扱う実装)に従う。

## 試験項目

試験の範囲は正本の [form の試験項目](../architecture/contract-changes.md#form-1)に従う。

- [ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(再利用するループの定義)・[計画](../architecture/data.md#計画-plan)(課題固有のループのデータ)のスキーマを form の適用対象に含めないこと。
