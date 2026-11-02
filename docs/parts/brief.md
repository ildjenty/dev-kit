# brief — 依頼の組み立て

## 責務

brief は、[作業者](../architecture/data.md#計画-plan)(作業を行うロール)への指示と、[判定者](../architecture/data.md#計画-plan)(評価を担うロール)・[計画者](../architecture/data.md#計画-plan)(計画を作るロール)への依頼文を組み立てる。公開する型と操作、バージョンの計算、他の部品との境界は [brief のモジュール契約](../architecture/modules.md#brief)に従い、計画の型と意味論は [plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)が扱う。

## 入力と出力

### 入力

作業依頼の組み立てでは、[ステージ](../architecture/data.md#ループの雛形-plantemplate)(作業の段階)の定義や前のステージの出力などに加え、[計画](../architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)の完了条件と上限を入力として受け取る。基本の入力構成と[規範](../architecture/guarantees.md#規範の固定)(評価に用いる規則)の参照先は[作業の指示](../architecture/flow.md#作業の指示)に従う。

リポジトリの事実として[配置ガイド](../architecture/layout.md#利用側のリポジトリ)(置き場所と役割の案内)と「元に戻せないもの」を受け取り、後者の取得元は[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従う。

修正[ラウンド](../architecture/data.md#ラウンド-round)(作業と判定の往復)では、指摘と検証の失敗情報を受け取る。入力の構成は[指摘の参照](../architecture/data.md#指摘-finding)、[brief のモジュール契約](../architecture/modules.md#brief)と[作業結果の判定](../architecture/flow.md#作業結果の判定)に従う。

計画依頼の組み立てでは、[作業の開始](../architecture/flow.md#作業の開始)に従って[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(ループの定義)と課題を受け取り、再提案・再計画では[計画の check と承認ゲート](../architecture/flow.md#計画の-check-と承認ゲート)と[計画のバージョン](../architecture/data.md#計画のバージョン)に従って指摘や残りの実行内容を受け取る。

判定依頼の組み立てでは、[check](check.md#入力と出力)(判定依頼の内容を決める部品)が決めた内容を入力として受け取る。

### 出力

brief の内容とバージョンを返し、計算・書き出しの責務は [brief のモジュール契約](../architecture/modules.md#brief)、指示との対応と配置は[指示](../architecture/data.md#指示-instruction)と[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。作業の brief には、誰のために何を行い、何を変更してよいか・いけないか、コマンドの実行結果や観察できる結果で完了条件をどう確認するかを既知の情報とともに示し、計画の上限と[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の返却要件を含める。

## 保証

同じ入力からは同じ内容とバージョンを組み立てる。brief はファイルの読み書きなどの外部入出力を行わず、規範の入力は [engine](../architecture/modules.md#engine)(ループの進行を担う実装)が [workspace](../architecture/modules.md#workspace)(git 操作を扱う実装)を通じて[規範の固定](../architecture/guarantees.md#規範の固定)に従って読み出した値を受け取る。

[一時状態](../architecture/data.md#進捗-progress)(再開まで引き継ぐ状況)の無効条件の照合は [ledger](../architecture/modules.md#ledger)(記録を扱う部品)が担い、brief は無効なものを除いた入力から組み立てる。

判定者・計画者の起動方法と、[指示 ID](../architecture/data.md#指示-instruction)(依頼を識別する値)から依頼本文のファイルを特定して読む手順は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

停止の判断は [engine](../architecture/modules.md#engine) と [router](../architecture/modules.md#router)(次の行動を選ぶ部品)が担う。

brief 自身は具体的な進め方や原因の仮説を付け加えず、前のステージの出力は内容を変えずに渡す。

## 試験項目

試験の範囲は正本の [brief の試験項目](../architecture/contract-changes.md#brief-1)に従う。

修正用の brief に、必須の [`command:` の check](../architecture/data.md#計画-plan)(登録した検証コマンドの実行)と [pre-commit](../architecture/guarantees.md#完了条件)(リポジトリの検証 hook)の失敗出力が含まれることを確認する。
