# 契約の改訂と試験

部品の契約への改訂は、[セッション内の CLI と hook の分担](../architecture.md#全体構成)に合わせて文書へ反映済みである。kit の skill の手順は未反映であり、試験ではモジュールごとの境界と組み合わせを確認する。

## 契約の改訂

### design

- [design.md](../design.md)の部品とループに組み込む要素の区別を、[modules の提供単位](modules.md#モジュールごとの契約)に合わせて改訂した。
- [design.md](../design.md#部品)の計画による取捨と設計の正本の所在を、[利用者が組むもの](../architecture.md#利用者が組むもの)・[構成要素](../architecture.md#構成要素)に合わせて改訂した。

### runtime

- [runtime.md](../parts/runtime.md)の進行と入力の受け渡しを、[architecture.md「全体構成」](../architecture.md#全体構成)・[engine の操作](modules.md#engine)・[判定結果の受理](guarantees.md#判定結果の受理)・[回答の受理](guarantees.md#回答の受理)・[UserPromptSubmit](hooks.md#userpromptsubmit)に合わせて改訂した。
- [runtime.md](../parts/runtime.md#ファイルの分類と置き場所)のファイルの分類と配置を、[所有と生存期間](data.md#所有と生存期間)・[利用側のリポジトリ](layout.md#利用側のリポジトリ)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の hook と書き込み先を、[hook の共通処理](hooks.md#共通処理)・[記録の書き込みと排他](data.md#記録の書き込みと排他)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の hook の共通処理と責務を、[Stop の観測内の継続回数と上限](hooks.md#stop)・[cmd/devkit](modules.md#cmddevkit)・[host/event](modules.md#hostevent)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の停止・完了と skill 編集時の確認を、[停止の補助](hooks.md#stop)・[submit の終了ブロック](data.md#進捗-progress)・[完了条件](guarantees.md#完了条件)・[人への確認対象](guarantees.md#人への確認対象)に合わせて改訂した。
- [runtime.md](../parts/runtime.md#ホストの差分と共通の仕組み)のホストの確認と実行権限を、[doctor](modules.md#doctor-の確認項目)・[セッションの観測](data.md#セッションの観測-sessionobservation)・[権限の境界](guarantees.md#権限の境界)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の隔離・排他・委任の扱いを、[ホストに任せる範囲](guarantees.md#実装対象外の機能)・[作業の排他制御](data.md#作業の排他制御)・[定型の依頼](guarantees.md#判定結果の受理)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の再開と対象の照合を、[セッションとブランチ](data.md#セッションとブランチ)・[稼働確認](guarantees.md#再開と-hook-の稼働確認)・[対象の照合](guarantees.md#commit-直前の対象の照合)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の検証と commit の扱いを、[config](modules.md#config)・[engine](modules.md#engine)・[完了条件](guarantees.md#完了条件)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)のループ計画と判断の入力を、[雛形](data.md#ループの雛形-plantemplate)・[計画](data.md#計画-plan)・[router の入力と判断](modules.md#router)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の設定と配置を、[固定配置](layout.md#利用側のリポジトリ)・[config の設定項目](modules.md#config)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の導入と配布を、[install の init](modules.md#install)・[バイナリの配布方法](decisions.md#バイナリの配布方法)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の実行の記録と網羅性を、[セッションの観測](hooks.md#stop)・[CLI による書き込み](data.md#記録の書き込みと排他)・[view の成果物による網羅性](modules.md#view)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の外部操作の再送を、[外部操作の二重適用](guarantees.md#外部操作の二重適用)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の再計画を、[計画のバージョン](data.md#計画のバージョン)・[flow の再計画](flow.md#router-の判断)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)のメタループの編集と検出試験を、[人への確認対象](guarantees.md#人への確認対象)・[観点ファイルだけ](guarantees.md#検出試験)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の組み立てと再開の試験を、[固定配置](layout.md#利用側のリポジトリ)・[ledger から組み立てる checkpoint](data.md#checkpoint)・[結合試験](#結合試験)に合わせて改訂した。
- [runtime.md](../parts/runtime.md#ホストの差分と共通の仕組み)の出力と checkpoint の注入を、[性能と出力の上限](guarantees.md#性能と出力の上限)に合わせて改訂した。
- [runtime.md](../parts/runtime.md)の設計上の決定の所在を、[decisions.md の仮決定](decisions.md#仮決定)・[不採用の案](decisions.md#不採用の案)に合わせて改訂した。
- [runtime.md](../parts/runtime.md#ホストの差分と共通の仕組み)のホスト形式の変換と起動・受理を、[host/event による変換](modules.md#hostevent)・[定型の依頼と結果の受理](guarantees.md#判定結果の受理)に合わせて改訂した。

### repository

- [repository.md](../repository.md)の指示ファイルと終了手順を、[指示ファイルとホスト設定](layout.md#指示ファイルとホスト設定)・[作業結果の提出](flow.md#作業結果の提出)に合わせて改訂した。
- [repository.md](../repository.md#実行の仕組みとの対応)の skill と計画者の役割を、[kit の skill](layout.md#指示ファイルとホスト設定)・[flow の計画者](flow.md#作業の開始)に合わせて改訂した。
- [repository.md](../repository.md#実行の仕組みとの対応)の作業と判定の分担を、[全体構成](../architecture.md#全体構成)・[Plan の担当](data.md#計画-plan)に合わせて改訂した。
- [repository.md](../repository.md#リポジトリ側の構成)の元に戻せないものと外部操作を、[Rules](data.md#規則と強制手段-rules)・[config](modules.md#config)・[Plan の宣言](data.md#計画-plan)に合わせて改訂した。
- [repository.md](../repository.md#リポジトリ側の構成)のリポジトリが育てるものを、[所有と生存期間](data.md#所有と生存期間)に合わせて改訂した。
- [repository.md](../repository.md#リポジトリ側の構成)の設定と実行の記録を、[config](modules.md#config)・[利用側のリポジトリ](layout.md#利用側のリポジトリ)に合わせて改訂した。
- [repository.md](../repository.md#リポジトリ側の構成)の固定物の供給を、[kit の供給](modules.md#kit)に合わせて改訂した。
- [repository.md](../repository.md#リポジトリ側の構成)の導入と配布・実行権限を、[init の役割](modules.md#install)・[バイナリの配布方法](decisions.md#バイナリの配布方法)・[権限の境界](guarantees.md#権限の境界)に合わせて改訂した。
- [repository.md](../repository.md#実行の仕組みとの対応)の進行と再計画を、[architecture.md「全体構成」](../architecture.md#全体構成)・[計画のバージョン](data.md#計画のバージョン)に合わせて改訂した。

### ledger

- [ledger.md](../parts/ledger.md)の checkpoint と確認済みの集合を、[checkpoint](data.md#checkpoint)・[状態変更](data.md#決定と証拠-decision--evidence)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の保存方法と実行状態を、[記録の書き込みと排他](data.md#記録の書き込みと排他)・[StageRun](data.md#ステージの実行-stagerun)に合わせて改訂した。
- [ledger.md](../parts/ledger.md#決定)の途中の記録を [note](data.md#決定と証拠-decision--evidence)への参照に改め、調査の記録順を削除した。kit の skill への手順の追加は[skill の手順](#skill-の手順)に残す。
- [ledger.md](../parts/ledger.md#check-の結果)の検出試験・判定と網羅性を、[DetectionTest](data.md#検出試験の記録-detectiontest)・[Judgment](data.md#check-の結果-judgment)・[view の成果物](modules.md#view)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の指示と回答を、[Instruction](data.md#指示-instruction)・[Round](data.md#ラウンド-round)・[SubagentStop](hooks.md#subagentstop)・[Answer / Approval](data.md#回答承認-answer--approval)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の共通項目と種別ごとの参照を、[記録の範囲](data.md#記録の範囲)に合わせて改訂した。
- [ledger.md](../parts/ledger.md#ステージの実行)の記録の書き手と外部操作を、[記録の書き込みと排他](data.md#記録の書き込みと排他)・[ExternalOp](data.md#外部への書き込み-externalop)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の構造と規則の所在を、[Structure](data.md#構造の記録-structure)・[Rules](data.md#規則と強制手段-rules)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の記録種別を、[ledger のペイロード](modules.md#ledger)に合わせて改訂した。
- [ledger.md](../parts/ledger.md#成果物)の成果物の記録を、[Artifact record](data.md#成果物-artifact-record)に合わせて改訂した。
- [ledger.md](../parts/ledger.md)の計画と完了記録を、[計画のバージョン](data.md#計画のバージョン)・[StageRun](data.md#ステージの実行-stagerun)に合わせて改訂した。

### router

- [router.md](../parts/router.md#check-の結果)の判定結果と完了の判断を、[完了条件](guarantees.md#完了条件)・[Judgment](data.md#check-の結果-judgment)に合わせて改訂した。
- [router.md](../parts/router.md#人への確認)の人への確認と hook の稼働確認を、[人への確認対象](guarantees.md#人への確認対象)・[hook の稼働不足](guarantees.md#再開と-hook-の稼働確認)に合わせて改訂した。
- [router.md](../parts/router.md#ロールと実行主体)のロールと実行主体を、[router のロールと実行主体](modules.md#router)に合わせて改訂した。
- [router.md](../parts/router.md#教訓の反映先)の教訓の分類と移行の記録を、[人への確認対象で定める順序](guarantees.md#人への確認対象)・[Rules](data.md#規則と強制手段-rules)に合わせて改訂した。
- [router.md](../parts/router.md#check-の結果)の分類の入力を、[router の契約](modules.md#router)に合わせて改訂した。

### check

- [check.md](../parts/check.md)の対象と依頼の入力を、[TargetVersion](data.md#対象のバージョン-targetversion)・[判定対象の展開](guarantees.md#判定対象の展開と完了時の永続化)・[check の公開契約](modules.md#check)に合わせて改訂した。
- [check.md](../parts/check.md)の判定結果と指摘の出力を、[form の判定者の結果](modules.md#form)・[Finding](data.md#指摘-finding)に合わせて改訂した。
- [check.md](../parts/check.md)の作業と判定の分離を、[Plan の担当と runner](data.md#計画-plan)・[判定結果の受理](guarantees.md#判定結果の受理)に合わせて改訂した。
- [check.md](../parts/check.md)の起動とコマンド結果・修正ラウンドの扱いを、[起動時の上書きの制限](hooks.md#pretooluse)・[作業結果の判定](flow.md#作業結果の判定)・[受理境界](guarantees.md#判定結果の受理)・[Finding](data.md#指摘-finding)に合わせて改訂した。
- [check.md](../parts/check.md)の来歴・実効条件と重複結果の照合を、[判定結果の受理](guarantees.md#判定結果の受理)に合わせて改訂した。
- [check.md](../parts/check.md)の規範の入力を、[規範の固定](guarantees.md#規範の固定)・[check](modules.md#check)に合わせて改訂した。
- [check.md](../parts/check.md)の検出試験を、[検出試験](guarantees.md#検出試験)・[DetectionTest の同一性](data.md#検出試験の記録-detectiontest)に合わせて改訂した。
- [check.md](../parts/check.md)の集約と遷移の境界を、[判定結果の受理](guarantees.md#判定結果の受理)・[router の完了条件](guarantees.md#完了条件)・[plan の遷移表](modules.md#plan)に合わせて改訂した。
- [check.md](../parts/check.md)の起動・操作を要する観点の runner を、[操作の禁止とサブエージェントの制限](guarantees.md#操作の禁止とサブエージェントの制限)に合わせて改訂した。

### form

- [form.md](../parts/form.md)の固定フォーマットを、[submit の入力](data.md#進捗-progress)・[form](modules.md#form)に合わせて改訂した。
- [form.md](../parts/form.md)の問いと回答案内を、[Question](data.md#問い-question)・[回答の受理](guarantees.md#回答の受理)・[状態遷移と受理条件](data.md#状態遷移と受理条件)に合わせて改訂した。
- [form.md](../parts/form.md)のコマンドの使い方を指示ファイルに置く記述を削り、[指示ファイルとホスト設定](layout.md#指示ファイルとホスト設定)に合わせて改訂した。
- [form.md](../parts/form.md)の PR 本文を、[form のテンプレート](modules.md#form)に合わせて改訂した。
- [form.md](../parts/form.md)のスキーマと計画の検証を、[form の契約](modules.md#form)・[plan](modules.md#plan)に合わせて改訂した。

### view

- [view.md](../parts/view.md)の表示の入力を、[view](modules.md#view)に合わせて改訂した。
- [view.md](../parts/view.md)の状態の変更と問いを、[状態変更](data.md#決定と証拠-decision--evidence)・[人の回答と再開](flow.md#人の回答と再開)・[状態遷移と受理条件](data.md#状態遷移と受理条件)に合わせて改訂した。
- [view.md](../parts/view.md)の確認状況の表示を、[Judgment の有効性](data.md#check-の結果-judgment)に合わせて改訂した。
- [view.md](../parts/view.md)のカーソルと確認済みの集合を、[状態変更](data.md#決定と証拠-decision--evidence)に合わせて改訂した。
- [view.md](../parts/view.md)の構造図の生成を、[view の構造図の生成](modules.md#view)に合わせて改訂した。

### loop-plan

- [loop-plan.md](../loop-plan.md)の雛形・計画・依頼の区別を、[雛形](data.md#ループの雛形-plantemplate)・[計画](data.md#計画-plan)・[brief](modules.md#brief)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md)の固定手順と利用者が組む範囲を、[作業の開始](flow.md#作業の開始)・[作業の指示から router の判断まで](flow.md#作業の指示)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md)の判断の入力と検証を、[router の入力](modules.md#router)・[完了条件](guarantees.md#完了条件)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md)の雛形・計画の管理と決定の共有を、[所有と生存期間](data.md#所有と生存期間)・[固定配置](layout.md#利用側のリポジトリ)・[資産の更新と共有](data.md#資産の更新と共有)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md#作成ルール)の雛形と計画の作成規則を、[PlanTemplate](data.md#ループの雛形-plantemplate)・[作業の開始](flow.md#作業の開始)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md#計画の-check-と人によるレビュー)の逸脱の確認と承認条件を、[Plan の雛形からの逸脱](data.md#計画-plan)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md)の担当と実行主体・例を、[Plan](data.md#計画-plan)・[examples.md の雛形と計画の JSON](examples.md#雛形と計画)に合わせて改訂した。
- [loop-plan.md](../loop-plan.md)の外部操作の宣言を、[外部操作の承認](guarantees.md#外部操作の承認)に合わせて改訂した。

### stages

- [stages.md](../stages.md)の雛形と計画による取捨を、[PlanTemplate](data.md#ループの雛形-plantemplate)・[計画者の手順](flow.md#作業の開始)に合わせて改訂した。
- [stages.md](../stages.md#初期化)の導入と確認を、[install の init・doctor](modules.md#install)に合わせて改訂した。
- [stages.md](../stages.md#2-構造)の構造の成果物を、[Structure](data.md#構造の記録-structure)・[view](modules.md#view)に合わせて改訂した。

### checks

- [checks.md](../checks.md)の各観点のステージ欄を削除し、使用ステージの指定を [PlanTemplate](data.md#ループの雛形-plantemplate)への参照に改めた。
- [checks.md](../checks.md#終了時の状態)の終了時の状態を、[submit の終了ブロック](data.md#進捗-progress)・[完了条件](guarantees.md#完了条件)に合わせて改訂した。
- [checks.md](../checks.md#harness)の skill と接続・再開の確認を、[skill の配置](layout.md#指示ファイルとホスト設定)・[試験項目](#試験項目)に合わせて改訂した。
- [checks.md](../checks.md#計画)の計画の逸脱の確認を、[Plan の雛形からの逸脱](data.md#計画-plan)に合わせて改訂した。
- [checks.md](../checks.md#差分)の承認・委任の照合を、[判定結果の受理](guarantees.md#判定結果の受理)に合わせて改訂した。

### rules

- [rules.md](../rules.md#作業の進め方)の並行作業の配置を、[layout.md「利用側のリポジトリ」](layout.md#利用側のリポジトリ)に合わせて改訂した。
- [rules.md](../rules.md#文章と記録)から番号の回答を言い直して実行する規則を削除済みであり、選択肢と文章の回答の受理条件は[回答の受理](guarantees.md#回答の受理)に従う。

### meta-loop

- [meta-loop.md](../meta-loop.md)の教訓の反映先を、[人への確認対象の分類順](guarantees.md#人への確認対象)に合わせて改訂した。
- [meta-loop.md](../meta-loop.md#メタループ)の編集対象と人への確認を、[人への確認対象](guarantees.md#人への確認対象)に合わせて改訂した。
- [meta-loop.md](../meta-loop.md#生成した設計の典型的な問題)の却下した設計案の記録を、[決定と証拠](data.md#決定と証拠-decision--evidence)に合わせて改訂した。

### brief

- [brief.md](../parts/brief.md)の入力と引き継ぎを、[brief の責務](modules.md#brief)・[作業の指示](flow.md#作業の指示)に合わせて改訂した。
- [brief.md](../parts/brief.md)の用途・バージョン・起動を、[brief](modules.md#brief)・[定型の依頼](guarantees.md#判定結果の受理)に合わせて改訂した。
- [brief.md](../parts/brief.md)の停止の判断を、[plan の上限](modules.md#plan)・[engine](modules.md#engine)・[router](modules.md#router)に合わせて改訂した。
- [brief.md](../parts/brief.md)の返却内容を、[進捗](data.md#進捗-progress)・[note](data.md#決定と証拠-decision--evidence)に合わせて改訂した。
- [brief.md](../parts/brief.md)の計画との区別と依頼の組み立てを、[plan の型](modules.md#plan)・[brief の組み立て](modules.md#brief)に合わせて改訂した。

### skill の手順

kit の skill の実装と調査の記録順の追加は未反映である。

- kit の skill の作業の進め方は[処理の流れ](flow.md)、人への確認は[人の回答と再開](flow.md#人の回答と再開)、判定者・計画者の起動は[定型の依頼](guarantees.md#判定結果の受理)に合わせる。
- kit の skill のコマンドの呼び方は[権限の境界](guarantees.md#権限の境界)、コミットの扱いは[完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)への参照にする。
- kit の skill に、再現手順の保存先とバージョンは修正開始前、仮説の結果は次の仮説を始める前に [devkit note](data.md#決定と証拠-decision--evidence)で記録する手順を追加する。

## 試験項目

### plan

- 遷移表、上限、作り直しの差分、スキーマ、完了条件 ID と観点の対応。
- [提案と実行可能性](data.md#計画のバージョン)の状態の組み合わせ、撤回と再計画待ち。
- [外部操作の宣言](guarantees.md#外部操作の承認)の照合境界。

### brief

- 固定入力に対する内容とバージョンの再現性、修正時の指摘の取り込み。
- 無効な一時状態を除いた入力からの組み立て。
- [起動時の定型と依頼の読み方](guarantees.md#判定結果の受理)に対応するサブエージェント定義。

### check

- 観点・依頼・結果の解析、スキーマ、適用除外、集約、必須 check の網羅状況と検出試験。
- [受理境界](guarantees.md#判定結果の受理)の各不一致、重複結果と判定の食い違い、コマンド結果の変換。
- [規範の識別と有効性](data.md#check-の結果-judgment)に対するコード変更・規範変更の違い。

### router

- [router の入力](modules.md#router)の組み合わせ。
- [人への確認対象](guarantees.md#人への確認対象)の既定と追記。
- [hook の稼働不足](guarantees.md#再開と-hook-の稼働確認)を含む次の指示との接続。

### form

- [スキーマとテンプレート](modules.md#form)の適用、Markdown の描画。
- 各固定フォーマットと [data.md の定義](data.md#エンティティ)の整合。
- [問いの回答案内](data.md#問い-question)と[停止する範囲・進められる範囲](data.md#状態遷移と受理条件)の表示。

### ledger

- 参照の整合、決定の有効性、計画と承認の状態、checkpoint・確認済みの集合・未受理の指示の派生。
- [承認](data.md#回答承認-answer--approval)の再送・訂正・競合、古い問い、完了後の更新、観測削除後の根拠。
- [指示 ID](data.md#指示-instruction)の作業・観点・計画・提案・試験ごとの差、出し直し、ラウンド前の識別と本文の扱い。
- [成果物による網羅性](modules.md#view)と[観測時刻による実行状態](data.md#ステージの実行-stagerun)。

### ledger/fsstore

- 同時追記と[原子的な公開](data.md#記録の書き込みと排他)、一時ファイルの扱い、壊れた記録の隔離。
- [作業ロック](data.md#作業の排他制御)と生成物の公開。
- [読めないバージョン](guarantees.md#ロールアウト)の失敗報告。

### engine

- 関数値を差し替えた Workspace・External と偽の結果・観測を使い、各操作列の遷移、進展、再計画、[状態遷移と受理条件](data.md#状態遷移と受理条件)の拒否境界。
- [指示](data.md#指示-instruction)の再現性・重複・古い結果、[束縛とブランチ](data.md#セッションとブランチ)、環境変数のない場合、[hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)。
- [`command:` の check と pre-commit の実行順](flow.md#作業結果の判定)、必須コマンド失敗時の起動抑止、pre-commit 失敗を router に渡さず「修正する」として記録すること。
- [commit 直前の対象の照合](guarantees.md#commit-直前の対象の照合)、対象の取り直しと再判定、再変化による問い、commit の成否と次の brief への接続。
- [回答](guarantees.md#回答の受理)の入力境界・訂正期限・文章、[未回答時と inbox](data.md#状態遷移と受理条件)の制約。
- [外部操作](guarantees.md#外部操作の承認)の受理から中断後の再送、[検出試験](guarantees.md#検出試験)の不足と不合格。

### external

- [操作 ID と要求 ID](guarantees.md#外部操作の二重適用)、成功・実行中・結果不明に対する再送と発生単位。
- [承認と実行順序](guarantees.md#外部操作の承認)、計画の宣言・登録定義・実行ファイルの照合。
- [許可値と入力](modules.md#external)、期限超過から照会または人への確認への分岐。

### hook

- 両ホストの固定イベント JSON による[束縛の有無とサブエージェント内の処理](hooks.md#共通処理)、[ブランチの不一致](data.md#セッションとブランチ)。
- [SessionStart](hooks.md#sessionstart)の環境変数・注入・警告と、[Stop](hooks.md#stop)の指示ごとの判断・継続回数。
- [PreToolUse](hooks.md#pretooluse)の登録・構文・読み出し元・起動制限と、[通常の結果ファイルへの書き込み](guarantees.md#誤った場所への書き込みと危険な操作)の deny。
- [SubagentStop](hooks.md#subagentstop)の担当・transcript・実効条件・定義の来歴と、[UserPromptSubmit](hooks.md#userpromptsubmit)の回答後を含む観測範囲。

### workspace

- 一時リポジトリで[公開する git 操作](modules.md#workspace)、refs の保持、ブランチ名・user.name・ルートの取得。
- 属性の影響、確定前後の HEAD・index・作業ツリーを含む[展開と永続化](guarantees.md#判定対象の展開と完了時の永続化)。
- [完了条件](guarantees.md#完了条件)に従う、一時 index を `GIT_INDEX_FILE` で与えた `git hook run pre-commit`、`write-tree` による作業ツリーの一致確認、hook を有効にした commit と失敗時の出力の取得、環境変数の引き渡し、フォーマッタによる変更時の差分の保持。
- sandbox の中で起動した場合、[起動時の書き込み試験](guarantees.md#ホスト設定と実行権限)が記録を残さず失敗し、理由を返すこと。

### install

- [導入](modules.md#install)の冪等性、ignore、コピー、skill 単位のリンク、既存設定と inbox の保持。
- [設定の追記と差分](guarantees.md#権限の境界)、deny の範囲と既定、allow の提案、hook の matcher と失敗時の設定。
- git のバージョン確認、`git hook run` が使えない場合に判定前の pre-commit を実行せず完了時だけにすること。
- [doctor の照合項目](modules.md#doctor-の確認項目)。

### kit

- [供給するコピー](modules.md#kit)と埋め込みの一致。
- Work とコピーのバージョンの境界。
- [ルートの埋め込み](modules.md#ルートパッケージ)から供給までの接続。

### 検出試験

- 違反例・適合例・許容例と期待判定の対応。
- [合格記録の識別](data.md#検出試験の記録-detectiontest)、定義の変更、実効条件の来歴と同一性の区別。
- [ラウンド開始前の実行](guarantees.md#検出試験)への接続。

### view

- 固定の ledger と構造による[表示](modules.md#view)の冒頭・3群・確認範囲の地図・確認済みの集合・待ち・失効・観測時刻。
- 網羅性の入力と、[問いの表示と停止](data.md#状態遷移と受理条件)の関係。
- 描画結果を [principles/ui.md](../../principles/ui.md) の原則9に従って確認する。

### 結合試験

- skill の手順と偽の hook 入力で [start から完了](flow.md)までを通し、Stop の途中と待ち状態、別セッション・compaction 後の SessionStart を確認する。
- 他の skill や plugin と共存する、[束縛のないセッション](hooks.md#共通処理)を確認する。
- 両ホストでの[実行権限とコマンドの呼び方](guarantees.md#ホスト設定と実行権限)と、sandbox の中での記録前の失敗を確認する。
