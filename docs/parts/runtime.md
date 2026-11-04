# runtime — 実行

runtime は単一バイナリの `devkit` として、セッション内でループを進める。作業者サブエージェント、セッションのまとめ役と CLI・hook の分担は[全体構成](../architecture.md#全体構成)に従う。

## 責務

- **進行**: runtime は[計画](../architecture/data.md#計画-plan)(課題ごとの実行内容)と [ledger](../architecture/modules.md#ledger)(記録を扱う部品)を使ってループを進める。実装モジュールへの責務配分は [engine](../architecture/modules.md#engine)(進行を担うモジュール)を含む[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。
- **作業者・判定者・計画者との接続**: 役割の agent であるサブエージェントの起動主体は[全体構成](../architecture.md#全体構成)、最終メッセージと来歴の採取・受け渡しは [SubagentStop](../architecture/hooks.md#subagentstop)(サブエージェント終了時のイベント)、起動の定型と結果の受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **git と外部操作**: git 操作と[外部操作](../architecture/flow.md#外部操作)(リポジトリ外への書き込み)の責務配分は [workspace](../architecture/modules.md#workspace) と [external](../architecture/modules.md#external) の契約に従う。
- **組み立て**: 設定、リポジトリの規範、バイナリが供給するスキーマと既定値の受け渡しを含む組み立ては [cmd/devkit](../architecture/modules.md#cmddevkit) の契約に従う。
- **導入**: バイナリの配布、利用側の配置と設定の準備は [install](../architecture/modules.md#install) の契約に従う。
- **指示ファイルと skill**: kit の操作手順と skill の配置、リポジトリの AGENTS.md・CLAUDE.md との境界は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。

<a id="ファイルの分類と既定の置き場所"></a>

### ファイルの分類と置き場所

ファイルの分類と所有者、更新と生存期間は[所有と生存期間](../architecture/data.md#所有と生存期間)、設定で変更できる配置と git 管理の範囲は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

知識は `[sources.<名前>]` のアダプタで取得し、規範はソース全体、成果は選んだ範囲を固定する。requirements は既定 `.devkit/requirements.md`、structure は `.devkit/structure.json` であり、成果として規範の識別に含めない。合格の索引はソースではなく `[state] detect`(既定 `.devkit/detect.json`)に置く。実行の記録と生成物 `.devkit/run/` は固定であり、git 管理しない。

## 入力と出力

- **設定と規範**: `devkit.toml` の設定とリポジトリの規範、バイナリが供給するスキーマと既定値を受け取る。判定コマンド(`[judges]`)・検証コマンド(`[commands]`)(いずれも設定項目)の解析・検証は [config](../architecture/modules.md#config)、スキーマと既定値の供給、記録形式とバイナリの互換性の識別は [kit](../architecture/modules.md#kit)に従う。
- **ループのパターン / 計画**: [ループのパターン](../architecture/data.md#ループの雛形-plantemplate)(ループの参考)と計画を受け取り、進行に用いる。[router](../architecture/modules.md#router)(次の行動を選ぶ部品)が判断に使う入力は同節に従う。
- **終了ブロックと成果物**: [ラウンド](../architecture/data.md#ラウンド-round)(作業依頼から判断までの往復)の[終了ブロック](../architecture/data.md#進捗-progress)(作業報告)を受け取る。返却後の[対象のバージョン](../architecture/data.md#対象のバージョン-targetversion)(判定対象の識別値)と成果物の記録は[作業結果の提出](../architecture/flow.md#作業結果の提出)、入力 JSON の場所と拒否条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。
- **判定結果**: 結果ファイルとコマンド実行結果を受け取る。受理から記録までの分担は [check](../architecture/modules.md#check)(観点ごとの判定を扱う部品)と [ledger の契約](../architecture/modules.md#ledger)、結果の種類と照合・集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)、`command:` の check(検証コマンドを実行する check)の定義は[計画](../architecture/data.md#計画-plan)、実行順は[作業結果の判定](../architecture/flow.md#作業結果の判定)に従う。
- **人の入力と訂正**: 観測された人の回答と訂正を受け取る。回答の解釈と記録・訂正は[回答の受理](../architecture/guarantees.md#回答の受理)、入力の観測は [UserPromptSubmit](../architecture/hooks.md#userpromptsubmit)(人の入力時のイベント)、問いの提示と再開は[人の回答と再開](../architecture/flow.md#人の回答と再開)に従う。
- **中断**: 人の依頼でエージェントが呼ぶ `devkit pause`・`devkit resume`、または人による直接の実行を受け付ける。中断中の `next` の応答を含む条件は[中断](../architecture/guarantees.md#中断)、CLI の呼び分けは [cmd/devkit](../architecture/modules.md#cmddevkit)、中断・再開の直接の記録は [engine](../architecture/modules.md#engine)に従う。
- **指示と表示**: CLI は次の指示と [brief](../architecture/modules.md#brief)(作業や判定の依頼文)、問い、状態、操作の結果を返す。出力の長さと返し方は[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。

`init` は導入内容を人が確認したあとコミットし、`--no-commit` で省略できる。HEAD がない、または作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合は、`start` を拒否して理由と直し方を返す。優先するパターンの省略時は `default_plan`、ブランチの運用は計画のステージの入力で選ぶ git の規則、完了時のコミットは `[git]`、Stop の継続上限は `[limits] stop_continuation` に従う。

作業中のチェックアウトでの2つ目の `start` は拒否する。別の worktree で新しいセッションを開けば、チェックアウトごとの `.devkit/run/` で複数のループを並行して回せる。

計画のステージの `split` と `units` に従う単位の作業は、単位の役割(既定 `worker`、agent `devkit-worker`)が `.devkit/worktrees/<単位の指示 ID>/` で進める。agent のある `work` の役割がステージの担当になったラウンドも、単位が1つの同じ手順で進める。devkit はラウンド開始時のスナップショットのコミットから detached の worktree を作り、全単位の結果受理後にツリーを SHA で固定して作業ブランチへコミットしないマージで合流する。単位 ID を付けて各単位の終了ブロックを欠けなく保持し、合流後のツリーを通常のステージのラウンドで判定する。単位ごとの判定はしない。未受理のサブエージェント指示の数は、全体の `[parallel] max_concurrent`(既定3、1以上)と役割ごとの上限の両方を満たす。空いた分だけ次の `next` で発行する。1でも作業場所と合流の手順は変えない。

合流がぶつかった場合の修正ラウンド、合流の記録と再開時の照合、結果を指示に結び付けられない場合の問い、中断・中止・作業完了時の worktree の扱いは[分割した作業と合流](../architecture/flow.md#分割した作業と合流)に従う。`work` の役割の agent は判定者用の `[deny] judge` の書き込み禁止の対象外である。担当・runner・単位の担当は `devkit.toml` の[役割](../architecture/data.md#役割-role)の ID で決まり、devkit は種類ごとの規則と職務の分離を守らせる。修正ラウンドの担当は[修正の担当の規則](../architecture/data.md#修正の担当の規則)で変えられ、ラウンドの上限は規則より先に効く。

ラウンド開始時に未登録の検証コマンドは、`auto-fixable` の指摘を伴う check の `FAIL` とする。作業者が検証の手段を作って登録し、既定では人の確認を経た次のラウンドから使う。

既定のパターンは `default`・`fix`・`docs` であり、最後に作業者の retrospective を置く。検出試験を除く作業の brief と判定依頼は `history` に従う経緯の固定した索引を参照し、振り返りはそこからすべての判定と指摘、合格・不合格の検出試験の例ごとの判定・指摘・期待判定との照合結果、人の回答や note の見逃しを確認する。振り返りの必須 check は `default` と `fix` では `command:test`・`test`・`harness`、`docs` では `harness` だけである。規範を変えた場合はループ自身の変更として確認する。

ソースへの書き込みはリポジトリ内の `writable = true` のソースに限り、直接編集して完了時のコミットで反映する。リポジトリ外のソースは読み取り専用であり、devkit は書き込まず、`writable = true` を書くと設定エラーになる。判定・承認・完了は git のツリーとリポジトリ外の成果の文書を合わせた同じ識別を指す。

`start --plan` は優先するパターン、`--task-file` は課題の本文を読む指定である。メインのエージェントが合意を課題にまとめ、途中の指示を `instruct` で記録する。次の計画者・作業者だけに渡し、判定者へは渡さない。計画や完了条件の変更は再計画と承認を通す。会話の写しは取らない。

計画の受理時とステージ開始時には、採用済みの経路と計画の到達可能な並びに `[[loop.stage]]`・`[[loop.path]]` を適用する。実体 ID と定義 ID、check の一覧、参照の解決、上限の天井は[計画](../architecture/data.md#計画-plan)に従う。

### ホストの差分と共通の仕組み

対象ホストと共通の処理は[hook の共通処理](../architecture/hooks.md#共通処理)、ホストごとの形式の変換は [host/event](../architecture/modules.md#hostevent)(ホスト形式の変換モジュール)に従う。

利用するイベントと各イベントの入出力は [hook](../architecture/hooks.md)、[checkpoint](../architecture/data.md#checkpoint)(再開用の現在地)の注入のタイミングは [SessionStart](../architecture/hooks.md#sessionstart)(起動・再開などのイベント)に従う。

## 保証

- **完了条件**: 完了の主張の受理と確定は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)・[完了条件](../architecture/guarantees.md#完了条件)に従う。[検出試験](../architecture/guarantees.md#検出試験)(check の判定能力の試験)の対象は同節、判定コマンドと `command:` の check の実行は [engine](../architecture/modules.md#engine)、判定者による check の起動と結果の受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **対象と規範**: 判定の対象への結び付けと有効性は [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、規範の固定は[規範の固定](../architecture/guarantees.md#規範の固定)に従う。判定対象の展開と commit 直前の照合は、[展開と永続化](../architecture/guarantees.md#判定対象の展開と完了時の永続化)・[対象の照合](../architecture/guarantees.md#commit-直前の対象の照合)に従う。
- **実行の記録と観測**: [実行の記録](../architecture/data.md#所有と生存期間)(作業が所有する履歴)の書き手の境界と公開方法は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、排他は[作業の排他制御](../architecture/data.md#作業の排他制御)に従う。網羅性(成果物を記録した範囲)の表示は [view](../architecture/modules.md#view)に従う。
- **再開**: 作業の特定は[セッションとブランチ](../architecture/data.md#セッションとブランチ)、受理時に必要な観測の照合は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)、注入する現在地の組み立ては [checkpoint](../architecture/data.md#checkpoint)に従う。
- **外部操作**: 承認済みの操作の実行と再送時の回復は、[外部操作の承認](../architecture/guarantees.md#外部操作の承認)・[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。
- **再計画**: 前のステージへの戻りと、ステージの遷移 `done: "replan"` による完了後の残りの作り直しに、既存の再計画の経路を使う。作り直した計画にも計画の check と承認の条件を適用し、要件定義は特別扱いしない。再計画に伴う [StageRun](../architecture/data.md#ステージの実行-stagerun)(ステージの実行の記録)と計画の切り替えは[計画のバージョン](../architecture/data.md#計画のバージョン)、手順は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **ホストと git に任せる範囲**: サブエージェントの起動と隔離、git の仕組みとの責務分担は[ホストと git に任せる範囲](../architecture/guarantees.md#実装対象外の機能)に従う。単位の worktree と合流は devkit が git 操作で管理し、ループ自体を並行して回す配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。
- **メタループの編集**: 規範・ホスト設定・skill・リポジトリ内で git が追跡する有効な pre-commit の設定の変更は、どのステージでも `[approval] meta_loop_edit` に従う。既定では変更が効く最初のラウンドの前に人が確認し、最後のラウンドの変更は判定後、完了時のコミット前に確認する。requirements と structure は成果としてステージの判定で確認する。
- **互換性**: `kit_version` とバイナリの非互換と、読めない記録は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従って報告する。
- **停止の補助**: 停止の判断は [Stop](../architecture/hooks.md#stop)、中断と継続上限は[中断](../architecture/guarantees.md#中断)、完了の確定は[完了条件](../architecture/guarantees.md#完了条件)に従う。
- **checkpoint の注入**: 出力上限を超えた場合の注入内容は[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。
- **登録と実行権限**: hook の登録と `devkit` の実行権限は[権限の境界](../architecture/guarantees.md#権限の境界)に従う。ホストの設定と必要な観測の不足、信頼設定の確認先は [doctor](../architecture/modules.md#doctor-の確認項目)、hook の実際の稼働は[セッションの観測](../architecture/data.md#セッションの観測-sessionobservation)と[稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)で確かめる。
