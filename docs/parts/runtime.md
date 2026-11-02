# runtime — 実行

runtime は単一バイナリの `devkit` として、Claude Code または Codex CLI のセッション内でループを進める。作業者はセッション自身であり、CLI が進行を判断し、同じバイナリの hook がホストイベントを観測する([全体構成](../architecture.md#全体構成))。

## 責務

- **進行**: [engine](../architecture/modules.md#engine) が計画と ledger から次の指示を計算し、結果を受理して状態機械を進める。brief・check・router・ledger・form・view との分担と runtime を構成する実装モジュールは、[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。
- **判定者・計画者との接続**: セッションのエージェントがサブエージェントを起動し、hook が最終メッセージと来歴を採取して CLI に渡す。起動の定型と結果の受理は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **git と外部操作**: git 操作は [workspace](../architecture/modules.md#workspace)、承認された外部への書き込みと回復は [external](../architecture/modules.md#external) が担う。
- **組み立て**: [cmd/devkit](../architecture/modules.md#cmddevkit) が workspace を使ってルートとブランチを解決し、設定と固定物を各モジュールへ渡す。ホストイベントの変換と出力の整形は [host/event](../architecture/modules.md#hostevent) が担う。
- **導入**: [install の `init`](../architecture/modules.md#install) が利用側の配置と設定を用意する。バイナリの配置を含む配布方法は[仮決定](../architecture/decisions.md#仮決定)にある未決定事項として扱う。

## 入力と出力

- **ループの雛形 / 計画**: 利用者が組む[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)と、計画者が課題ごとに作る[計画](../architecture/data.md#計画-plan)を扱う。次の行動の判断に使う入力と完了条件の照合は [router](../architecture/modules.md#router)に従う。
- **終了ブロックと成果物**: `submit` が[進捗の終了ブロック](../architecture/data.md#進捗-progress)を受け取り、返却後の対象のバージョンと成果物を記録する。入力 JSON の場所と拒否条件は[状態遷移と受理条件](../architecture/data.md#状態遷移と受理条件)に従う。
- **判定結果**: サブエージェントの結果ファイル、判定コマンド(`[judges]`)の結果、`command:` の check の実行結果を受理する。対象のバージョン・規範・来歴の照合と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)、実行順は[作業結果の判定](../architecture/flow.md#作業結果の判定)に従う。
- **人の入力と訂正**: [UserPromptSubmit](../architecture/hooks.md#userpromptsubmit) の観測を根拠に、CLI が回答を受理する。選択肢と文章の回答、訂正期限は[回答の受理](../architecture/guarantees.md#回答の受理)、問いの提示と再開は[人の回答と再開](../architecture/flow.md#人の回答と再開)に従う。
- **設定と固定物**: `devkit.toml` の解析・検証と、判定コマンド(`[judges]`)・検証コマンド(`[commands]`)を含む設定項目は [config](../architecture/modules.md#config)、固定物の供給とバージョンの照合は [kit](../architecture/modules.md#kit)に従う。
- **指示と表示**: CLI は次の指示と brief、問い、状態、操作の結果を返す。出力の全文と要点・ファイルのパスの切り替えは[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。

## 保証

- **完了条件**: 作業者の完了の主張と、CLI による完了の確定を区別する。必須 check、検出試験、判定前の pre-commit、完了時の `git commit` と失敗時の扱いは[完了条件](../architecture/guarantees.md#完了条件)、ステージの check の実行は [engine](../architecture/modules.md#engine)に従う。
- **対象と規範**: 判定を[対象のバージョン](../architecture/data.md#対象のバージョン-targetversion)に結び付け、[規範の固定](../architecture/guarantees.md#規範の固定)と[Judgment の有効性](../architecture/data.md#check-の結果-judgment)に従って照合する。判定対象の展開と commit 直前の照合は、[展開と永続化](../architecture/guarantees.md#判定対象の展開と完了時の永続化)・[対象の照合](../architecture/guarantees.md#commit-直前の対象の照合)に従う。
- **実行の記録と観測**: ledger への書き込みは CLI、hook の書き込みは生成物に分担する。公開方法と排他は[記録の書き込みと排他](../architecture/data.md#記録の書き込みと排他)・[作業の排他制御](../architecture/data.md#作業の排他制御)、網羅性の表示は [view の成果物を記録した範囲](../architecture/modules.md#view)に従う。
- **再開**: 計画と ledger から現在地を復元し、[セッションとブランチ](../architecture/data.md#セッションとブランチ)に従って作業を特定する。直前の観測による進行の可否は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)、注入する現在地は ledger から組み立てる [checkpoint](../architecture/data.md#checkpoint)に従う。
- **外部操作**: 実行の承認と、成功済み・実行中・結果不明の操作への再送を区別する。成功済みの記録の返却と、実行中・結果不明からの照会または人への確認は、[外部操作の承認](../architecture/guarantees.md#外部操作の承認)・[外部操作の二重適用](../architecture/guarantees.md#外部操作の二重適用)に従う。
- **再計画**: router が「前のステージに戻る」と判断した後は、計画者が残りの計画を作り直す。旧 StageRun の再計画待ち、計画の check と必要な承認、新しいバージョンへの切り替えは[計画のバージョン](../architecture/data.md#計画のバージョン)と [router の判断](../architecture/flow.md#router-の判断)に従う。
- **隔離と委任**: サブエージェントの起動と隔離、worktree の扱いは[ホストと git に任せる範囲](../architecture/guarantees.md#実装対象外の機能)、並行作業の配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に従う。
- **harness の改善とメタループの編集**: メタループの編集を宣言したステージと、それ以外のステージの確認対象は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。検出試験は[判定者が担当する観点ファイル](../architecture/guarantees.md#検出試験)を対象とする。
- **互換性**: バイナリ・固定物・記録形式のバージョンの不一致と、読めない記録は[ロールアウト](../architecture/guarantees.md#ロールアウト)に従って報告する。

## ファイルの分類と既定の置き場所

育てるもの / 固定物 / 実行の記録 / 生成物の分類と所有者、更新と生存期間は[所有と生存期間](../architecture/data.md#所有と生存期間)に従う。配置は[利用側のリポジトリ](../architecture/layout.md#利用側のリポジトリ)に固定し、固定物はリポジトリ内のコピーから供給し、実行の記録と生成物は git 管理から除外した `.devkit/run/` に置く。

## ホストの差分と共通の仕組み

対象ホストは Claude Code と Codex CLI とし、どちらも既定にしない。共通の処理とホストごとの差は、[hook の共通処理](../architecture/hooks.md#共通処理)と [host/event](../architecture/modules.md#hostevent)に従う。

- **kit が使う hook**: SessionStart・Stop・PreToolUse・SubagentStop・UserPromptSubmit の5イベントを使う。各イベントの入力・判断・出力・書く生成物は [hook](../architecture/hooks.md)に従う。
- **停止の補助**: Stop は次の指示と観測内の継続回数・上限から停止の可否を決める。完了条件の判定とは分け、[Stop](../architecture/hooks.md#stop)の契約に従う。
- **checkpoint の注入**: [SessionStart](../architecture/hooks.md#sessionstart) が起動・再開・compaction 後に現在地を注入する。上限内の本文と、超過時の要約・`devkit status` の案内の切り替えは[性能と出力の上限](../architecture/guarantees.md#性能と出力の上限)に従う。
- **登録と実行権限**: hook の登録先、ホスト設定への追記、`devkit` の sandbox の外での実行は[権限の境界](../architecture/guarantees.md#権限の境界)に従う。設定と信頼状態は [doctor](../architecture/modules.md#doctor-の確認項目)、hook の実際の稼働は[セッションの観測](../architecture/data.md#セッションの観測-sessionobservation)と[稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)で確かめる。
- **指示ファイルと skill**: kit の操作手順と skill の配置、リポジトリの AGENTS.md・CLAUDE.md との境界は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)に従う。

## 試験項目

- [engine の試験](../architecture/contract-changes.md#engine)に従い、操作列による遷移、指示の再現性と重複、古い結果の拒否、セッションとブランチの束縛、hook の稼働不足時の受理境界を確認する。
- [hook の試験](../architecture/contract-changes.md#hook)に従い、両ホストの固定イベント JSON で5イベントを確認する。束縛のないセッション、サブエージェント内、Stop の継続回数、起動の定型、判定者の来歴、人の入力と訂正の観測を含める。
- [結合試験](../architecture/contract-changes.md#結合試験)に従い、固定配置での保存と再開、ledger から組み立てた checkpoint の別セッション・compaction 後の注入、待ち状態と他の skill・plugin との共存を確認する。
- [engine](../architecture/contract-changes.md#engine) と [workspace](../architecture/contract-changes.md#workspace) の試験に従い、`command:` の check・pre-commit・判定の実行順、必須コマンド失敗時の起動抑止、commit 直前の照合、コミットの成否と次の brief への接続を確認する。
- [plan](../architecture/contract-changes.md#plan) と [engine](../architecture/contract-changes.md#engine) の試験に従い、計画の作り直しのバージョンと差分、check と承認、旧 StageRun の再計画待ちと切り替えを確認する。
- [external の試験](../architecture/contract-changes.md#external)に従い、承認と実行の順序、実行直後の中断、成功済み・実行中・結果不明への再送で二重適用しないことを確認する。
- [ledger/fsstore](../architecture/contract-changes.md#ledgerfsstore)・[install](../architecture/contract-changes.md#install)・[kit](../architecture/contract-changes.md#kit) の試験に従い、記録と生成物の公開・排他、導入の冪等性と既存設定の保持、doctor、固定物とバージョンの照合を確認する。

## 設計上の決定

採用する構成と未決定事項は[仮決定](../architecture/decisions.md#仮決定)、外側のプロセスによる担当の起動など採用しない構成は[不採用の案](../architecture/decisions.md#不採用の案)、確認が必要なホストの挙動は[未確認の前提](../architecture/decisions.md#未確認の前提)に従う。
