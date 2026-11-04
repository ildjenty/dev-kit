# ledger — 状態の外部保存

## 責務

ledger は、作業ごとの[実行の記録](../architecture/data.md#所有と生存期間)(作業が所有する履歴)を扱う。記録の意味論と他のモジュールとの分担は [ledger の契約](../architecture/modules.md#ledger)に従う。

## 入力と出力

作業ごとの記録を受け取り、記録の取得と[派生値](../architecture/modules.md#ledger)(記録から導く状態)の問い合わせに結果を返す。記録の型と種別、公開する操作と派生値、決定と承認の解決は [ledger の契約](../architecture/modules.md#ledger)、共通項目と種別ごとの参照は[記録の範囲](../architecture/data.md#記録の範囲)に従う。

### 作業

作業の単位と識別は [Work](../architecture/data.md#作業-work)(課題単位のループ)の定義に従う。`kit_version` は記録形式とバイナリの互換性の識別として保存し、バイナリと `init` の設定値は `"v3"` である。`start` に渡した課題と、UserPromptSubmit で観測した人の要求の原文を結び付け、観測がなければ原文なしと記録する。

作業の中断・再開と現在地の関係は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)、受理条件は[中断](../architecture/guarantees.md#中断)に従う。

### 計画

[計画](../architecture/data.md#計画-plan)(課題ごとの実行内容)に関する記録の分離と実行可能性の導出は[計画のバージョン](../architecture/data.md#計画のバージョン)に従う。

### 決定

`devkit note` の受理と決定の状態は[決定と証拠](../architecture/data.md#決定と証拠-decision--evidence)(判断と調査結果)の定義に従う。

自然文の有効条件・適用条件と外部依存の充足を判定する評価関数は受け取らず、条件と参照を決定に添えて返す。承認・却下・保留、置換と記録内の依存関係の解決結果は、条件の充足とは区別する。

### 問い・回答・承認

[問い](../architecture/data.md#問い-question)(人への確認事項)と[回答・承認](../architecture/data.md#回答承認-answer--approval)(問いへの判断)の保存項目は各定義、受理と訂正は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

ループ自身の変更の確認済みの範囲は、確認したスナップショットの規範の識別で表す。計画承認では人が承認した完了条件の ID と文面を比較の基準とする。

確認済みの規範の識別は作業対象と別に保持する。却下しても対象を古いツリーに差し替えず、変更を取り消す作業ラウンドを指示する。ステージの問いへの解釈 `back` は router の戻りの判断として記録する。

### ステージの実行

記録項目と計画のバージョンへの束縛は [StageRun](../architecture/data.md#ステージの実行-stagerun)(ステージの実行の記録)、完了記録は[完了条件](../architecture/guarantees.md#完了条件)に従う。

### ラウンドと指示

記録内容と指示 ID の導出・出し直し・本文の扱いは [Round](../architecture/data.md#ラウンド-round)(作業依頼から判断までの往復)と [Instruction](../architecture/data.md#指示-instruction)(エージェントへの指示)の定義に従う。

分割した作業では、単位ごとの作業指示・場所・終了ブロックと、合流の開始時の HEAD・入力として固定した SHA・結果を記録する。再開時は合流の記録と git のマージの状態を照合する。単位ごとの判定は記録せず、合流後のツリーと、単位の終了ブロックをまとめた作業者の終了ブロックをステージのラウンドで扱う。

### 進捗

[終了ブロック](../architecture/data.md#進捗-progress)(ラウンドの作業報告)の記録は[進捗](../architecture/data.md#進捗-progress)、入力の場所と受理条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。

### check の結果

結果の受理と記録の分担は [check](../architecture/modules.md#check)(観点ごとの判定を扱う部品)と [ledger の契約](../architecture/modules.md#ledger)に従う。判定値と項目、有効性・失効は [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、結果の受理と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

### 検出試験

合格記録の正本との分離、記録の同一性と来歴は [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)(check の判定能力の試験記録)の定義に従う。

合格の識別には、担当の定義、判定規則を含む判定依頼テンプレート、実際に基準として渡した文書の集合と内容ハッシュ、例の文書の内容ハッシュを含める。索引はソースではなく `[state] detect`(既定 `.devkit/detect.json`)で設定する。

索引の読み出し・書き込み・復元は、試験の指示に固定した同じ参照元の設定を使い、未コミットの `[state] detect` の変更には従わない。合格・不合格の両方の検出試験について、例ごとの判定・指摘・期待判定との照合結果を作業履歴に含める。

### 外部操作の記録

[外部操作](../architecture/flow.md#外部操作)(リポジトリ外への書き込み)の記録項目は [ExternalOp](../architecture/data.md#外部への書き込み-externalop)(操作の実行記録)、承認との結び付けは[外部操作の承認](../architecture/guarantees.md#外部操作の承認)、再送時の解決は[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。

### 成果物

判定・承認・反映・完了の対象は、git のツリーと外部の成果の文書のソース名・ID・内容ハッシュの集合を合わせて識別する。固定した本文と元のバージョンの識別子を来歴として記録する。

生成方法と参照項目は[成果物の記録](../architecture/data.md#成果物-artifact-record)(変更と永続化先の対応)の定義に従う。

### checkpoint

終了ブロックの[一時状態](../architecture/data.md#進捗-progress)(無効になる条件を持つ情報)を含む現在地の組み立ては [checkpoint](../architecture/data.md#checkpoint)(再開用の現在地)、注入との接続は [SessionStart](../architecture/hooks.md#sessionstart)(起動・再開などのイベント)に従う。一時状態の条件を判定する評価関数は受け取らず、条件による除外も行わない。確認方法・出典・無効になる条件をそのまま添えて渡す。

### 状態の問い合わせ

問い合わせ可能な派生値は [ledger の公開する契約](../architecture/modules.md#ledger)に従う。

## 保証

- **互換性**: 記録形式の互換性は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従う。
- **保存と書き手**: 保存・公開・排他の責務分離は [ledger](../architecture/modules.md#ledger) と [ledger/fsstore](../architecture/modules.md#ledgerfsstore)(ファイル保存を担うモジュール)の契約、書き込みの境界は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、hook の[生成物](../architecture/data.md#所有と生存期間)(削除や再生成が可能なデータ)は [hook](../architecture/hooks.md)に従う。
- **生存期間**: 完了・中止後の記録と、人の判断の追記は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

### 育てるものとの境界

要件の正本は既定 `.devkit/requirements.md`、構造の記録は `.devkit/structure.json`、規則は `.devkit/rules.md` に置き、`[sources.requirements]`・`[sources.structure]`・`[sources.rules]` で変更できる。実行の記録 `.devkit/run/` は git 管理せず、後の作業に効かせる決定は要件・構造・規則のいずれかに残す。要件は purpose で承認された完了条件と既存の要件を照らして更新し、purpose で未反映の要件は retrospective で反映する。

成果のソース・検出試験の索引・実行の記録は規範の識別に含めない。成果の更新は規範の変更による失効を起こさないが、判定対象の識別には含める。requirements と structure はループ自身の変更の確認対象に含めず、ステージの判定で確認する。規則の変更はループ自身の変更として確認する。

[構造](../architecture/data.md#構造の記録-structure)(リポジトリの構造の資産)と[規則と強制手段](../architecture/data.md#規則と強制手段-rules)(設計上の決定と守る手段)の分類は、[育てるもの](../architecture/data.md#所有と生存期間)(リポジトリが git で管理する資産)を定める「所有と生存期間」と各定義に従う。
