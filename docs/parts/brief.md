# brief — 依頼の組み立て

## 責務

brief は、作業者への指示と、判定者・計画者への依頼文を組み立てる。公開する型と操作、バージョンの計算、他の部品との境界は [brief のモジュール契約](../architecture/modules.md#brief)に従い、計画の型と意味論は [plan](../architecture/modules.md#plan)が扱う。

## 入力

ステージ定義、前のステージの出力、リポジトリの事実(配置ガイド、rules.md の元に戻せないもの、利用可能な check)から組み立て、修正ラウンドには前回の指摘と指摘 ID を含める。入力の構成と規範の参照先は[作業の指示](../architecture/flow.md#作業の指示)、元に戻せないものは[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従う。

判定依頼には、check が決めた規範・観点・対象・出力スキーマ・前回の指摘 ID を受け取る。内容を決める責務は [check](../architecture/modules.md#check)、依頼文に組み立てる責務は [brief](../architecture/modules.md#brief)に従う。

## 出力

brief の内容と、内容ハッシュで表すバージョンを返す。本文は生成物 `brief/<指示 ID>.md` として engine が書き出し、指示との対応は[指示](../architecture/data.md#指示-instruction)、配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

作業の brief には次の要点を示す。

- **ゴール**: 何を、誰のために行うのか。
- **完了条件**: コマンドの実行結果、または観察できる結果によって確認できる条件。計画の条件との対応は [Plan](../architecture/data.md#計画-plan)に従う。
- **提示する根拠**: 完了条件を満たしたことを、どのように示すか。
- **既知の情報**: 症状、再現手順、ログの参照先。
- **スコープ**: 変更してよいものと、変更してはいけないもの。
- **上限**: [plan](../architecture/modules.md#plan)が扱うラウンド数と進展がなかった回数の上限の表示。
- **返却内容**: [進捗](../architecture/data.md#進捗-progress)に従う終了ブロック。終了の種別、変更・確認状況、参照しなかった入力、一時状態と再開条件などを示す。

途中の決定と証拠は [note](../architecture/data.md#決定と証拠-decision--evidence)で記録し、人の判断が必要な事項は終了の種別で表す。終了ブロックの項目と提出は[進捗](../architecture/data.md#進捗-progress)と[作業結果の提出](../architecture/flow.md#作業結果の提出)に従う。

## 保証

同じ固定入力からは同じ内容とバージョンを組み立て、無効な一時状態を引き継がない。組み立ては [brief](../architecture/modules.md#brief)、一時状態の有効性は [checkpoint](../architecture/data.md#checkpoint)に従う。

判定者・計画者の起動には定型の1行を使い、依頼の本文は brief から読ませる。起動の定型と本文の解決は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

brief は上限を表示し、停止の判断は [engine](../architecture/modules.md#engine)と [router](../architecture/modules.md#router)が担う。ステージ間の引き継ぎも、前の出力を入力の一部として[組み立てる](../architecture/modules.md#brief)。

個々の作業の brief には、具体的な進め方や原因の仮説を含めない。受け取った側の着手手順は [principles/working.md](../../principles/working.md) の原則1に従う。

## 試験項目

固定入力に対する内容とバージョンの再現性、修正時の指摘と指摘 ID の取り込み、無効な一時状態の除外を確認する。起動時の定型と依頼の読み方に対応するサブエージェント定義を含む範囲は [brief の試験項目](../architecture/contract-changes.md#brief-1)に従う。
