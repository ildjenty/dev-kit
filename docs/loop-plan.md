# ループ計画

ループは、利用者が組む[ループの雛形](architecture/data.md#ループの雛形-plantemplate)(再利用するループの定義)と、[計画者](architecture/flow.md#作業の開始)(雛形から計画を作るサブエージェント)が課題ごとに作る[計画](architecture/data.md#計画-plan)(課題に合わせたループのデータ)の2層で表す。計画のデータ型は [plan](architecture/modules.md#plan) モジュールが定め、計画者への依頼文は [brief](architecture/modules.md#brief)(入力から組み立てる依頼文)として渡す。

kit は、[雛形から計画を作る手順](architecture/flow.md#作業の開始)と、1[ラウンド](architecture/data.md#ラウンド-round)(作業の指示から結果に対する判断までの往復)の進め方を固定し、利用者は雛形の構成を定める。

雛形はリポジトリが所有し、既定の `.devkit/plans/<name>.json` を `devkit.toml` の `[paths] plans` で変更できる。計画は作業ごとの実行の記録であり、固定の `.devkit/run/` に保存して git 管理しない。後の作業にも効かせる決定は、要件・構造・規則に残す。

## フォーマット

雛形と計画の形式、検証の方式と責務は [plan](architecture/modules.md#plan)、項目は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)と[計画](architecture/data.md#計画-plan)に従う。

ステージ(作業を区切る単位)の担当と [check](architecture/modules.md#check)(観点やコマンドによる確認)の実行主体(runner)は[計画](architecture/data.md#計画-plan)で定める。

個別の承認なしに計画の承認だけで実行してよい[外部操作](architecture/flow.md#外部操作)(リポジトリ外への書き込み行為)は、[計画の宣言](architecture/data.md#計画-plan)と[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従って扱う。

計画には、要求のうち計画者が決められなかった点を任意の `open_questions`(文字列の配列)として記入する。曖昧な要求を推測で埋めない。完了条件の書き方と条件ごとの確認手段は[計画](architecture/data.md#計画-plan)に従う。

ステージの遷移 `done` には、次のステージ ID、作業の終了を表す `null`、残りの計画の作り直しを表す `"replan"` を書ける。`"replan"` では、そのステージの完了後に既存の再計画の経路で計画者が残りを作り直す。作り直した計画にも計画の check と承認の条件を適用する。要件定義を先に行って再計画するかは雛形の組み方で決まり、kit は要件定義を特別扱いしない。

計画の検証では、`done` をたどって同じステージを再び通る循環を拒否する。`"replan"` または `null` で遷移の連鎖を終える。

## 既定の雛形

- **`default`**: `purpose → scope → structure → contract → steps → implement → retrospective`。候補は `research` と `prototype` である。`scope` に人の承認ゲートを置く。`implement` に必須の `command:test` と、観点 `test`・`diff` の check を置く。
- **`fix`**: `implement → retrospective`。候補は `research` と `prototype` である。`implement` に必須の `command:test` と、観点 `test`・`diff` の check を置く。
- **`docs`**: `implement → retrospective`。`implement` に観点 `wording` の check を置く。

`default` は `purpose` の遷移 `done` を `"replan"` にする。最初の計画には、要件と完了条件の合意に関する完了条件を持たせる。`purpose` で人に確かめながら要件を詰め、既定の `.devkit/requirements.md` に書き、完了条件の案を作る。置き場所は `[paths] requirements` で変更できる。`purpose` の完了後、計画者が合意した完了条件で残りの計画を作り直し、人が承認する。`fix` と `docs` はこの形を使わない。

すべての既定の雛形の `retrospective` は作業者が担当する。`default` と `fix` は必須の `command:test`・`test`・`harness`、`docs` は必須の `harness` だけを置く。観点の判定者(runner)は、`init` が配る `devkit-judge` である。`--plan` の省略時は `default_plan` の値を使い、既定は `"default"` である。計画者は不要なステージを外してよく、外したステージは逸脱として承認の問いに載せる。

## 例

JSON の例は[雛形と計画](architecture/examples.md#雛形と計画)を参照する。check の実行主体として登録する[判定コマンド(`[judges]`)](architecture/modules.md#config)(判定結果を返すコマンド)と[検証コマンド(`[commands]`)](architecture/modules.md#config)(終了コードで検証結果を返すコマンド)の例は[設定](architecture/examples.md#設定)を参照する。

## 作成ルール

### 雛形を組む規則

利用者は変更の種類(機能追加・修正・文書など)ごとに雛形を組み、担当を含む構成項目は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。

各雛形の開始ステージは、修正方法を知っているかではなく、何を変えるかで決める。元に戻せないもの(変更困難な決定)の判断は、公開契約か内部契約かにかかわらず、[`assets/principles/designing.md` の原則1](../assets/principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。

- 目的やスコープの根拠を具体化するなら、[ステージ0(目的)](stages.md#0-目的)。
- 元に戻せないものに触れるなら、[ステージ1(スコープ)](stages.md#1-スコープ)。
- 責務・配置・依存を変えるなら、[ステージ2(構造)](stages.md#2-構造)。
- 元に戻せないものに触れない内部の契約だけなら、[ステージ3(契約)](stages.md#3-契約)。
- コードだけなら、[ステージ4(手順)](stages.md#4-手順)または[ステージ5(実装)](stages.md#5-実装)。

check とゲート(人の承認を待つ関門)は、次の規則で組む。

- どのステージにも、そのステージで変えうるものを確かめる check を置く。
- 画面・データ・外部との通信を変えるステージには、観点[「動作検証」](checks.md#動作検証)の check を置く。
- 人によるレビューゲートの理由の書き方は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。人に確認する条件は[人への確認対象](architecture/guarantees.md#人への確認対象)で定める。
- 画面のデザインを始める前のレビューゲートの承認対象は、[問いの定義](architecture/data.md#問い-question)に従う。
- check の追加・必須化の判断は [`assets/principles/harness.md` の原則5](../assets/principles/harness.md#5-防ぐ失敗と運用負担から仕組みを選ぶ)に従う。[判定者](architecture/data.md#計画-plan)(check を担当するロール)が担当する観点を必須にする条件は、[検出試験](architecture/guarantees.md#検出試験)(観点ファイルが期待する判定を出せるかの試験)に従う。

ステージ間の前提と戻り先の設計は [`assets/principles/harness.md` の原則3](../assets/principles/harness.md#3-前提が崩れたときに戻るステージを決める)に従う。戻り先の表現は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)、入力の組み立ては[作業の指示](architecture/flow.md#作業の指示)、再計画は[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

### 計画を作る規則

計画への入力は、雛形、人の要求の原文、`start` に渡した課題、要件の正本、リポジトリの事実である。要件の正本は `.devkit/requirements.md` であり、`[paths] requirements` で変更できる。原文を観測できていなければ原文なしと記録する。brief の「元に戻せないもの」に渡す範囲は[規則と強制手段](architecture/data.md#規則と強制手段-rules)に従う。

課題ごとの雛形の選択と計画者による取捨の範囲は[作業の開始](architecture/flow.md#作業の開始)に従い、開始ステージの変更を含む逸脱の判定は[計画](architecture/data.md#計画-plan)で定める。

変えるものだけを扱う最小の計画に保ち、戻り先を定めるためだけに不要なステージを足さない。

## 計画の check と人によるレビュー

計画自体に観点[「計画」](checks.md#計画)の check を適用し、雛形からの逸脱と承認条件は[計画](architecture/data.md#計画-plan)に従って確認する。

計画の判定依頼には、作業に記録した要求の原文と `start` の課題を固定入力として渡す。完了条件と `open_questions` が原文の要求を漏らさず扱い、要求を追加していないかを確かめる。

観点と実行主体は `[plan_check] aspect = "plan"`・`runner = "devkit-judge"` が既定である。登録済みの判定コマンドは devkit が実行し、それ以外はサブエージェントへ指示する。

要件の合意として、既定では最初の計画、直前に人が承認した完了条件の ID または文面を変える計画、雛形から逸脱する計画、外部操作を宣言する計画を人が承認する。完了条件が同じで逸脱も外部操作も未決事項もない作り直しは再確認しない。`[approval] plan` は `"requirements"`(既定)、`"always"`(すべての計画を確認)、`"deviation"`(逸脱か外部操作の宣言があるときに確認)から選ぶ。

`open_questions` が空でない計画は、承認方針によらず人の承認を必須とする。

承認の問いは、エージェントの要約に頼らず、`devkit` が原文と計画から組み立てる。

- 人の要求の原文と `start` の課題。
- ID 付きの完了条件。
- 採用したステージと外したステージ、雛形からの逸脱の明示。
- 各ステージの必須 check。
- 課題固有の元に戻せないものと、宣言した外部操作。
- `open_questions` に記入した未決事項。

未決事項への人の回答は、計画の却下と作り直しの指示として計画者に渡す。この再提案に回数の上限はない。計画の check の判定のやり直しには、雛形の上限(既定3)を適用する。人が承認した完了条件を作業の完了の基準とする。

check と必要なゲートの進行は[計画の check と承認ゲート](architecture/flow.md#計画の-check-と承認ゲート)、作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

## ステージ実行時の扱い

次の行動を選ぶ入力は [router](architecture/modules.md#router)(check の結果から次の行動を分類する部品)、必須 check とコミットの関係は[完了条件](architecture/guarantees.md#完了条件)に従う。起動・操作を要する check の実行主体は[操作の禁止とサブエージェントの制限](architecture/guarantees.md#操作の禁止とサブエージェントの制限)で定める。

計画で使う `command:<name>` がラウンド開始時の `devkit.toml` に登録されていなければ、その check を `FAIL` とする。指摘は `auto-fixable` とし、「検証コマンド `<name>` が登録されていない。検証の手段を作り `devkit.toml` の `[commands.<name>]` に登録する」と伝える。作業者が用意したコマンドは、次のラウンド開始時のスナップショットから使う。登録はループ自身の変更として人が確認する。

ループ自身の変更はステージの宣言によらず扱う。既定の `[approval] meta_loop_edit = "confirm"` では、未確認の変更が効くラウンドを開く前に止めて人が確認し、最後のラウンドでの変更は完了時のコミット前に確認する。`"allow"` も設定できる。要件と構造の記録は作業の成果であり、この分類に含めない。
