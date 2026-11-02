# 開始から完了まで

人が課題を渡すと、計画、作業、検証、判定の順に進む。エージェントは `devkit` の指示を実行し、`devkit` は結果を記録して次の行動を決める。人の回答や外部操作を待つ間は現在地を保持し、別のセッションでも記録から再開する。

## 開始

人は通常どおりセッションを開き、kit の skill で課題を伝える。エージェントが `devkit start <課題>` を実行すると、engine は作業 ID とブランチを作り、基点コミットと kit のバージョンを記録して[セッションを束縛](data.md#セッションとブランチ)する。返すのは計画者への指示 ID と `brief/<指示 ID>.md` のパスである。

エージェントは `devkit-planner` を、[定型の prompt](guarantees.md#判定結果の受理)で起動する。計画者が最終メッセージで計画を返すと、[SubagentStop](hooks.md#subagentstop)が結果を保存する。次の `devkit next` で engine が結果を受理し、JSON Schema で検証して提案中の計画として記録する。

## 計画の check とゲート

`devkit next` は計画の check の指示 ID と判定依頼のパスを返す。エージェントは `devkit-judge` を起動し、hook を経由した結果を次の `next` が受理して集約する。`FAIL` なら指摘を渡して計画者への指示を返す。`PASS` でゲートが必要なら問いを記録し、承認待ちにする。

エージェントが問いをチャットで見せ、人が回答し、`devkit answer <問い ID>` で記録する。回答の条件は[回答の受理](guarantees.md#回答の受理)、実行可能になる条件と作り直しは[計画のバージョン](data.md#計画のバージョン)に従う。必要な[検出試験](guarantees.md#検出試験)も、作業のラウンドを開く前に `next` の指示で済ませる。

## 作業の指示

`devkit next` は現在の StageRun とラウンドを特定する。[hook の稼働](guarantees.md#再開と-hook-の停止)を確認し、[対象のバージョン](data.md#対象のバージョン-targetversion)を取得して、識別したセッション ID をラウンドに結び付ける。engine は brief をファイルに書き、[出力上限](guarantees.md#性能と出力)に応じて返す。人が担当するステージなら問いを起票し、回答待ちにする。

brief にはステージ定義、前のステージの出力、リポジトリの事実(配置ガイド、`rules.md` の元に戻せないもの、利用可能な check)を含める。修正ラウンドには前回の指摘と指摘 ID も含める。規範の参照先は [kit のコピー](modules.md#kit)と `.devkit/checks/` であり、読み出しは[規範の固定](guarantees.md#規範の固定)に従う。

エージェントはセッション内で作業し、他の skill や plugin も使える。途中の決定や調査結果は `devkit note`、作業の終了ブロックは `devkit submit <ファイル>` で渡す。入力 JSON の場所と受理の条件は[進行と受理](data.md#進行と受理)に従う。コミットは[完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)に委ねる。

## submit と verify

engine は終了ブロックを受理し、[検証対象の一致](guarantees.md#検証対象の一致)の手順で成果物と StageRun を更新して、「`devkit verify` を実行する」と返す。エージェントは Bash の timeout を上限値に設定して `devkit verify` を実行する。

`devkit` が検証コマンド、機械の check、登録コマンドの判定を実行し、成功記録と Judgment を書く。検証によって対象が変わった場合の扱いも[検証対象の一致](guarantees.md#検証対象の一致)に従う。中断後は `verify` を再実行できる。待ち方は[作業の排他](data.md#作業の排他)に従い、判定者による check が必要なら StageRun を判定待ちにする。

## 判定

`devkit next` は判定依頼を `brief/<指示 ID>.md` に書き、[対象を展開](guarantees.md#判定対象の展開と完了時の永続化)して、指示 ID と展開先のパスを返す。依頼には規範、観点ファイル、対象の展開先、最終メッセージとして返す JSON の形式、修正ラウンドなら前回の指摘 ID を含める。

エージェントが判定者を起動し、判定者は結果を最終メッセージで返す。hook が観測を添えて保存し、次の `next` が[判定結果の受理](guarantees.md#判定結果の受理)に従って Judgment を記録・集約する。

## router の判断

engine は [Finding と完了条件 ID](data.md#指摘-finding)の差分から進展を計算する。router は集約した判定、ラウンド数、進展の有無、計画の上限、停止分類を受け取って判断し、engine が記録する。

- **修正する**: 次の `next` が指摘を取り込んだ brief を返す。
- **人に確認する**: 問いを記録し、StageRun を回答待ちにして、`next` が問いの指示を返す。[既定の分類](guarantees.md#人に確認する分類)を使う。
- **前のステージに戻る**: [再計画の扱い](data.md#計画のバージョン)に従って旧 StageRun と提案を更新し、`next` が計画者への指示を返す。
- **完了する**: [完了条件](guarantees.md#完了条件)を照合してステージ完了を記録し、workspace が[永続化](guarantees.md#判定対象の展開と完了時の永続化)する。次のステージへ進み、最後のステージなら作業完了を記録して、`next` が完了の指示を返す。

## 人の回答と再開

人の判断が必要なら、エージェントはチャットで問いを見せてセッションを終える。人の入力を hook が観測し、エージェントが `devkit answer <問い ID>` で記録して `devkit next` で続ける。`answer` は[決定の状態変更](data.md#決定と証拠-decision--evidence)も受け付ける。訂正は[回答の受理](guarantees.md#回答の受理)に従う。

セッションをまたぐ再開はホストのセッション再開と [SessionStart](hooks.md#sessionstart)の注入で行う。停止の可否は [Stop](hooks.md#stop)、引き継ぐ状態は [checkpoint](data.md#checkpoint)に従う。

## 外部操作

エージェントは `devkit external <種別> <入力ファイル> --request <要求 ID>` を実行する。engine が[操作を識別](guarantees.md#外部操作の二重適用)し、[承認の条件](guarantees.md#外部操作の承認)に従って受理する。確認待ちになれば、人の回答を `answer` で記録してから同じ呼び出しを再実行する。external が実行と結果の記録を担い、結果を返す。

## hook

各段階のホストイベントは [5つの hook](hooks.md)が処理する。入力・判断・出力・書く生成物と、サブエージェント内や束縛のないセッションでの扱いはそちらで定める。
