# form — 固定フォーマット

## 責務

form は固定フォーマットの型と、[スキーマ・テンプレート](../architecture/modules.md#form)(入力形式の定義と文面の定型)の適用、Markdown の描画を提供する。

## 入力と出力

[kit](../architecture/modules.md#kit) がバイナリから供給するスキーマ、リポジトリが所有するテンプレート、および適用・描画する値を受け取り、固定フォーマットの定義、適用結果、人向けの Markdown を返す。

### 固定フォーマット

スキーマの適用対象と形式は [form の定義](../architecture/modules.md#form)、各データの項目は[エンティティの定義](../architecture/data.md#エンティティ)に従う。[判定者](../architecture/data.md#計画-plan)(評価を担うロール)の結果の項目は、[判定結果](../architecture/data.md#check-の結果-judgment)(観点から対象を判定した記録)と[指摘](../architecture/data.md#指摘-finding)(失敗や判断を要する点)に従う。

終了ブロックのスキーマは[進捗](../architecture/data.md#進捗-progress)の必須項目を含めて定義する。入力ファイルの場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

単位の作業者 `devkit-worker` の最終メッセージにも、`schemas/progress.json` の終了ブロックを使う。単位の結果は SubagentStop が保存し、合流後は単位の終了ブロックをまとめた作業者の終了ブロックをステージのラウンドとして扱う。

テンプレートの既定の置き場所は `.devkit/templates/` であり、`devkit.toml` の `[paths] templates` で変更できる。JSON Schema はバイナリと記録の約束であり、リポジトリで育てる対象にはしない。

エージェントに渡すスキーマは、すべての `$ref` の参照先を同梱して単独で解釈できる形にする。作業の brief には note の種別一覧と各種別の同梱済みスキーマも渡す。構造の記録は `schemas/structure.json` に従う。

#### 問い

問い(人への確認事項)の文面に添える回答の案内は[問いの定義](../architecture/data.md#問い-question)に従う。回答の受理と訂正は[回答の受理](../architecture/guarantees.md#回答の受理)、停止しているステージと理由の表示は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

計画承認の問いは、要求の原文、`start` の課題、ID 付きの文章で書かれた完了条件、採用・除外したステージと逸脱、各ステージの必須 check、課題固有の元に戻せないもの、外部操作、任意の文字列配列 `open_questions` を記録から描画する。完了条件ごとの確認手段は[計画](../architecture/data.md#計画-plan)に従う。未決事項に対する人の回答は、計画の却下と作り直しの指示として計画者に渡す。

画面のデザインの承認時の表示要件は[問いの定義](../architecture/data.md#問い-question)に従う。

ゲートの問いは、計画のゲート理由、終了ブロックの実施内容・確認済み事項・未確認事項、差分ファイルのパスを記録から描画する。ループ自身の変更の問いは、変更した規範・ホスト設定・skill のファイル一覧と差分ファイルへの参照を描画する。エージェントの要約には依存しない。

### テンプレート

テンプレートの対象と PR 本文に含める情報は [form の定義](../architecture/modules.md#form)に従う。

[観点ファイル](../architecture/modules.md#check)(判定の確認事項)の例は[観点と検出試験](../architecture/examples.md#観点と検出試験)を参照する。

PR 本文で共有する情報は[資産の更新と共有](../architecture/data.md#資産の更新と共有)、記載内容は [`assets/principles/writing.md` の原則1](../../assets/principles/writing.md#1-結論と人の判断に必要なことを先に書く)に従う。

設計案には供給されたテンプレートを適用し、元に戻せないもの(変更困難な決定)は[計画の一覧](../architecture/data.md#計画-plan)に従う。

## 保証

形式と意味論の境界、および供給された定義の扱いは [form の契約](../architecture/modules.md#form)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

計画の検証との責務の境界は [plan の契約](../architecture/modules.md#plan)(計画の意味論を扱う実装)に従う。
