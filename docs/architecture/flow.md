# 処理の流れ

人が課題を渡すと、計画、作業、判定、完了時のコミットの順に進む。エージェントは `devkit` の指示を実行し、`devkit` は結果を記録して次の行動を決める。人の回答や外部操作を待つ間は現在地を保持し、別のセッションでも記録から再開する。各段階の進行には[中断](guarantees.md#中断)の照合を適用する。

## 作業の開始

人は通常どおりセッションを開き、kit の skill で課題を伝える。エージェントは `devkit start --plan <name> <課題>` で[ループの雛形](data.md#ループの雛形-plantemplate)(再利用するループの定義)を指定する。`start --plan <name>` の解決先は `.devkit/plans/` だけとし、`--plan` の省略時はリポジトリ側の `.devkit/plans/default.json` を使う。[engine](modules.md#engine)(進行を担うモジュール)は作業 ID とブランチ `devkit/<作業 ID>` を作ってチェックアウトを切り替え、基点コミットと kit のバージョンを記録して[セッションを束縛](data.md#セッションとブランチ)する。返すのは計画者への指示 ID と `brief/<指示 ID>.md` のパスである。

エージェントは `devkit-planner` を、[定型の prompt](guarantees.md#判定結果の受理)で起動する。計画者は[計画](data.md#計画-plan)(雛形から作る課題ごとの実行内容)の項目を記入する。通常の取捨では、完了条件を書き、課題に関係のないステージ(作業を区切る単位)を外し、[check](modules.md#check)(評価と判定)の対象を指定し、必要なら調査・試作のステージを雛形の候補から足す。課題固有の元に戻せないものと外部操作(リポジトリ外への書き込み行為)の宣言も計画者が記入する。構成の変更が逸脱に当たる条件と承認は[計画の定義](data.md#計画-plan)に従う。計画者が最終メッセージで計画を返すと、[SubagentStop](hooks.md#subagentstop)が結果を保存する。次の `devkit next` で engine が結果を受理し、JSON Schema で検証して提案中の計画として記録する。

## 計画の check と承認ゲート

`devkit next` は計画の check の指示 ID と判定依頼のパスを返す。エージェントは `devkit-judge` を起動し、hook を経由した結果を次の `next` が受理して集約する。[判定値](data.md#check-の結果-judgment)が `FAIL` の場合の再提案と人への確認は[計画のバージョン](data.md#計画のバージョン)に従う。`PASS` でゲート(人の承認を待つ関門)が必要なら問いを記録し、承認待ちにする。雛形からの逸脱の承認条件は[計画](data.md#計画-plan)、宣言した外部操作に計画の承認を使える条件は[外部操作の承認](guarantees.md#外部操作の承認)に従う。

承認待ちからは[人の回答と再開](#人の回答と再開)に従って進める。実行可能になる条件と作り直しは[計画のバージョン](data.md#計画のバージョン)に従う。必要な[検出試験](guarantees.md#検出試験)(観点ファイルの判定能力を確かめる試験)も、作業のラウンドを開く前に `next` の指示で済ませる。

## 作業の指示

`devkit next` は現在の [StageRun](data.md#ステージの実行-stagerun)(ステージの実行記録)と[ラウンド](data.md#ラウンド-round)(作業の指示から次の行動の確定までの往復)を特定する。[hook の稼働](guarantees.md#再開と-hook-の稼働確認)を確認し、[対象のバージョン](data.md#対象のバージョン-targetversion)(評価対象を識別する値)を取得して、識別したセッション ID をラウンドに結び付ける。engine は [brief](modules.md#brief)(作業や判定の依頼文)をファイルに書き、[出力上限](guarantees.md#性能と出力の上限)に応じて返す。人が担当するステージの扱いは[計画の担当](data.md#計画-plan)に従う。

engine は [brief の責務](modules.md#brief)で定める入力を解決して brief を組み立てる。観点ファイル(check の確認事項を定めるファイル)の参照元と、規範全体の読み出しは[規範の固定](guarantees.md#規範の固定)に従う。

エージェントはセッション内で作業し、他の skill や plugin も使える。途中の決定や調査結果は `devkit note`、作業の[終了ブロック](data.md#進捗-progress)(作業結果を固定形式で返す JSON)は `devkit submit <ファイル>` で渡す。教訓も `note` で渡し、分類と記録は [engine の責務](modules.md#engine)に従う。入力 JSON の場所と受理の条件は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。コミットは[完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)に委ねる。

## 作業結果の提出

engine は `submit` で[状態遷移と受理条件](data.md#状態遷移と受理条件)に従って終了ブロックを受理し、返却後の[スナップショット](data.md#対象のバージョン-targetversion)を取得して成果物を記録し、終了ブロックの終了の種別にかかわらず StageRun を判定待ちにする。次の `next` が[作業結果の判定](#作業結果の判定)を進め、終了の種別は [router](modules.md#router)(次の行動を分類する部品)の判断にも渡す。

## 作業結果の判定

engine は `next` の中で、ステージに置いた `command:` の check(終了コードによる検証)をすべて判定者の check より先に実行する。雛形の列挙順は同じ種類の中だけ有効とする。コマンドの実行場所と実行前後の照合は[判定対象の展開と完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)に従う。環境変数 `DEVKIT_STAGE=<ステージ名>` と `DEVKIT_WORK=<作業 ID>` を渡す。終了コードと出力を [Judgment](data.md#check-の結果-judgment)(check の結果)に変換し、終了コードが0なら `PASS`、0以外なら `FAIL`、期限超過は `NOT ASSESSED` とする。必須の `command:` の check が `FAIL` なら、pre-commit も判定者も実行せず、engine が「修正する」として記録して出力を次の brief に含める。

続いて、同じ `next` の中で判定の指示を返す前に pre-commit を実行する。実行の可否・入力条件・失敗時の扱いは[完了条件](guarantees.md#完了条件)に従う。

`devkit next` は判定依頼を `brief/<指示 ID>.md` に書き、[対象を展開](guarantees.md#判定対象の展開と完了時の永続化)して、指示 ID と展開先のパスを返す。そのラウンドの終了ブロックを含む判定依頼の内容は [check の契約](modules.md#check)、観点「差分」の有効な承認・委任の一覧は[判定結果の受理](guarantees.md#判定結果の受理)に従う。

エージェントが判定者サブエージェントを起動し、結果はその最終メッセージで返る。hook が観測を添えて保存し、次の `next` が[判定結果の受理](guarantees.md#判定結果の受理)に従って Judgment を記録・集約する。判定コマンドを担当とする判定は `devkit` が実行し、同じ受理境界に従う。

定義を編集したラウンドの完了手順は[サブエージェント定義の変更と完了](../architecture.md#サブエージェント定義の変更と完了)に残す。

## router の判断

engine は [Finding と完了条件 ID](data.md#指摘-finding)の差分から進展を計算し、[router の入力](modules.md#router)を渡して判断を呼び出し、結果を記録する。終了ブロックの終了の種別の定義は[進捗](data.md#進捗-progress)に従う。

- **修正する**: 次の `next` が指摘を取り込んだ brief を返す。
- **人に確認する**: 問いを記録し、StageRun を回答待ちにして、`next` が問いの指示を返す。[既定の分類](guarantees.md#人への確認対象)を使う。
- **前のステージに戻る**: [再計画の扱い](data.md#計画のバージョン)に従って旧 StageRun を再計画待ちにし、`next` が計画者への指示を返す。
- **完了する**: [workspace](modules.md#workspace)(git 操作を担うモジュール)に完了処理を依頼する。確定と失敗時の扱いは[完了条件](guarantees.md#完了条件)に従い、完了が確定したら次のステージへ進む。最後のステージなら作業完了を記録して、`next` が完了の指示を返す。

## 人の回答と再開

人の判断が必要なら、エージェントはチャットで問いを見せてターンを終える。人の入力を hook が観測し、エージェントが `devkit answer <問い ID>` で記録して `devkit next` で続ける。決定を確認する問いと回答の訂正も[回答の受理](guarantees.md#回答の受理)に従う。

人が計画の変更を提案する場合は、エージェントが `devkit plan <ファイル>` で提案を記録する。撤回は `devkit plan --withdraw` で記録し、その後の進行は[計画のバージョン](data.md#計画のバージョン)に従う。作業を中止する場合は、エージェントが `devkit abort` を呼ぶ。

セッションをまたぐ再開はホストのセッション再開と [SessionStart](hooks.md#sessionstart)の注入で現在地を引き継ぐ。人による中断からの再開条件は[中断](guarantees.md#中断)に従う。新しいセッションからの束縛とブランチ不一致時の扱いは[セッションとブランチ](data.md#セッションとブランチ)、指示の出し直しは[指示](data.md#指示-instruction)に従う。停止の可否は [Stop](hooks.md#stop)、引き継ぐ状態は [checkpoint](data.md#checkpoint)に従う。

## 人による中断

中断の入口、Stop の判断、再開と継続上限は[中断の保証](guarantees.md#中断)に従う。観測と記録の対応は[中断の観測](data.md#中断の観測-pauseobservation)、中断前の現在地との関係は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。

## 外部操作

エージェントは `devkit external <種別> <入力ファイル> --request <要求 ID>` を実行する。入力ファイルの場所は[状態遷移と受理条件](data.md#状態遷移と受理条件)を参照する。engine が[操作を識別](guarantees.md#外部操作の二重適用)し、[承認の条件](guarantees.md#外部操作の承認)に従って受理する。確認待ちになれば、人の回答を `answer` で記録してから同じ呼び出しを再実行する。external が実行と結果の記録を担い、結果を返す。

## hook

各段階のホストイベントは [5つの hook](hooks.md)が処理する。入力・判断・出力・書く生成物と、サブエージェント内や束縛のないセッションでの扱いはそちらで定める。
