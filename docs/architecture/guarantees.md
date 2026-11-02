# 保証と対策

kit は事故、自己評価の偏り、再開時の食い違い、バージョンの不一致を防ぐ。保証の対象は、記録上、正しくない完了・承認・外部操作が成立しないことである。作業者と評価対象のコードに悪意がないことを前提に、ホストの権限管理と `devkit` の受理境界を組み合わせる。

## 信頼の前提

作業者(セッションのエージェント)と評価対象のコードに悪意はないものとする。記録や承認の偽造、規範を書き換えて check を通す細工、資格情報の窃取は、同じ OS 利用者の権限で動く kit では防げないため範囲外とする。チームでは育てるものを git 管理し、PR レビューで確認できるようにする。[principles/harness.md](../../principles/harness.md) の原則5に従い、この前提を超える保証の仕組みは設けない。記録には秘密情報を書かない。

pre-commit hook・判定コマンド・検証コマンドの権限および人のレビューは[権限の境界](#権限の境界)に従う。エージェントが書いたテストコードも同じ権限で動くことを信頼の前提に含める。

<a id="実装対象外の機能"></a>

## ホスト・git と kit の責務分担

git と、Claude Code・Codex CLI の両方にある機能で賄えるものは kit で実装しない。[principles/designing.md](../../principles/designing.md) の原則5に従う。

- ブランチ、worktree、ignore、履歴、アーカイブ、利用者の名前は git に任せる。
- sandbox、hook の登録と実行の仕組み、permissions、skill、指示ファイル、サブエージェントの起動と隔離、セッション再開と compaction、人とのやり取りはホストに任せる。
- kit が持つのは、計画・依頼・判定・分類・形式(plan・brief・check・router・form)、記録と保存(ledger・ledger/fsstore)、進行と外部操作(engine・external)、ホストとの接続(hook・host/event)、git 操作(workspace)、設定・導入・固定物(config・install・kit)、表示(view)、組み立て(cmd/devkit)である。契約は[モジュール](modules.md#モジュールごとの契約)に従う。
- `[deny]` は束縛のあるセッションに限って効かせるため kit が照合する。`permissions.deny` では範囲が広すぎる。

## 防ぐ失敗と対策

### 手順の省略と完了の過大申告

- **失敗**: エージェントが進行を決め、手順を飛ばしたり、完了条件を満たさず完了を主張したりする。
- **仕組み**: ステージ・ラウンド、次の指示、完了条件、記録、結果と答えの受理は `devkit` が計算する。エージェントは実行と返却を担う。手順を飛ばしても記録上の状態は進まない。[状態遷移と受理条件](data.md#状態遷移と受理条件)に従って同じ指示を返す。
- **置き場所**: `internal/engine/` の `next`・`submit` と `internal/plan/`・`internal/router/` の遷移、以下の[完了条件](#完了条件)。

### 完了条件

- **失敗**: 未検証の変更や未判定の要求を完了に含める。
- **仕組み**: ステージの完了には、返却後の対象のバージョンに対するすべての必須 check(`command:` の check を含む)の `PASS`、必須 check のうち判定者が担当する観点とその担当に対する検出試験の合格、そして `devkit` が行う `git commit` の成功が必要である。検出試験の対象は[検出試験](#検出試験)に従う。Judgment の有効性は [check の結果](data.md#check-の結果-judgment)に従う。Stop hook はこの条件を照合しない。
- **置き場所**: `internal/plan/` の完了条件の規則を `internal/router/` が照合し、`internal/engine/` が `internal/workspace/` にコミットを依頼してステージ完了を記録する。

完了の確定では、条件を満たさない完了の主張を完了として扱わない。ステージごとに選ぶ検証は雛形の check として置き、pre-commit はステージに関係なく完了時のコミットで必ず通る検証である。devkit は検証の集合を自分で決めず、省略も監視しない。何を置くかは雛形を組む人が決め、[雛形からの逸脱](data.md#計画-plan)は計画の check と人のゲートで見る。

`devkit` は完了時に、返却後のスナップショットと同じ内容の index で本物の `git commit` を実行する。`--no-verify` は使わず、pre-commit hook がリポジトリで定義した検証を走らせる。commit の失敗は router に渡さず engine が「修正する」として記録し、hook の出力を次の brief に含める。commit の成功がその対象のバージョンに対する pre-commit の検証の記録であり、保存項目は [StageRun の完了記録](data.md#ステージの実行-stagerun)に従う。pre-commit の別の検証結果の記録は持たない。

commit 直前は[対象の照合](#commit-直前の対象の照合)に従う。

commit 時には環境変数 `DEVKIT_STAGE=<ステージ名>` と `DEVKIT_WORK=<作業 ID>` を渡す。

hook の共有は git の `core.hooksPath` による。`init` は、リポジトリに hook の仕組み(husky、pre-commit フレームワーク、既存の `core.hooksPath`)がなければ `.githooks/` を作って `core.hooksPath` を向け、空の pre-commit を置く。既にあれば触らない。`doctor` は pre-commit が存在することを確認する。

フォーマッタが pre-commit でファイルを書き換え、hook が非0で終了した場合は commit が失敗し、作業ツリーに差分が残る。commit の失敗は前述の扱いで次のラウンドに渡す。[Git の pre-commit の仕様](https://git-scm.com/docs/githooks#_pre_commit)では、ファイルの書き換えだけで commit の失敗は保証されない。判定前も `git hook run pre-commit`(git 2.36 以降)で同じ hook を実行する。commit 時と同じ入力条件になるよう、返却後のスナップショットと同じ内容の一時 index を `GIT_INDEX_FILE` で与え、作業ツリーから一時 index で `write-tree` を取ってスナップショットと一致することを確かめてから実行する。`DEVKIT_STAGE` と `DEVKIT_WORK` も commit 時と同じく渡す。失敗は router に渡さず engine が「修正する」として記録し、hook の出力を次の brief に含める。実行順は[作業結果の判定](flow.md#作業結果の判定)に従う。`init` が git のバージョンを確認し、使えない環境では pre-commit による検証は完了時の commit だけで行う。判定前の実行は早く失敗を知るためのもので、pre-commit による完了条件の記録は commit の成功だけである。

### 規範の固定

- **失敗**: ラウンド中に規範を書き換え、そのラウンドの判定を有利にする。
- **仕組み**: [規範の識別](data.md#check-の結果-judgment)に含まれるものはすべて、ラウンド開始時のスナップショットから `git show` で読む。例えば、観点ファイル、検証コマンドと判定コマンドの定義、分類表の追記、`[deny]`、サブエージェントの定義、rules.md、ループの雛形、kit のコピー、指示ファイル(AGENTS.md と CLAUDE.md)が含まれる。編集中の値は次のラウンドから適用する。変更は対象のバージョンの差分に現れ、確認条件は[人への確認対象](#人への確認対象)に従う。
- **置き場所**: `internal/engine/` のラウンド開始と、`internal/workspace/` の読み出し。hook の deny の読み出しもこの境界に従う。

ラウンド外の判定(計画の check、検出試験)では、指示を出す時点でスナップショットを取り、規範の識別・読み出しと Judgment の有効性の照合に使う。ラウンド外の `[deny]` も同じスナップショットから読む。観点ファイルの参照元は kit のコピー(`.devkit/kit/<バージョン>/checks/`)と `.devkit/checks/` の両方である。

ラウンドも判定指示もない期間の規範の参照元は[指示のない期間の規範](decisions.md#指示のない期間の規範)に残す。

### 判定結果の受理

- **失敗**: 作成者自身が `PASS` を付けたり、自己評価を依頼文に混ぜたり、別の条件で得た判定を流用したりする。
- **仕組み**: 判定者・計画者の起動時の prompt は、実行しない固定文字列 `devkit-instruction <指示 ID>` だけの1行とする。これはサブコマンドではない。起動の制限は [PreToolUse](hooks.md#pretooluse)、最終メッセージと来歴の採取は [SubagentStop](hooks.md#subagentstop)に従う。
- **置き場所**: `.claude/agents/devkit-<ロール>.md`・`.codex/agents/devkit-<ロール>.toml` の定義、`internal/check/` の受理、`internal/engine/` の `next`。

サブエージェントの定義には次の2点を記載する。

- prompt が上の定型以外を含む場合は `NOT ASSESSED` を返す。
- 指示 ID に含まれる作業 ID から `.devkit/run/work/<作業 ID>/brief/<指示 ID>.md` を解決し、依頼の本文を読む。

`next` は SubagentStop hook が書いた結果ファイルだけを読み、スキーマ、指示 ID、対象のバージョン、規範の識別を照合する。さらに、最初の利用者メッセージが上の定型と一致し、定義ファイルの内容ハッシュが[規範の固定](#規範の固定)で定めるスナップショット内の定義と一致し、観測した effort(Claude Code)または model(Codex)が定義の値と一致することを検証する。不一致は受理せず、エージェントが書いたファイルや標準出力も受理しない。

定義を編集したラウンドの完了手順は[サブエージェント定義の変更と完了](decisions.md#サブエージェント定義の変更と完了)に残す。

同じ指示 ID に複数の結果があれば、SubagentStop の観測時刻が最も早いものだけを受理し、他は参考として記録に残す。判定が食い違えば Finding に `needs-decision` として含める。判定コマンドの判定と `command:` の check は `devkit` が実行し、同じ受理境界で Judgment に変換する。`command:` の結果の変換と実行順は[作業結果の判定](flow.md#作業結果の判定)に従う。

適用除外の主張と根拠がある場合は、それによって対象から外れる要求に限って違反としない。判定は、`FAIL`、`NOT ASSESSED`、`PASS` の優先順で集約する。

ラウンドの判定依頼には、そのラウンドの[終了ブロック](data.md#進捗-progress)(作業者の報告)を含める。作成者の説明は [Judgment](data.md#check-の結果-judgment)(判定者の結果)の代わりにならず、その扱いは [`principles/harness.md` の原則1](../../principles/harness.md#1-作成者の説明に依存せず対象を合意条件と照合する)に従う。

観点「差分」の判定依頼には、有効な[承認](data.md#回答承認-answer--approval)・委任(判断・操作を任せた範囲)の一覧を含め、判定者が対象と範囲を照合できるようにする。委任の判断基準は [`principles/working.md` の原則6](../../principles/working.md#6-合意と委任の範囲内で実行上の判断を進め必要な承認を求める)・[原則7](../../principles/working.md#7-頼まれていない変更をしない)に従う。

### 検出試験

- **失敗**: 違反を検出できるか確かめていない観点を、必須 check にする。
- **仕組み**: 対象は判定者(サブエージェントと判定コマンド)が担当する観点ファイルだけとし、`command:` の check は検出試験を要しない。違反例では `FAIL`、適合例・許容例では `PASS` を期待する。[principles/harness.md](../../principles/harness.md) の原則2に従い、計画に指定された当該 check の実行主体と一致する[合格記録](data.md#検出試験の記録-detectiontest)がない観点は必須 check にしない。記録が不足すれば、`next` はラウンドを開く前に検出試験を指示し、合格を記録してから開始時のスナップショットを取る。不合格なら観点を修正するか必須から外すかの問いを起票し、停止する。
- **置き場所**: `internal/check/` の試験仕様と結果の判定、`internal/engine/` の `next`、`.devkit/detect.json` の合格記録の索引。

合格記録の再利用は[内容ハッシュによる同一性](data.md#検出試験の記録-detectiontest)に従い、表記修正でも識別に使うハッシュが変われば試験をやり直す。[principles/harness.md](../../principles/harness.md) の原則2の適用除外は、人が手で行う試験だけに適用する。

### 回答の受理

- **失敗**: 人の曖昧な答え、引用、説明、別の主体の報告を、都合よく承認と解釈する。
- **仕組み**: `devkit answer <問い ID>` は選択肢を引数に取らない。起票後に観測した入力のうち、全文から前後の空白を除いたものが「問い ID 選択肢」だけの1行と完全に一致する最新の入力だけを受理する。引用・説明を含む入力や長文は一致しない。ゲートと外部操作の承認に、この照合を必須とする。
- **置き場所**: [UserPromptSubmit](hooks.md#userpromptsubmit)が書く観測、`internal/engine/` の `answer`、[回答・承認](data.md#回答承認-answer--approval)の記録。

選択肢の回答が定型に一致しなければ実行せず、「問い ID 選択肢」だけの1行での再入力を求める。

受付期限内に新たな定型の入力が観測されれば、訂正としてそちらを優先する。訂正は、承認した外部操作の実行開始、またはゲート通過後の次のステージの開始まで受け付ける。それ以後の定型の入力は訂正として受理せず、新しい問いの起票を促す。

文章での回答(決定の保留の理由など)は、全文が問い ID で始まる入力のうち最新のものを取り、本文をそのまま記録に写す。同一性・訂正の競合・根拠の保存は[回答・承認](data.md#回答承認-answer--approval)に従う。

### commit 直前の対象の照合

- **失敗**: 判定後にファイルが変更され、判定・完了・コミットの対象が食い違う。
- **仕組み**: commit の直前に、作業ツリーから一時インデックスで `write-tree` を取り、返却後のスナップショットと一致することを確かめる。一致しなければ commit せず、変更後のツリーで返却後のスナップショットと成果物を更新して判定からやり直す。取り直しは1回だけとし、2回目も変われば問いを起票する。
- **置き場所**: `internal/engine/` の完了の確定と、`internal/workspace/` の一時インデックス。

check と判定前の pre-commit の実行前にスナップショットが一致しない場合の扱いは[検証前の対象の不一致](decisions.md#検証前の対象の不一致)に残す。

### 判定対象の展開と完了時の永続化

- **失敗**: git の属性で判定対象が変わったり、完了時に未判定の変更を取り込んだり、index と HEAD が食い違ったりする。
- **仕組み**: 判定者には返却後のスナップショットを一時インデックスから `git read-tree` と `git checkout-index --prefix` で `snap/<対象のバージョン>/` に展開し、対象はその読み取り専用の場所だけを読ませる。`.gitattributes` の `export-ignore`・`export-subst` の影響を受けない。
- **置き場所**: `internal/workspace/` の展開と永続化、`internal/engine/` の判定指示と完了の確定。

完了時の永続化は[完了条件](#完了条件)の `git commit` に従う。エージェントにはコミットさせない。

### 外部操作の承認

- **失敗**: 未承認の対象や、承認時と異なるコマンドで外部に書き込む。
- **仕組み**: 承認対象を操作 ID・入力のハッシュ・登録定義とリポジトリ内の実行ファイルの内容ハッシュで識別する。計画の承認で足りるのは、計画に宣言した種別・対象・入力のハッシュと、宣言時の登録定義・実行ファイルの内容ハッシュがすべて一致する操作だけであり、それ以外は操作ごとに問いを起票して「外部操作の確認待ち」で止める。
- **置き場所**: `internal/plan/` の宣言の照合、`internal/engine/` の `external` の受理、`internal/external/` の実行直前の照合。

承認への参照、実行前の記録、実行、結果の記録の順を守る。開始直前に有効な承認を読み直し、登録定義と実行ファイルの内容ハッシュが受理時と一致するか照合する。不一致なら新しい問いを起票する。入力の渡し方と対象の許可値は [external](modules.md#external)に従う。

### 外部操作の二重適用

- **失敗**: 再送や中断後の再実行により、同じ書き込みを二重に適用する。
- **仕組み**: 操作 ID は(種別、対象、入力のハッシュ)から導く。同じ入力で正当に繰り返す操作(コメントの追加など)は、登録に「発生単位」として要求ごとに変わる入力の項目を宣言し、導出に含める。要求 ID は同じ呼び出しの再送の識別だけに使う。
- **置き場所**: `internal/engine/` の `external` の要求受理、`internal/external/` の冪等キー・実行前後の記録・回復手続き。

同じ要求 ID に異なる内容が来たら拒否する。同じ操作 ID が実行中または結果不明なら、要求 ID が違っても新規には受理せず記録済みの状態を返す。成功済みなら記録済みの成功を返し、再実行しない。実行と照会には登録ごとの期限を設け、超えれば結果不明とする。外部側で識別・照会できるときだけ照会して確定し、できなければ再実行せず人に確認する。

### 誤った場所への書き込みと危険な操作

- **失敗**: チェックアウト外への書き込み、元に戻せない操作、external を迂回する外部書き込み、結果ファイルへの通常の書き込みが起きる。
- **仕組み**: 書き込み先と外部通信は[権限の境界](#権限の境界)で制限する。危険な操作と迂回は [PreToolUse](hooks.md#pretooluse)の `[deny]` で止める。結果ファイルへの通常の書き込みは[操作の禁止](#操作の禁止とサブエージェントの制限)の `permissions.deny` と、[PreToolUse](hooks.md#pretooluse)の「リダイレクト・置換を含むコマンドは分割せず deny」で止め、それ以外は信頼の前提の範囲とする。
- **置き場所**: ホストの sandbox と permissions、`devkit.toml` の `[deny]`、`internal/hook/` の PreToolUse、`internal/install/` の設定の追記。

### 再開と hook の稼働確認

- **失敗**: 再開時に別の作業・計画・指示で進めたり、hook が動いていないまま承認や完了へ進んだりする。
- **仕組み**: [StageRun の束縛](data.md#計画のバージョン)、[指示 ID](data.md#指示-instruction)、[セッションとブランチ](data.md#セッションとブランチ)、[checkpoint](data.md#checkpoint)で現在地を復元する。途中でセッションが終わっても作業は「作業中」のまま残り、次のセッションに引き継ぐ。継続は [Stop](hooks.md#stop)が補助し、唯一の保証とはしない。
- **置き場所**: `internal/engine/` の受理前の照合、セッションの観測、`internal/hook/` の SessionStart と Stop。

`devkit` は Bash のツール呼び出しなので、その直前に PreToolUse が発火する。`next`・`submit`・`answer`・`external`・`plan`・`note` は、同じセッションの PreToolUse の最終観測が呼び出し直前(数秒以内)にあることを照合する。なければ `next` は状態を進めず「hook の稼働を確認する」指示を返す。それ以外の書く操作は拒否し、`answer` を含め拒否理由に「hook が動いていない。`devkit doctor` と `/hooks` を確認する」を含める。ゲートの承認もこの照合を通らなければ受理しない。

観測がない状態での `start` の扱いは[観測のないセッションでの開始](decisions.md#観測のないセッションでの開始)に残す。

### 人への確認対象

- **失敗**: 依存やループ自身の変更を、確認なしに進める。
- **仕組み**: 「人に確認する」は kit の分類表の既定データとし、リポジトリ側の追記で上書きできる。既定には依存の宣言(ロックファイル・マニフェスト)の差分、宣言のないメタループの編集、[commit 直前の対象の照合](#commit-直前の対象の照合)で定める対象の再変化を含める。メタループの編集は `.devkit/`(`run/` を除く)・`devkit.toml`・ホストの設定・指示ファイル・skill・git hook(`core.hooksPath` の先の pre-commit)の変更である。`.devkit/` にはループの雛形を置く `.devkit/plans/` も含まれる。検証そのものを変える編集なので、メタループの編集を宣言したステージ以外では人が確認する。
- **置き場所**: `kit/router/` の既定、`devkit.toml` の追記、`internal/router/` の分類。hook の稼働を確認できない場合は[再開と hook の稼働確認](#再開と-hook-の稼働確認)で扱う。

「暫定判断で進めてレビューで確認する」場合は、問いを起票せず、暫定の決定を `devkit note` で記録して進み、次のゲートまたは PR レビューで人が確認する。router の分類の出力としては「修正する」または「完了する」で返す。

#### 教訓の反映先

router の「教訓の反映先」の優先順は、hook・lint・型 → [ループの雛形](data.md#ループの雛形-plantemplate) → 原則 → skill の手順 → kit に戻す、とする。

### 記録の破損と競合

- **失敗**: 書きかけの記録を読んだり、並行する操作が状態を食い違わせたりする。
- **仕組み**: [記録の書き込みと排他](data.md#記録の書き込みと排他)の公開方法と[作業の排他制御](data.md#作業の排他制御)を使う。共有資産の競合は[資産の更新と共有](data.md#資産の更新と共有)で扱う。
- **置き場所**: `internal/ledger/fsstore/` の公開・隔離・ロック、git のマージと人のレビュー。

### バージョンの不一致

- **失敗**: 別の kit や互換性のない形式を使い、作業やチーム内の前提が食い違う。
- **仕組み**: [kit](modules.md#kit)でコピーを照合し、[バージョン互換性](#バージョン互換性)の境界で拒否する。
- **置き場所**: `internal/kit/`、`devkit doctor`、記録の読み込み。

## 権限の境界

### ホスト設定と実行権限

ホスト設定の正本はリポジトリ内の `.claude/settings.json`・`.claude/agents/`・`.codex/hooks.json`・`.codex/config.toml`・`.codex/agents/`・`.codex/rules/` とする。`init` は kit の断片を追記し、リポジトリ全体に適用する項目を差分として示して人の確認を待つ。起動時に kit はホストへ設定を渡さない。hook の登録先はリポジトリ内を既定とし、ユーザー側の設定への登録も選択肢とする。

エージェントの書き込み先と外部通信はホストの sandbox が制限する。`devkit` 自身は git への書き込み(スナップショット、refs、コミット)と外部操作のために sandbox の外で動く。

pre-commit hook・判定コマンド・検証コマンドは `devkit` の子プロセスとして sandbox の外で動き、専用の隔離は行わない。pre-commit hook はリポジトリが所有して人がレビューする。判定コマンドは `devkit.toml` の `[judges]`、検証コマンドは `[commands]` に登録する育てるものであり、登録時に人がレビューする。

- Codex は `.codex/rules/devkit.rules` の `prefix_rule(pattern=["devkit"], decision="allow")` を使う。sandbox の外で承認なしに動かす機能であり、experimental であることは確認済みである。
- Claude Code は `sandbox.excludedCommands` の `devkit *` と `permissions.allow` の `Bash(devkit *)` を併用する。前者だけでは権限の問い合わせが残る。この挙動と、プロジェクト設定の `sandbox` が効くことは確認済みである。`init` は利便性のためこの allow を提案し、記録上の保証には使わない。

`devkit` は `cd`・リダイレクト・連結・置換を付けず、単独のコマンドとして呼ぶ。付けた場合は sandbox の中で動く。skill にこの呼び方を書き、起動時には `.git` に一時 ref を作成・削除する書き込み試験を行う。sandbox の中なら記録を書く前に失敗し、理由を返す。

### 操作の禁止とサブエージェントの制限

リポジトリ全体に適用する操作の deny は、人がリポジトリの方針として `.claude/settings.json` に書く。kit が `permissions.deny` に追加するのは `Edit(.devkit/run/**/records/**)`・`Edit(.devkit/run/**/results/**)`・`Edit(.devkit/run/**/sessions/**)`・`Edit(.devkit/run/**/prompts/**)`・`Edit(.devkit/run/**/snap/**)` に限る。kit の `[deny]` とサブエージェントの起動の制限は [PreToolUse](hooks.md#pretooluse)で適用し、settings には足さない。Claude Code の hook は `onFailure: "block"` 相当の設定で登録する。

Claude Code の判定者・計画者は、定義の `tools` を読み取り用に限定する。この制限と `Agent(fork)`・`Agent(model:*)` の deny が効くことは確認済みである。Codex は親の sandbox を継承するため、カスタム定義に `features.shell_tool = false` を指定する。この設定を書けることは確認済みである。読み取りの可否と代替は[未確認の前提](decisions.md#未確認の前提)に残す。

起動・操作を要する check(動作検証)の runner は判定コマンドまたは `command:<name>` に限り、読み取り専用の判定者サブエージェントには割り当てない。

## 性能と出力の上限

運用上の制約と対策は [principles/designing.md](../../principles/designing.md) の原則6に従う。

作業ごとの記録数は少ないため、独立したファイルを一覧して読み込む方式で足りる。`next` は毎回 ledger を全走査する。対象のバージョンの計算は、追跡ファイルと ignore されていない未追跡ファイルを走査してコミットオブジェクトを作れる規模のリポジトリを前提とする。判定対象の展開は対象のバージョンごとに1回行い、作業の完了・中止時に削除する。

`command:` の check、判定前の pre-commit、完了時の commit を実行する `next` の呼び出しでは、利用中のホストの Bash ツールの仕様で定める timeout の最大値を指定する。

CLI の標準出力は、Claude Code の Bash の上限(正常時は約3万字、失敗時は約1万字)に収まるなら全文、超えるなら要点とファイルのパスを出す。hook の出力上限は Claude Code が約1万字、Codex が約2,500トークンと確認済みである。checkpoint は上限内なら本文を注入し、超えるなら要約と「`devkit status` で全文」の案内を注入する。

## 観測性

記録そのものを観測の基盤とし、`devkit status` と view で状態を確認する。プロセスのログは標準エラー出力に出す。記録の網羅性は[成果物を記録した範囲](modules.md#view)として示す。

<a id="ロールアウト"></a>

## バージョン互換性

バイナリ、固定物、記録形式はそれぞれバージョンを持つ。Work と kit のコピーの一致は [kit](modules.md#kit)で照合する。未知のバージョンや互換性のない組み合わせは、失敗を隠さずエラーにする。読めない記録は失敗として報告し、記録形式の移行の仕組みは設けない。読めなくなった作業は古いバイナリで終えるか `abort` する。
