# brief — 依頼の組み立て

## 責務

brief は、[作業者](../architecture/data.md#計画-plan)(作業を行うロール)への指示と、[判定者](../architecture/data.md#計画-plan)(評価を担うロール)・[計画者](../architecture/data.md#計画-plan)(計画を作るロール)への依頼文を組み立てる。公開する型と操作、他の部品との境界は [brief のモジュール契約](../architecture/modules.md#brief)に従い、計画の型と意味論は [plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)が扱う。

## 入力と出力

### 入力

作業依頼の組み立てでは、[ステージ](../architecture/data.md#ループの雛形-plantemplate)(作業の段階)の定義や前のステージの出力などに加え、[計画](../architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)の完了条件と上限を入力として受け取る。基本の入力構成と[規範](../architecture/guarantees.md#規範の固定)(評価に用いる規則)の参照先は[作業の指示](../architecture/flow.md#作業の指示)に従う。

規範の本文には、そのステージ定義、判定の観点ファイルと参照する原則、rules、requirements、関係する構造の記録を入れる。rules・requirements・structure の既定の場所は `.devkit/rules.md`・`.devkit/requirements.md`・`.devkit/structure.json` であり、他の規範も含めて `[paths]` で変更できる。計画者・作業者・判定者の brief には requirements を必ず入れる。その他の規範は論理名と実際のパスの一覧として渡す。パスはラウンド開始時、ラウンド外の判定では指示を出す時点のスナップショットを `snap/` に展開した場所を指し、作業ツリーで編集中の規範は読ませない。`schemas/…` はバイナリが出力スキーマの本文として渡す。

リポジトリの事実として[配置ガイド](../architecture/layout.md#利用側のリポジトリ)(置き場所と役割の案内)と「元に戻せないもの」を受け取り、後者の取得元は[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従う。

修正[ラウンド](../architecture/data.md#ラウンド-round)(作業と判定の往復)では、指摘と検証の失敗情報を受け取る。入力の構成は[指摘の参照](../architecture/data.md#指摘-finding)、[brief のモジュール契約](../architecture/modules.md#brief)と[作業結果の判定](../architecture/flow.md#作業結果の判定)に従う。

計画依頼の組み立てでは、[作業の開始](../architecture/flow.md#作業の開始)に従って[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(ループの定義)、人の要求の原文、`start` の課題、requirements を受け取り、再提案・再計画では[計画の check と承認ゲート](../architecture/flow.md#計画の-check-と承認ゲート)と[計画のバージョン](../architecture/data.md#計画のバージョン)に従って人の修正指示、check の指摘や残りの実行内容を受け取る。

ステージの遷移 `done` が `"replan"` の場合も、ステージの完了後に既存の再計画の経路で残りを作り直す依頼を組み立てる。要件定義を特別扱いせず、雛形の遷移に従う。

判定依頼の組み立てでは、[check](check.md#入力と出力)(判定依頼の内容を決める部品)が決めた内容を入力として受け取る。

### 出力

brief の内容を返し、組み立て・書き出しの責務は [brief のモジュール契約](../architecture/modules.md#brief)、指示との対応と配置は[指示](../architecture/data.md#指示-instruction)と[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。作業の brief には、誰のために何を行い、scope の出力と rules に基づく作業範囲、文章で書いた完了条件と[計画](../architecture/data.md#計画-plan)に定める条件ごとの確認手段を既知の情報とともに示し、計画の上限と[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の返却要件を含める。

計画承認の問いには、要求の原文と `start` の課題、ID 付きの完了条件、採用したステージと外したステージ、逸脱の明示、各ステージの必須 check、課題固有の元に戻せないもの、宣言した外部操作、計画者の未決事項 `open_questions` を載せる。devkit が計画などの記録から組み立て、エージェントの要約には依存しない。要求の原文を観測していなければ原文なしと示す。

振り返りの brief には、その作業のすべての判定と指摘(blocking でないものを含む)、FAIL と NOT ASSESSED の経緯、ラウンド数、人の回答と作り直しの指示、note の教訓で伝えた見逃し、規範の一覧を含める。

## 保証

同じ入力からは同じ内容を組み立てる。外部入出力の扱いは[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従い、規範の入力は[規範の固定](../architecture/guarantees.md#規範の固定)に従う。

決定の有効条件・適用条件と[一時状態](../architecture/data.md#進捗-progress)(再開まで引き継ぐ状況)の条件は、出典・参照とともにそのまま項目に添えて渡す。[ledger のモジュール契約](../architecture/modules.md#ledger)に従い、条件を評価して入力を除外することはせず、有効かどうかはエージェントと人が判断する。

判定者・計画者の起動方法と、[指示 ID](../architecture/data.md#指示-instruction)(依頼を識別する値)から依頼本文のファイルを特定して読む手順は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

停止の判断は [engine](../architecture/modules.md#engine) と [router](../architecture/modules.md#router)(次の行動を選ぶ部品)が担う。

brief 自身は具体的な進め方や原因の仮説を付け加えず、前のステージの出力は内容を変えずに渡す。
