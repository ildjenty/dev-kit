# brief — 依頼の組み立て

## 責務

brief は、[作業者](../architecture/data.md#計画-plan)(作業を行うロール)への指示と、[判定者](../architecture/data.md#計画-plan)(評価を担うロール)・[計画者](../architecture/data.md#計画-plan)(計画を作るロール)への依頼文を組み立てる。公開する型と操作、バージョンの計算、他の部品との境界は [brief のモジュール契約](../architecture/modules.md#brief)に従い、計画の型と意味論は [plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)が扱う。

## 入力と出力

### 入力

作業依頼の組み立てでは、[ステージ](../architecture/data.md#ループの雛形-plantemplate)(作業の段階)の定義と前の出力などの入力に、[計画](../architecture/data.md#計画-plan)(課題ごとのループ)の完了条件と上限を受け取る。基本の入力構成と規範の参照先は[作業の指示](../architecture/flow.md#作業の指示)に従う。

[配置ガイド](../repository.md#リポジトリ側の構成)(置き場所と役割の案内)として変更対象のディレクトリの README を取り込み、「元に戻せないもの」は[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従って `.devkit/rules.md` と計画の両方から受け取る。

修正[ラウンド](../architecture/data.md#ラウンド-round)(作業と判定の往復)の入力は[指摘の参照](../architecture/data.md#指摘-finding)と[作業結果の判定](../architecture/flow.md#作業結果の判定)に従い、失敗した必須の `command:` の check(登録した検証コマンドの実行)と [pre-commit](../architecture/guarantees.md#完了条件)(コミット時の検証)の出力も受け取る。

判定依頼の組み立てでは、[check](check.md#入力と出力)(判定依頼の内容を決める部品)が決めた内容を入力として受け取る。

### 出力

brief の内容とバージョンを返し、計算・書き出しの責務は [brief のモジュール契約](../architecture/modules.md#brief)、指示との対応と配置は[指示](../architecture/data.md#指示-instruction)と[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。作業の brief には、誰のために何を行い、何を変更してよいか・いけないか、コマンドの実行結果や観察できる結果で完了条件をどう確認するかを既知の情報とともに示し、計画の上限と[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の返却要件を含める。

## 保証

同じ入力からは同じ内容とバージョンを組み立てる。入力の読み出しは[規範の固定](../architecture/guarantees.md#規範の固定)に従う。

[一時状態](../architecture/data.md#進捗-progress)(再開まで引き継ぐ状況)の無効条件の照合は [ledger](../architecture/modules.md#ledger)(記録を扱う部品)が担い、brief は無効なものを除いた入力から組み立てる。

判定者・計画者の起動と本文の解決は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

停止の判断は [engine](../architecture/modules.md#engine)(ループの進行を担う実装)と [router](../architecture/modules.md#router)(次の行動を選ぶ部品)が担う。

brief 自身は具体的な進め方や原因の仮説を付け加えず、前のステージの出力は内容を変えずに渡す。受け取った側の着手手順は [principles/working.md](../../principles/working.md) の原則1に従う。

途中の記録は [`devkit note`](../architecture/data.md#決定と証拠-decision--evidence)(決定と証拠を残すコマンド)、終了ブロックの提出は[作業結果の提出](../architecture/flow.md#作業結果の提出)に従う。

## 試験項目

試験の範囲は正本の [brief の試験項目](../architecture/contract-changes.md#brief-1)に従う。

修正用の brief に、必須の `command:` の check と pre-commit の失敗出力が含まれることを確認する。
