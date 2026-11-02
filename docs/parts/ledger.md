# ledger — 状態の外部保存

## 責務

ledger は、作業ごとの[実行の記録](../architecture/data.md#所有と生存期間)(作業が所有する履歴)を扱う。記録の意味論と他のモジュールとの分担は [ledger の契約](../architecture/modules.md#ledger)に従う。

## 入力と出力

作業ごとの記録を受け取り、記録の取得と[派生値](../architecture/modules.md#ledger)(記録から導く状態)の問い合わせに結果を返す。記録の型と種別、公開する操作と派生値、決定と承認の解決は [ledger の契約](../architecture/modules.md#ledger)、共通項目と種別ごとの参照は[記録の範囲](../architecture/data.md#記録の範囲)に従う。

### 作業

作業の単位と識別は [Work](../architecture/data.md#作業-work)(課題単位のループ)の定義に従う。

### 計画

[計画](../architecture/data.md#計画-plan)(課題ごとの実行内容)に関する記録の分離と実行可能性の導出は[計画のバージョン](../architecture/data.md#計画のバージョン)に従う。

### 決定

`devkit note` の入力と決定の状態、[確認済みの記録の集合](../architecture/data.md#決定と証拠-decision--evidence)の導出は[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)(判断と調査結果)の定義に従う。

### 問い・回答・承認

[問い](../architecture/data.md#問い-question)(人への確認事項)と[回答・承認](../architecture/data.md#回答承認-answer--approval)(問いへの判断)の保存項目と同一性・訂正・競合の解決は各定義、受理の条件と期限は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

### ステージの実行

記録項目と計画のバージョンへの束縛は [StageRun](../architecture/data.md#ステージの実行-stagerun)(ステージの実行の記録)、完了記録は[完了条件](../architecture/guarantees.md#完了条件)に従う。

### ラウンドと指示

記録内容と指示 ID の導出・出し直し・本文の扱いは [Round](../architecture/data.md#ラウンド-round)(作業依頼から判断までの往復)と [Instruction](../architecture/data.md#指示-instruction)(エージェントへの指示)の定義に従う。

### 進捗

[終了ブロック](../architecture/data.md#進捗-progress)(ラウンドの作業報告)の記録は[進捗](../architecture/data.md#進捗-progress)、入力の場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

### check の結果

結果の受理と記録の分担は [check](../architecture/modules.md#check)(観点ごとの判定を扱う部品)と [ledger の契約](../architecture/modules.md#ledger)に従う。判定値と項目、有効性・失効は [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、結果の受理と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

### 検出試験

合格記録の正本との分離、記録の同一性と来歴は [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)(観点ファイルの判定能力の試験記録)の定義に従う。

### 外部操作の記録

[外部操作](../architecture/flow.md#外部操作)(リポジトリ外への書き込み)の記録項目は [ExternalOp](../architecture/data.md#外部への書き込み-externalop)(操作の実行記録)、承認との結び付けは[外部操作の承認](../architecture/guarantees.md#外部操作の承認)、再送時の解決は[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。

### 成果物

生成方法と参照項目は[成果物の記録](../architecture/data.md#成果物-artifact-record)(変更と永続化先の対応)の定義に従う。

### checkpoint

終了ブロックの[一時状態](../architecture/data.md#進捗-progress)(無効になる条件を持つ情報)の照合を含む現在地の組み立ては [checkpoint](../architecture/data.md#checkpoint)(再開用の現在地)、注入との接続は [SessionStart](../architecture/hooks.md#sessionstart)(起動・再開などのイベント)に従う。

### 状態の問い合わせ

問い合わせ可能な派生値は [ledger の公開する契約](../architecture/modules.md#ledger)に従う。

## 保証

- **互換性**: 記録形式の互換性は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従う。
- **保存と書き手**: 保存・公開・排他の責務分離は [ledger](../architecture/modules.md#ledger) と [ledger/fsstore](../architecture/modules.md#ledgerfsstore)(ファイル保存を担うモジュール)の契約、書き込みの境界は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、hook の[生成物](../architecture/data.md#所有と生存期間)(削除や再生成が可能なデータ)は [hook](../architecture/hooks.md)に従う。
- **生存期間**: 完了・中止後の記録と、人の判断の追記は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

### 育てるものとの境界

[構造](../architecture/data.md#構造の記録-structure)(リポジトリの構造の資産)と[規則と強制手段](../architecture/data.md#規則と強制手段-rules)(設計上の決定と守る手段)の分類は、[育てるもの](../architecture/data.md#所有と生存期間)(リポジトリが git で管理する資産)を定める「所有と生存期間」と各定義に従う。

## 試験項目

試験は正本の[試験項目](../architecture/contract-changes.md#試験項目)に従う。
