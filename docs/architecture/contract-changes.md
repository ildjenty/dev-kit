# 契約の改訂と試験

部品の契約は、セッション内の CLI が状態機械を進め、hook の観測を受理する構成に合わせて改訂する。受理条件は [data.md](data.md) と [guarantees.md](guarantees.md) に従う。試験ではモジュールごとの境界と組み合わせを確認する。

## 契約の改訂

### runtime

- workflow を外側のループから [engine の操作](modules.md#engine)へ改訂する。担当の起動と結果の受け取りは[判定結果の受理](guarantees.md#判定結果の受理)、人の入力と訂正は[回答の受理](guarantees.md#回答の受理)・[UserPromptSubmit](hooks.md#userpromptsubmit)へ合わせる。
- 実行の記録と生成物の配置を[利用側のリポジトリ](layout.md#利用側のリポジトリ)へ合わせ、git 管理を選べる仕組みと設定項目を削除する。hook の登録先と sandbox の外での実行を[権限の境界](guarantees.md#権限の境界)へ加える。
- Stop を[停止の補助](hooks.md#stop)として位置付ける。記録の網羅性は [view](modules.md#view)へ合わせる。「Read では大きな文書の全文読み込みを拒否する」を削除する。SubagentStop は結果の受け取りに使い、PostToolUse による skill 編集の検知は[router の分類](guarantees.md#人への確認対象)へ移す。
- ホストの能力の検出試験と代替経路を削除し、[doctor](modules.md#doctor-の確認項目)と[セッションの観測](data.md#セッションの観測-sessionobservation)に置き換える。「並列作業や試作は worktree で行う」「作業の排他」「委任の区別と依存」は[ホストに任せる範囲](guarantees.md#実装対象外の機能)、[作業の排他制御](data.md#作業の排他制御)、[定型の依頼](guarantees.md#判定結果の受理)へ改訂する。
- 束縛とブランチの扱いは[セッションとブランチ](data.md#セッションとブランチ)、hook の直前の観測は[稼働確認](guarantees.md#再開と-hook-の稼働確認)、commit 直前の対象の照合は[commit 直前の対象の照合](guarantees.md#commit-直前の対象の照合)へ合わせる。
- 検証コマンドの登録を [config](modules.md#config)、ステージの check の実行を [engine](modules.md#engine)へ合わせる。pre-commit の実行時点と commit の扱いは[完了条件](guarantees.md#完了条件)に従う。

### repository

- 指示ファイルの開始・終了手順と「AGENTS.md を正本にし CLAUDE.md から取り込む」構成をやめ、kit は指示ファイルに触れない。devkit の動かし方は kit の skill に閉じ、終了手順は [作業結果の提出](flow.md#作業結果の提出)に従う。記録ディレクトリは[利用側の配置](layout.md#利用側のリポジトリ)へ置き換える。
- hook の登録先を[権限の境界](guarantees.md#権限の境界)へ合わせ、ホスト設定・サブエージェント定義・rules を[育てるもの](data.md#所有と生存期間)に含める。

### ledger

- checkpoint、閲覧、カーソル、確認済みの集合を [checkpoint](data.md#checkpoint)と[状態変更](data.md#決定と証拠-decision--evidence)の派生値へ合わせる。
- 保存は[記録の書き込みと排他](data.md#記録の書き込みと排他)へ合わせ、「改行・タブを除く」という制限を削除する。実行状態は [StageRun](data.md#ステージの実行-stagerun)へ合わせ、「異常終了・不明」を削る。
- 途中の記録は [note](data.md#決定と証拠-decision--evidence)、検出試験の正本と実行の事実は [DetectionTest](data.md#検出試験の記録-detectiontest)、判定の失効は [Judgment](data.md#check-の結果-judgment)へ合わせる。記録範囲の照合の節は削除し、網羅性は[成果物の記録](modules.md#view)から導く。
- 指示と出し直しは [Instruction](data.md#指示-instruction)、発行した指示の集合は [Round](data.md#ラウンド-round)、結果ファイル名は [SubagentStop](hooks.md#subagentstop)へ合わせる。回答の本文と出所は [Answer / Approval](data.md#回答承認-answer--approval)へ加える。

### router

- ステージの[完了条件](guarantees.md#完了条件)を定め、Stop と共有する記述を改訂する。[人への確認対象](guarantees.md#人への確認対象)と [hook の稼働不足](guarantees.md#再開と-hook-の稼働確認)を反映する。
- `command:` の check の結果は [Judgment](data.md#check-の結果-judgment)として扱い、pre-commit を伴う commit の成否と合わせた完了の判断は[完了条件](guarantees.md#完了条件)へ合わせる。
- ロールの語彙を作業者・人・判定者・計画者に揃え、分類表の出力をサブエージェントまたは判定コマンドの名前にする。

### check

- 実行主体を [Plan の担当と観点](data.md#計画-plan)へ合わせる。「作成と判定のコンテキスト ID と履歴継承設定を照合する」「会話の内容を知らないサブエージェント」は[判定結果の受理](guarantees.md#判定結果の受理)と [PreToolUse](hooks.md#pretooluse)へ改訂する。
- 「Codex ではモデル名を明示する」はリポジトリ内の定義と[起動時の上書きの制限](hooks.md#pretooluse)へ改訂する。`command:` の check の実行結果から Judgment への変換は[作業結果の判定](flow.md#作業結果の判定) と[受理境界](guarantees.md#判定結果の受理)へ合わせる。修正ラウンドの依頼と Finding は[指摘](data.md#指摘-finding)へ合わせる。
- transcript・定義・実効条件の照合、結果の選択、判定の食い違いは[判定結果の受理](guarantees.md#判定結果の受理)を参照する。

### form

- 終了ブロックを [submit の入力](data.md#進捗-progress)とし、判定者の最終メッセージ、指示、問い、決定と証拠のスキーマを [form](modules.md#form)へ合わせる。playbook は[skill の配置](layout.md#指示ファイルとホスト設定)へ統合する。
- 問いの回答案内は [Question](data.md#問い-question)、選択肢と文章の回答・訂正期限は[回答の受理](guarantees.md#回答の受理)、表示上の範囲と入力 JSON の場所は[状態遷移と受理条件](data.md#状態遷移と受理条件)へ合わせる。

### view

- 待ち・失効・最後の観測時刻・網羅性、構造図と git の履歴の入力元を [view](modules.md#view)へ合わせる。
- 閲覧と確認済みの集合は[状態変更](data.md#決定と証拠-decision--evidence)、問いの表示と停止は[状態遷移と受理条件](data.md#状態遷移と受理条件)を参照する。

### loop-plan

- [雛形](data.md#ループの雛形-plantemplate)(利用者が組む)と[計画](data.md#計画-plan)(課題ごと)の2層に分け、フォーマットを JSON(英語キー)で定める。例も雛形と計画に分ける。
- 担当と check の実行主体の表記を [Plan](data.md#計画-plan)へ合わせ、計画の check の確認事項には雛形からの逸脱を含める。`command:` の check と pre-commit の関係は[完了条件](guarantees.md#完了条件)に従う。
- 計画の承認で足りる外部操作の宣言を加え、[外部操作の承認](guarantees.md#外部操作の承認)の識別項目へ合わせる。

### skill の手順

- 作業の進め方は[処理の流れ](flow.md)、人への確認は[人の回答と再開](flow.md#人の回答と再開)、判定者・計画者の起動は[定型の依頼](guarantees.md#判定結果の受理)に合わせる。
- コマンドの呼び方は[権限の境界](guarantees.md#権限の境界)、コミットの扱いは[完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)を参照する。

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

- 集約、ラウンド数、進展、上限、停止分類の組み合わせ。
- [人への確認対象](guarantees.md#人への確認対象)の既定と追記。
- [hook の稼働不足](guarantees.md#再開と-hook-の稼働確認)を含む次の指示との接続。

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
- [起動時の書き込み試験](guarantees.md#ホスト設定と実行権限)が記録を残さず失敗し、理由を返すこと。

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
