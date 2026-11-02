# ledger — 状態の外部保存

ledger は、作業ごとの実行の記録を構造化データとして保持し、記録の型と派生値への問い合わせ、決定と承認の解決を提供する。責務と依存は [ledger の契約](../architecture/modules.md#ledger)に従う。

## 入力

記録の共通項目と種別ごとの参照は[記録の範囲](../architecture/data.md#記録の範囲)、ペイロードは [ledger の持つデータ](../architecture/modules.md#ledger)に従う。ペイロードには plan・check・router・form の型を用い、保存の操作には ledger/fsstore の具体型を受け取る。

### 作業

[Work](../architecture/data.md#作業-work)を記録し、課題と作業範囲の記録を結び付ける。

### 計画

[計画](../architecture/data.md#計画-plan)の各バージョンの提案と撤回、承認を別の記録として保持する。実行可能性は[計画のバージョン](../architecture/data.md#計画のバージョン)に従って導く。

### 決定

途中の判断と調査結果は `devkit note` で受け付ける[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)として保持し、決定の有効性・置換・依存を解決する。決定の状態と確認済みの集合は、`devkit answer` で追記する状態変更から導く。

調査中の記録順は [skill の手順](../architecture/contract-changes.md#skill-の手順)に従う。

### 問い・回答・承認

[問い](../architecture/data.md#問い-question)と、それに対する[回答・承認](../architecture/data.md#回答承認-answer--approval)を保持し、回答には観測した入力の本文と出所を残す。同一性・訂正・競合の解決は回答・承認の定義、受理の条件と期限は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

### ステージの実行

開始時の計画のバージョンに束縛した [StageRun](../architecture/data.md#ステージの実行-stagerun)の実行状態と観測時刻を保持する。実行状態の種類は StageRun の定義、完了記録のコミットのハッシュは[完了条件](../architecture/guarantees.md#完了条件)に従う。

### ラウンドと指示

[Round](../architecture/data.md#ラウンド-round)には発行した指示 ID の集合を結び付け、指示の出し直しも記録する。指示 ID の導出と本文の扱いは [Instruction](../architecture/data.md#指示-instruction)、判定者・計画者の結果ファイルの命名と来歴は [SubagentStop](../architecture/hooks.md#subagentstop)に従う。

### 進捗

`devkit submit` が受け付けた終了ブロックを[進捗](../architecture/data.md#進捗-progress)として保持する。入力の場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

### check の結果

`PASS` / `FAIL` / `NOT ASSESSED` の判定と指摘を [Judgment](../architecture/data.md#check-の結果-judgment)として保持する。対象のバージョンと規範の識別による有効性・失効は Judgment の定義、結果の受理と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

### 検出試験

ledger は検出試験の実行の事実を保持する。対象とする観点ファイル、合格記録の正本である `.devkit/detect.json` の索引との区別、記録の同一性と来歴は [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)に従う。

### 外部への書き込み

外部書き込みの冪等キーと実行前後の記録、結果は [ExternalOp](../architecture/data.md#外部への書き込み-externalop)に保持する。承認との結び付けは[外部操作の承認](../architecture/guarantees.md#外部操作の承認)、再送時の解決は[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。

### 成果物

`devkit` が開始時と返却後のスナップショットの差分から生成し、永続化したコミットへの参照を持つ[成果物の記録](../architecture/data.md#成果物-artifact-record)を保持する。view の網羅性は、この[成果物を記録した範囲](../architecture/modules.md#view)から導く。

## 出力

### checkpoint

再開用の checkpoint を記録から組み立て、SessionStart での注入に提供する。一時状態の無効条件の照合を含む組み立ては [checkpoint](../architecture/data.md#checkpoint)に従う。

### 状態の問い合わせ

有効な決定とその状態、計画の実行可能なバージョンと提案の状態、観点ごとの最新判定とバージョンの一致、完了記録のない StageRun、未回答の問い、承認の解決、確認済みの集合、未受理の指示を派生値として返す。問い合わせの範囲は [ledger の公開する契約](../architecture/modules.md#ledger)に従う。

## 保証

- **参照と互換性**: 共通項目と種別ごとの参照は[記録の範囲](../architecture/data.md#記録の範囲)、記録形式の互換性は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従う。
- **保存と書き手**: CLI による実行の記録と hook による生成物を区別し、保存・公開・排他は [ledger/fsstore](../architecture/modules.md#ledgerfsstore)に委ねる。書き込みの境界は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、Stop hook の生成物は [Stop](../architecture/hooks.md#stop)に従う。
- **生存期間**: 完了・中止後の記録と、人の判断の追記は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

## 育てるものとの境界

構造は [Structure の `.devkit/structure.json`](../architecture/data.md#構造の記録-structure)、規則と強制手段は [Rules の `.devkit/rules.md`](../architecture/data.md#規則と強制手段-rules)としてリポジトリが育てるものに置く。固定物・育てるもの・実行の記録・生成物の区別は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。

## 試験項目

- [記録の範囲](../architecture/data.md#記録の範囲)に従う参照の整合と、決定の有効性・置換・依存の解決。
- [計画のバージョン](../architecture/data.md#計画のバージョン)と承認の組み合わせによる状態、および [checkpoint](../architecture/data.md#checkpoint)・確認済みの集合・未受理の指示の派生。
- [回答・承認](../architecture/data.md#回答承認-answer--approval)の再送・訂正・競合、古い問い、完了後の更新、観測の削除後にも残る本文と出所。
- [指示 ID](../architecture/data.md#指示-instruction)の作業・観点・計画・提案・検出試験ごとの差、出し直し、ラウンド前の識別、本文の扱い。
- [Judgment](../architecture/data.md#check-の結果-judgment)の対象のバージョンと規範の識別による有効性、および [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)の実行の事実と合格記録の索引の区別。
- [成果物による網羅性](../architecture/modules.md#view)、[観測時刻による実行状態](../architecture/data.md#ステージの実行-stagerun)、完了記録と永続化したコミットの対応。
