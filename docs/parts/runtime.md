# runtime — 実行

runtime は単一バイナリの `devkit` として、セッション内でループを進める。作業者(セッションのエージェント)と CLI・hook の分担は[全体構成](../architecture.md#全体構成)、設計上の決定は[設計上の決定](../architecture/decisions.md)に従う。

## 責務

- **進行**: runtime は[計画](../architecture/data.md#計画-plan)(課題ごとの実行内容)と [ledger](../architecture/modules.md#ledger)(記録を扱う部品)を使ってループを進める。実装モジュールへの責務配分は [engine](../architecture/modules.md#engine)(進行を担うモジュール)を含む[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。
- **判定者・計画者との接続**: 起動主体は[全体構成](../architecture.md#全体構成)、最終メッセージと来歴の採取・受け渡しは [SubagentStop](../architecture/hooks.md#subagentstop)(サブエージェント終了時のイベント)、起動の定型と結果の受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **git と外部操作**: git 操作と[外部操作](../architecture/flow.md#外部操作)(リポジトリ外への書き込み)の責務配分は [workspace](../architecture/modules.md#workspace) と [external](../architecture/modules.md#external) の契約に従う。
- **組み立て**: 設定と[固定物](../architecture/data.md#所有と生存期間)(kit が提供する資産)の受け渡しを含む組み立ては [cmd/devkit](../architecture/modules.md#cmddevkit) の契約に従う。
- **導入**: 利用側の配置と設定の準備は [install](../architecture/modules.md#install) の契約、配布方法の決定状況は[仮決定27](../architecture/decisions.md#仮決定)に従う。
- **指示ファイルと skill**: kit の操作手順と skill の配置、リポジトリの AGENTS.md・CLAUDE.md との境界は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。

<a id="ファイルの分類と既定の置き場所"></a>

### ファイルの分類と置き場所

ファイルの分類と所有者、更新と生存期間は[所有と生存期間](../architecture/data.md#所有と生存期間)、固定配置と git 管理の範囲は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。

## 入力と出力

- **設定と固定物**: `devkit.toml` の設定と固定物を受け取る。判定コマンド(`[judges]`)・検証コマンド(`[commands]`)(いずれも設定項目)の解析・検証は [config](../architecture/modules.md#config)、固定物の供給とバージョンの照合は [kit](../architecture/modules.md#kit)に従う。
- **ループの雛形 / 計画**: [ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(ループの定義)と計画を受け取り、進行に用いる。[router](../architecture/modules.md#router)(次の行動を選ぶ部品)が判断に使う入力は同節に従う。
- **終了ブロックと成果物**: [ラウンド](../architecture/data.md#ラウンド-round)(作業依頼から判断までの往復)の[終了ブロック](../architecture/data.md#進捗-progress)(作業報告)を受け取る。返却後の[対象のバージョン](../architecture/data.md#対象のバージョン-targetversion)(判定対象の識別値)と成果物の記録は[作業結果の提出](../architecture/flow.md#作業結果の提出)、入力 JSON の場所と拒否条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。
- **判定結果**: 結果ファイルとコマンド実行結果を受け取る。受理から記録までの分担は [check](../architecture/modules.md#check)(観点ごとの判定を扱う部品)と [ledger の契約](../architecture/modules.md#ledger)、結果の種類と照合・集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)、`command:` の check(検証コマンドを実行する check)の定義は[計画](../architecture/data.md#計画-plan)、実行順は[作業結果の判定](../architecture/flow.md#作業結果の判定)に従う。
- **人の入力と訂正**: 観測された人の回答と訂正を受け取る。回答の根拠と受理主体、回答の形式と訂正期限は[回答の受理](../architecture/guarantees.md#回答の受理)、入力の観測は [UserPromptSubmit](../architecture/hooks.md#userpromptsubmit)(人の入力時のイベント)、問いの提示と再開は[人の回答と再開](../architecture/flow.md#人の回答と再開)に従う。
- **指示と表示**: CLI は次の指示と [brief](../architecture/modules.md#brief)(作業や判定の依頼文)、問い、状態、操作の結果を返す。出力上限を超えた場合の返し方は[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。

### ホストの差分と共通の仕組み

対象ホストと共通の処理は[hook の共通処理](../architecture/hooks.md#共通処理)、ホストごとの形式の変換は [host/event](../architecture/modules.md#hostevent)(ホスト形式の変換モジュール)に従う。

利用するイベントと各イベントの入出力は [hook](../architecture/hooks.md)、[checkpoint](../architecture/data.md#checkpoint)(再開用の現在地)の注入のタイミングは [SessionStart](../architecture/hooks.md#sessionstart)(起動・再開などのイベント)に従う。

## 保証

- **完了条件**: 完了の主張の受理と確定は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)・[完了条件](../architecture/guarantees.md#完了条件)に従う。[検出試験](../architecture/guarantees.md#検出試験)(観点ファイルの判定能力の試験)の対象は同節、判定コマンドと `command:` の check の実行は [engine](../architecture/modules.md#engine)、判定者による check の起動と結果の受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **対象と規範**: 判定の対象への結び付けと有効性は [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、規範の固定は[規範の固定](../architecture/guarantees.md#規範の固定)に従う。判定対象の展開と commit 直前の照合は、[展開と永続化](../architecture/guarantees.md#判定対象の展開と完了時の永続化)・[対象の照合](../architecture/guarantees.md#commit-直前の対象の照合)に従う。
- **実行の記録と観測**: [実行の記録](../architecture/data.md#所有と生存期間)(作業が所有する履歴)の書き手の境界と公開方法は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)、排他は[作業の排他制御](../architecture/data.md#作業の排他制御)に従う。網羅性(成果物を記録した範囲)の表示は [view](../architecture/modules.md#view)に従う。
- **再開**: 作業の特定は[セッションとブランチ](../architecture/data.md#セッションとブランチ)、直前の観測による進行の可否は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)、注入する現在地の組み立ては [checkpoint](../architecture/data.md#checkpoint)に従う。
- **外部操作**: 承認済みの操作の実行と再送時の回復は、[外部操作の承認](../architecture/guarantees.md#外部操作の承認)・[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。
- **再計画**: 再計画に伴う [StageRun](../architecture/data.md#ステージの実行-stagerun)(ステージの実行の記録)と計画の切り替えは[計画のバージョン](../architecture/data.md#計画のバージョン)、手順は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **ホストと git に任せる範囲**: サブエージェントの起動と隔離、worktree の扱いは[ホストと git に任せる範囲](../architecture/guarantees.md#実装対象外の機能)、並行作業の配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。
- **メタループの編集**: [メタループ](../meta-loop.md#メタループ)(ループ自体を改善するループ)の編集を宣言したステージの扱いと、人への確認の条件は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **互換性**: バイナリ・固定物・記録形式のバージョンの不一致と、読めない記録は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従って報告する。
- **停止の補助**: 停止の判断は [Stop](../architecture/hooks.md#stop)、完了の確定は[完了条件](../architecture/guarantees.md#完了条件)に従う。
- **checkpoint の注入**: 出力上限を超えた場合の注入内容は[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。
- **登録と実行権限**: hook の登録と `devkit` の実行権限は[権限の境界](../architecture/guarantees.md#権限の境界)に従う。ホストの設定とプロジェクトの信頼(workspace trust / `trust_level`)の確認は [doctor](../architecture/modules.md#doctor-の確認項目)、hook の実際の稼働は[セッションの観測](../architecture/data.md#セッションの観測-sessionobservation)と[稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)で確かめる。

## 試験項目

試験は正本の[試験項目](../architecture/contract-changes.md#試験項目)に従う。
