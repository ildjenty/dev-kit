# ledger — 状態の外部保存

## 責務

ledger は、作業ごとの[実行の記録](../architecture/data.md#所有と生存期間)(作業が所有する履歴)を構造化データとして保持し、記録の型と派生値への問い合わせ、決定と承認の解決を提供する。責務と依存は [ledger の契約](../architecture/modules.md#ledger)に従う。

## 入力と出力

作業ごとの記録を受け取り、記録の取得と派生値の問い合わせに結果を返す。共通項目と種別ごとの参照は[記録の範囲](../architecture/data.md#記録の範囲)、受け取る記録の種類は [ledger の持つデータ](../architecture/modules.md#ledger)に従う。

### 作業

[Work](../architecture/data.md#作業-work)(課題単位のループ)を受け取り、作業の記録として保持する。

### 計画

[計画](../architecture/data.md#計画-plan)(課題ごとの実行内容)に関する記録を受け取り、実行可能性を返す。記録の分離と実行可能性の導出は[計画のバージョン](../architecture/data.md#計画のバージョン)に従う。

### 決定

`devkit note` で受け付ける[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)(判断と調査結果)を保持し、決定の有効性・置換・依存の解決を返す。決定の状態と[確認済みの記録の集合](../architecture/data.md#決定と証拠-decision--evidence)(人が閲覧して判断した記録)の導出も、同じ定義に従う。

### 問い・回答・承認

[問い](../architecture/data.md#問い-question)(人への確認事項)と[回答・承認](../architecture/data.md#回答承認-answer--approval)(問いへの判断)を受け取り、保持する。保存項目と同一性・訂正・競合の解決は各定義、受理の条件と期限は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

### ステージの実行

[StageRun](../architecture/data.md#ステージの実行-stagerun)(ステージの実行の記録)を受け取り、保持する。記録項目と計画のバージョンへの束縛は StageRun の定義、完了記録は[完了条件](../architecture/guarantees.md#完了条件)に従う。

### ラウンドと指示

[Round](../architecture/data.md#ラウンド-round)(作業依頼から判断までの往復)と、[Instruction](../architecture/data.md#指示-instruction)(エージェントへの指示)に関する記録を受け取り、保持する。記録内容と指示 ID の導出・出し直し・本文の扱いは各定義に従う。

### 進捗

`devkit submit` が受け付けた[終了ブロック](../architecture/data.md#進捗-progress)(ラウンドの作業報告)を進捗として保持する。入力の場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

### check の結果

[Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)を受け取り、保持する。判定値と項目、有効性・失効は Judgment の定義、結果の受理と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

### 検出試験

[DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)(観点ファイルの判定能力の試験記録)を受け取り、保持する。合格記録の正本との分離、記録の同一性と来歴は同じ定義に従う。

### 外部操作の記録

[ExternalOp](../architecture/data.md#外部への書き込み-externalop)(外部操作の記録)を受け取り、保持する。記録項目は同じ定義、承認との結び付けは[外部操作の承認](../architecture/guarantees.md#外部操作の承認)、再送時の解決は[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。

### 成果物

[成果物の記録](../architecture/data.md#成果物-artifact-record)(変更と永続化先の対応)を受け取り、保持する。生成方法と参照項目は同じ定義に従う。

### checkpoint

記録から [checkpoint](../architecture/data.md#checkpoint)(再開用の現在地)を組み立て、SessionStart での注入に返す。終了ブロックの[一時状態](../architecture/data.md#進捗-progress)(無効になる条件を持つ情報)の照合を含む組み立ては checkpoint の定義に従う。

### 状態の問い合わせ

記録から導いた状態を派生値として返す。問い合わせ可能な派生値の一覧は [ledger の公開する契約](../architecture/modules.md#ledger)に従う。

## 保証

- **互換性**: 記録形式の互換性は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従う。
- **保存と書き手**: 保存・公開・排他は [ledger/fsstore](../architecture/modules.md#ledgerfsstore)(ファイル保存を担うモジュール)に委ねる。書き込みの境界は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、hook の[生成物](../architecture/data.md#所有と生存期間)(削除や再生成が可能なデータ)は [hook](../architecture/hooks.md)に従う。
- **生存期間**: 完了・中止後の記録と、人の判断の追記は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

### 育てるものとの境界

[構造](../architecture/data.md#構造の記録-structure)と[規則と強制手段](../architecture/data.md#規則と強制手段-rules)は ledger の記録に含めず、[育てるもの](../architecture/data.md#所有と生存期間)(リポジトリが git で管理する資産)として扱う。

## 試験項目

試験は正本の[試験項目](../architecture/contract-changes.md#試験項目)に従う。
