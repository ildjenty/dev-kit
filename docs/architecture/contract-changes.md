# 契約の改訂と試験

部品の契約は、セッション内の CLI が状態機械を進め、hook の観測を受理する構成に合わせて改訂する。受理条件は [data.md](data.md) と [guarantees.md](guarantees.md) に従う。試験ではモジュールごとの境界と組み合わせを確認する。

## 契約の改訂

### design

- [design.md](../design.md) 冒頭の「ループを構成する部品」を「ループに組み込む要素」に改め、「部品」は [modules の提供単位](modules.md#モジュールごとの契約)である7つに限る改訂は、本文に反映済みである。
- [design.md「部品」](../design.md#部品)の「ループ計画が選ぶのは…」を「雛形が定め、計画は取捨する」に改め、末尾の範囲を各部品の実装モジュール・kit の固定物・配置・依存へ広げて [architecture.md](../architecture.md) と architecture/ を設計の正本とする改訂は、本文に反映済みである。

### runtime

- [runtime.md](../parts/runtime.md) 冒頭と「workflow」を [architecture.md「全体構成」](../architecture.md#全体構成)と [engine の操作](modules.md#engine)へ合わせてセッション内の CLI による進行に改め、担当の起動・結果の受け取りと人の入力・訂正は[判定結果の受理](guarantees.md#判定結果の受理)・[回答の受理](guarantees.md#回答の受理)・[UserPromptSubmit](hooks.md#userpromptsubmit)への参照にする。
- [runtime.md「ファイルの分類と既定の置き場所」](../parts/runtime.md#ファイルの分類と既定の置き場所)の一覧を[所有と生存期間](data.md#所有と生存期間)への参照に置き換え、固定物の外部参照と実行の記録の git 管理を選べる記述を削り、配置は[利用側のリポジトリ](layout.md#利用側のリポジトリ)に固定する。
- [runtime.md「hook」「kit が使う hook」](../parts/runtime.md)の一覧を [hooks.md](hooks.md) の5イベントに置き換え、PostToolUse の利用と「ホスト固有のイベント」の Codex での代替を削り、hook の書き込み先は[生成物だけ](data.md#記録の書き込みと排他)に改める。
- [runtime.md「hook」の共通処理](../parts/runtime.md)の `stop_hook_active` によるループ防止を [Stop の観測内の継続回数と上限](hooks.md#stop)へ改め、ルートの解決は [cmd/devkit](modules.md#cmddevkit)、出力の整形は [host/event](modules.md#hostevent)の責務にする。
- [runtime.md「hook」「終了条件」](../parts/runtime.md)の Stop による check を[停止の補助](hooks.md#stop)と [submit の終了ブロック](data.md#進捗-progress)・[完了条件](guarantees.md#完了条件)へ改め、「Read では大きな文書の全文読み込みを拒否する」を削り、skill 編集の検知は PostToolUse ではなく[人への確認対象](guarantees.md#人への確認対象)への参照にする。
- [runtime.md「ホストの能力の検出試験」](../parts/runtime.md#ホストの差分と共通の仕組み)の試験と代替経路を [doctor](modules.md#doctor-の確認項目)と[セッションの観測](data.md#セッションの観測-sessionobservation)へ置き換え、hook の登録先と sandbox の外での実行は[権限の境界](guarantees.md#権限の境界)に合わせる。
- [runtime.md「隔離」「作業の排他」「委任の区別と依存」](../parts/runtime.md)を[ホストに任せる範囲](guarantees.md#実装対象外の機能)・[作業の排他制御](data.md#作業の排他制御)・[定型の依頼](guarantees.md#判定結果の受理)への参照に置き換え、「Codex の隔離は別プロセスで行う」を削る。
- [runtime.md「再開」](../parts/runtime.md)の束縛とブランチの扱いを[セッションとブランチ](data.md#セッションとブランチ)、hook の直前の観測を[稼働確認](guarantees.md#再開と-hook-の稼働確認)、commit 直前の照合を[対象の照合](guarantees.md#commit-直前の対象の照合)に合わせる。
- [runtime.md「終了条件」](../parts/runtime.md)の検証コマンドの登録を [config](modules.md#config)、ステージの check の実行を [engine](modules.md#engine)へ改め、pre-commit の実行時点と commit の扱いは[完了条件](guarantees.md#完了条件)への参照にする。
- [runtime.md「ループ計画」](../parts/runtime.md)を[雛形](data.md#ループの雛形-plantemplate)と[計画](data.md#計画-plan)の2層へ改め、「分類と計数だけを評価する」は [router の入力と判断](modules.md#router)への参照に置き換える。
- [runtime.md「設定」](../parts/runtime.md)の「すべての置き場所の正本」と git 管理の可否を削り、[固定配置](layout.md#利用側のリポジトリ)と [config の設定項目](modules.md#config)への参照に置き換える。
- [runtime.md「インストーラ」](../parts/runtime.md)の「バイナリを置く」を削り、[install の init](modules.md#install)による導入と[バイナリ配布の未決定](decisions.md#仮決定)を分ける。
- [runtime.md「ステージの実行記録」「記録範囲の照合」](../parts/runtime.md)の Stop による ledger の書き込みを[セッションの観測](hooks.md#stop)へ、実行状態の記録を [CLI による書き込み](data.md#記録の書き込みと排他)へ、網羅性の照合を [view の成果物による網羅性](modules.md#view)へ改める。
- [runtime.md「再開」](../parts/runtime.md)の外部書き込みの再実行を[外部操作の二重適用](guarantees.md#外部操作の二重適用)への参照に置き換え、成功済みは記録を返し、実行中・結果不明は照会か人への確認に進み、盲目的に再実行しない扱いに改める。
- [runtime.md「前のステージに戻る」「計画の作り直し」](../parts/runtime.md)を[計画のバージョン](data.md#計画のバージョン)と [flow の再計画](flow.md#router-の判断)へ合わせ、計画者が作り直し、check の合格と必要な承認を経て切り替え、旧 StageRun は再計画待ちとする。
- [runtime.md「harness の改善」「メタループの編集対象」](../parts/runtime.md)の「途中で修正しない」と編集対象の列挙を[人への確認対象](guarantees.md#人への確認対象)と雛形の宣言への参照に置き換え、検出試験の対象は[観点ファイルだけ](guarantees.md#検出試験)に改める。
- [runtime.md](../parts/runtime.md) の組み立ての試験の「設定ファイルから引いた置き場所」と「保存した checkpoint」を[固定配置](layout.md#利用側のリポジトリ)と [ledger から組み立てる checkpoint](data.md#checkpoint)へ改め、別セッション・compaction 後の注入を[結合試験](#結合試験)に合わせて確認する。
- [runtime.md「hook の出力上限」](../parts/runtime.md#ホストの差分と共通の仕組み)の一律にファイル参照とする注入を[性能と出力の上限](guarantees.md#性能と出力の上限)への参照に置き換え、上限内は本文、超えるときは要約と `devkit status` の案内に改める。
- [runtime.md「決定候補」](../parts/runtime.md#決定候補)を削除して決定の所在を [decisions.md の仮決定](decisions.md#仮決定)と[不採用の案](decisions.md#不採用の案)への参照に移し、担当を子プロセスで起動する推奨は不採用の案への参照に置き換える。
- [runtime.md「hook の形式」「hook の共通化」](../parts/runtime.md#ホストの差分と共通の仕組み)の入出力・終了コードが両ホストで共通という記述を [host/event による変換](modules.md#hostevent)へ改め、「担当の起動の共通化」のアダプタは[定型の依頼と結果の受理](guarantees.md#判定結果の受理)への参照に置き換える。

### repository

- [repository.md「CLAUDE.md」「指示ファイル」](../repository.md)の開始・終了手順と指示ファイルの正本の選択を削り、[指示ファイルとホスト設定](layout.md#指示ファイルとホスト設定)に合わせて kit は指示ファイルに触れず、終了手順は[作業結果の提出](flow.md#作業結果の提出)への参照にする。
- [repository.md「skill」](../repository.md#実行の仕組みとの対応)を[kit の skill](layout.md#指示ファイルとホスト設定)による開始・`next`・`submit`・問いの見せ方・サブエージェントの起動と振り返り・教訓の反映先の選択に改め、計画の作成は [flow の計画者](flow.md#作業の開始)へ移す。
- [repository.md「サブエージェント・Codex」](../repository.md#実行の仕組みとの対応)の作業の委任を削り、[全体構成](../architecture.md#全体構成)と [Plan の担当](data.md#計画-plan)に合わせて作業はセッション自身、判定者・計画者はサブエージェント、別系統は判定コマンド、担当は雛形で定める形に改める。
- [repository.md「元に戻せないものの一覧と、外部への書き込みの一覧」](../repository.md#リポジトリ側の構成)の所在を、前者は [Rules](data.md#規則と強制手段-rules)の `.devkit/rules.md` と計画、後者は [config](modules.md#config)の登録と [Plan の宣言](data.md#計画-plan)に改める。
- [repository.md「リポジトリが育てるもの」](../repository.md#リポジトリ側の構成)の一覧を[所有と生存期間](data.md#所有と生存期間)への参照に置き換え、雛形・`.devkit/detect.json`・kit のコピーを含め、playbook と CLAUDE.md の索引を外し、ホスト設定・サブエージェント定義・rules も同じ資産として扱う。
- [repository.md「設定ファイル」「記録ディレクトリ」](../repository.md#リポジトリ側の構成)を [config](modules.md#config)と[利用側のリポジトリ](layout.md#利用側のリポジトリ)への参照に置き換え、置き場所を設定から引く記述と git 管理の可否を削り、`devkit.toml` は kit のバージョン・判定コマンド・検証コマンド・外部操作・deny・分類表の追記を扱う説明に改める。
- [repository.md「kit の固定物」](../repository.md#リポジトリ側の構成)の外部参照の選択肢を削り、[kit の供給](modules.md#kit)に合わせて `.devkit/kit/<バージョン>/` のコピーに限る。
- [repository.md「設定ファイル」](../repository.md#リポジトリ側の構成)の「インストーラがバイナリを置く」を削り、[init の役割](modules.md#install)と[バイナリ配布の未決定](decisions.md#仮決定)を分け、hook の登録先は[権限の境界](guarantees.md#権限の境界)への参照にする。
- [repository.md「runtime」と人の判断の段落](../repository.md#実行の仕組みとの対応)を [architecture.md「全体構成」](../architecture.md#全体構成)へ合わせ、外側の workflow をセッション内の CLI による進行と hook の観測に改め、計画の作り直しは[計画のバージョン](data.md#計画のバージョン)への参照にする。

### ledger

- [ledger.md「checkpoint」「カーソル」](../parts/ledger.md)の上書き記録と閲覧・確認済みの集合を [checkpoint](data.md#checkpoint)と[状態変更](data.md#決定と証拠-decision--evidence)の派生値へ改める。
- [ledger.md](../parts/ledger.md) 末尾の保存方法を[記録の書き込みと排他](data.md#記録の書き込みと排他)への参照に置き換え、「改行・タブ・数式として解釈される文字は除く」を削り、「ステージの実行」の実行状態は [StageRun](data.md#ステージの実行-stagerun)に合わせて「異常終了・不明」を削る。
- [ledger.md](../parts/ledger.md) 冒頭の保存時刻と利用時刻の照合を削って途中の記録を [note](data.md#決定と証拠-decision--evidence)へ合わせ、「決定」の調査の記録順は [skill の手順](#skill-の手順)へ移す。
- [ledger.md「check の結果」](../parts/ledger.md#check-の結果)の検出試験の対象と正本・実行の事実を [DetectionTest](data.md#検出試験の記録-detectiontest)へ、判定の失効を [Judgment](data.md#check-の結果-judgment)へ合わせ、「記録範囲の照合」は削って網羅性を [view の成果物](modules.md#view)から導く。
- [ledger.md](../parts/ledger.md) の指示と出し直しを [Instruction](data.md#指示-instruction)、発行した指示の集合を [Round](data.md#ラウンド-round)、結果ファイル名を [SubagentStop](hooks.md#subagentstop)へ合わせ、回答の本文と出所は [Answer / Approval](data.md#回答承認-answer--approval)に従って記録する形に改める。
- [ledger.md](../parts/ledger.md) 冒頭の全記録に課題・計画・実行・対象への参照を要求する記述を[記録の範囲](data.md#記録の範囲)への参照に置き換え、共通項目と種別ごとの参照を分ける。
- [ledger.md「ステージの実行」](../parts/ledger.md#ステージの実行)と Stop hook の記録責務を[記録の書き込みと排他](data.md#記録の書き込みと排他)へ合わせ、Stop hook はセッションの観測だけを生成し、外部書き込みの冪等キーと結果は StageRun から [ExternalOp](data.md#外部への書き込み-externalop)へ移す。
- [ledger.md「構造」「規則と強制手段」](../parts/ledger.md)を削り、[Structure](data.md#構造の記録-structure)の `.devkit/structure.json` と [Rules](data.md#規則と強制手段-rules)の `.devkit/rules.md` への参照に置き換える。
- [ledger.md](../parts/ledger.md) の記録種別に [ledger のペイロード](modules.md#ledger)に従って作業・計画の撤回・証拠・状態変更・問い・回答・承認・外部への書き込み・進捗を加える。
- [ledger.md「成果物」](../parts/ledger.md#成果物)を [Artifact record](data.md#成果物-artifact-record)へ合わせ、`devkit` がスナップショットの差分から生成し、永続化したコミットへの参照を持つ記録に改める。
- [ledger.md「計画」「ステージの実行」](../parts/ledger.md)を[計画のバージョン](data.md#計画のバージョン)と [StageRun](data.md#ステージの実行-stagerun)へ合わせ、計画の提案・撤回と承認を別記録として実行可能性を派生させ、完了記録には commit のハッシュを持たせる。

### router

- [router.md「check の結果」](../parts/router.md#check-の結果)の完了条件と Stop と共有する記述を[完了条件](guarantees.md#完了条件)への参照に置き換え、`command:` の結果を [Judgment](data.md#check-の結果-judgment)として扱い、pre-commit を伴う commit の成否と合わせた完了の判断に改める。
- [router.md「人への確認」](../parts/router.md#人への確認)を[人への確認対象](guarantees.md#人への確認対象)と [hook の稼働不足](guarantees.md#再開と-hook-の稼働確認)に合わせ、既定と利用側の追記による分類へ改める。
- [router.md「ロールとエンジン」](../parts/router.md#ロールとエンジン)のロールを [Plan の担当](data.md#計画-plan)に合わせて作業者・人・判定者・計画者に揃え、分類表の出力はサブエージェントまたは判定コマンドの名前へ改める。
- [router.md「教訓の反映先」](../parts/router.md#教訓の反映先)の分類順を[人への確認対象で定める順序](guarantees.md#人への確認対象)への参照に置き換え、移行記録の結び付け先を ledger から [Rules](data.md#規則と強制手段-rules)の `.devkit/rules.md` の項目へ改める。
- [router.md「check の結果」](../parts/router.md#check-の結果)の分類の入力を [router の契約](modules.md#router)へ合わせ、集約した判定・ラウンド数・進展の有無・計画の上限・停止分類に改める。

### check

- [check.md「入力」](../parts/check.md)を [TargetVersion](data.md#対象のバージョン-targetversion)・[判定対象の展開](guarantees.md#判定対象の展開と完了時の永続化)・[check の公開契約](modules.md#check)へ合わせ、「commit または content hash」をツリーハッシュ・計画のバージョン・構造のバージョンに改め、読み取り専用の展開先・固定した規範と観点・出力スキーマ・前回の指摘 ID を渡す形にする。
- [check.md「出力」「指摘」](../parts/check.md)を [form の判定者の結果](modules.md#form)と [Finding](data.md#指摘-finding)へ合わせ、「1行目に PASS/FAIL/NOT ASSESSED」を削って最終メッセージの JSON に改める。
- [check.md「実行主体」と作成・判定の分離の規則](../parts/check.md)を [Plan の担当と runner](data.md#計画-plan)と[判定結果の受理](guarantees.md#判定結果の受理)へ合わせ、コンテキスト ID と履歴継承設定の照合および「会話の内容を知らないサブエージェント」という記述を置き換える。
- [check.md「実行主体」](../parts/check.md)の「Codex ではモデル名を明示する」をリポジトリ内の定義と[起動時の上書きの制限](hooks.md#pretooluse)への参照に改め、`command:` の実行結果から Judgment への変換は[作業結果の判定](flow.md#作業結果の判定)と[受理境界](guarantees.md#判定結果の受理)へ、修正ラウンドの依頼は [Finding](data.md#指摘-finding)へ合わせる。
- [check.md](../parts/check.md) の transcript・定義・実効条件の照合、複数結果の選択と食い違いの扱いは[判定結果の受理](guarantees.md#判定結果の受理)への参照にする。
- [check.md](../parts/check.md) の規範の一覧の CLAUDE.md を「指示ファイル(AGENTS.md と CLAUDE.md)」、「ループ計画」を「計画」へ改め、固定方法と規範の束ね方は[規範の固定](guarantees.md#規範の固定)と [check](modules.md#check)への参照にする。
- [check.md](../parts/check.md) の検出試験の規則を[検出試験](guarantees.md#検出試験)と [DetectionTest の同一性](data.md#検出試験の記録-detectiontest)への参照に置き換え、対象を観点ファイルに限り、表記修正で再試験を免除する記述を削って、原則2の適用除外を人が手で行う試験だけに適用する形に改める。
- [check.md](../parts/check.md) 末尾の集約と必須 check が `NOT ASSESSED` のときの遷移を分け、集約は[判定結果の受理](guarantees.md#判定結果の受理)、遷移は [router の完了条件](guarantees.md#完了条件)と [plan の遷移表](modules.md#plan)への参照に改める。
- [check.md「実行主体」](../parts/check.md)の起動・操作を要する観点の runner を[操作の禁止とサブエージェントの制限](guarantees.md#操作の禁止とサブエージェントの制限)への参照に置き換え、読み取り専用の判定者では実行しない形に改める。

### form

- [form.md「終了ブロック」](../parts/form.md)を [submit の入力](data.md#進捗-progress)へ、判定者の最終メッセージ・指示・問い・決定と証拠のスキーマを [form](modules.md#form)へ合わせ、playbook は[skill の配置](layout.md#指示ファイルとホスト設定)に従って skill に統合する。
- [form.md「人への確認事項」](../parts/form.md)の回答案内を [Question](data.md#問い-question)、選択肢と文章の回答・訂正期限を[回答の受理](guarantees.md#回答の受理)、表示上の範囲と入力 JSON の場所を[状態遷移と受理条件](data.md#状態遷移と受理条件)へ合わせる。
- [form.md「コマンドの使い方」](../parts/form.md)の「CLAUDE.md の索引から」を[指示ファイルとホスト設定](layout.md#指示ファイルとホスト設定)に合わせて「devkit のコマンドは kit の skill に置く」へ改める。
- [form.md「PR 本文」](../parts/form.md)に[資産の更新と共有](data.md#資産の更新と共有)に従って計画の要約と共有に必要な決定を加える。
- [form.md](../parts/form.md) の計画・判定者の結果・指示・問い・決定と証拠・終了ブロックのスキーマの正本を [form の依存と kit の固定物](modules.md#form)へ合わせ、form が供給された定義を使う形に改める。

### view

- [view.md](../parts/view.md) の待ち・失効・最後の観測時刻・網羅性と構造図・git の履歴の入力元を [view](modules.md#view)へ合わせる。
- [view.md](../parts/view.md) の「人が状態を変更できるようにする」を[状態変更](data.md#決定と証拠-decision--evidence)と[人の回答と再開](flow.md#人の回答と再開)への参照に置き換え、HTML からは変更せずチャット経由の `answer` で記録する形に改め、問いの表示と停止は[状態遷移と受理条件](data.md#状態遷移と受理条件)へ合わせる。
- [view.md](../parts/view.md) の「確認状況の要点」を [Judgment の有効性](data.md#check-の結果-judgment)へ合わせ、対象のバージョンと規範の識別の両方の一致を示す形に改める。
- [view.md](../parts/view.md) のカーソルと一括確認の操作を削り、閲覧と確認済みの集合を[状態変更](data.md#決定と証拠-decision--evidence)から導く形に改める。
- [view.md](../parts/view.md) の構造図の Figma による手作業の調整を削り、[view の構造図の生成](modules.md#view)への参照に置き換える。

### loop-plan

- [loop-plan.md](../loop-plan.md) 冒頭とフォーマットを[雛形](data.md#ループの雛形-plantemplate)(利用者が組む)と[計画](data.md#計画-plan)(課題ごと)の2層・JSON(英語キー)へ改め、「ループ計画は brief の一種」を削って計画は plan の型、計画者への依頼文は [brief](modules.md#brief)の一種とする。
- [loop-plan.md](../loop-plan.md) 冒頭と末尾の「固定の手順は計画を作る手順だけ」を[作業の開始](flow.md#作業の開始)と[作業の指示から router の判断まで](flow.md#作業の指示)へ合わせ、kit が雛形から計画を作る手順と1ラウンドの進め方を固定し、ステージ・遷移・ゲート・上限は利用者が雛形として組む説明に改める。
- [loop-plan.md](../loop-plan.md) の「runtime は分類と計数だけを評価する」を [router の入力](modules.md#router)と[完了条件](guarantees.md#完了条件)への参照に置き換え、必須 check と `command:`・pre-commit の関係を同じ完了条件に合わせる。
- [loop-plan.md](../loop-plan.md) の「計画は記録として扱い、既定は git 管理の外」と「ステージの並び、担当、観点は使い捨て」を[所有と生存期間](data.md#所有と生存期間)・[固定配置](layout.md#利用側のリポジトリ)・[資産の更新と共有](data.md#資産の更新と共有)への参照に置き換え、雛形は git 管理で育て、計画は `.devkit/run/` の ignore 固定の記録とし、残す価値がある決定は PR 本文へ写す形に改める。
- [loop-plan.md「作成ルール」](../loop-plan.md#作成ルール)を [PlanTemplate](data.md#ループの雛形-plantemplate)と[作業の開始](flow.md#作業の開始)に合わせて利用者の「雛形を組む規則」と計画者の「計画を作る規則」に分け、前者は開始ステージ・ステージ・check・ゲート・上限・遷移表・候補ステージ、後者は完了条件・関係のないステージの除外・check の対象・候補からの追加を扱い、計画への入力に雛形を加える。
- [loop-plan.md「計画の check と人によるレビュー」](../loop-plan.md#計画の-check-と人によるレビュー)の承認条件を [Plan の雛形からの逸脱](data.md#計画-plan)への参照に置き換え、計画の check に逸脱の確認を含め、候補にないステージの追加・ゲートの削除・遷移の変更・必須 check の削除があるときに人の承認を要する形に改める。
- [loop-plan.md「フォーマット」「例」](../loop-plan.md)の担当と実行主体を [Plan](data.md#計画-plan)へ合わせ、作業者は担当名を持たないセッション自身でモデルを割り当てず、例は [examples.md の雛形と計画の JSON](examples.md#雛形と計画)への参照に置き換える。
- [loop-plan.md](../loop-plan.md) に計画の承認で足りる外部操作の宣言を加え、[外部操作の承認](guarantees.md#外部操作の承認)の識別項目へ合わせる。

### stages

- [stages.md](../stages.md) 冒頭の「ループ計画が選ぶステージ」を [PlanTemplate](data.md#ループの雛形-plantemplate)と[計画者の手順](flow.md#作業の開始)へ合わせ、「雛形が並べるステージを計画は外し、調査・試作は候補として計画者が足せる」へ改める。
- [stages.md「初期化」](../stages.md#初期化)の「harness を導入し」を削り、導入と確認は [install の init・doctor](modules.md#install)への参照に置き換える。
- [stages.md「2 構造」](../stages.md#2-構造)の成果物を [Structure](data.md#構造の記録-structure)と [view](modules.md#view)に合わせて「`.devkit/structure.json` の新しいバージョン(図は view が生成する)…」へ改める。

### checks

- [checks.md](../checks.md) の各観点の「ステージ」欄を観点ファイルの項目から外して [PlanTemplate の check](data.md#ループの雛形-plantemplate)へ移し、stages.md にない文書・設計・harness・メタの名前は [kit の既定の雛形](layout.md#dev-kit-のリポジトリ)の名前として示す形に改める。
- [checks.md「終了時の状態」](../checks.md#終了時の状態)の「(Stop)」と runtime の終了条件への参照を外し、[submit の終了ブロック](data.md#進捗-progress)と[完了条件](guarantees.md#完了条件)への参照に改める。
- [checks.md「harness」](../checks.md#harness)の playbook と保存済み checkpoint の記述を外し、[skill の配置](layout.md#指示ファイルとホスト設定)と[試験項目](#試験項目)への参照に改める。
- [checks.md「計画」](../checks.md#計画)の確認事項に [Plan の雛形からの逸脱](data.md#計画-plan)の有無を加え、逸脱があれば人の承認を要する指摘とする形に改める。
- [checks.md「差分」](../checks.md#差分)の「有効な承認や委任があるか」を[判定結果の受理](guarantees.md#判定結果の受理)に合わせ、判定依頼に含めた有効な承認・委任の一覧との照合として記述する。

### rules

- [rules.md「コードの構造」](../rules.md#コードの構造)の並列作業に使う異なる worktree の参照先を runtime の「隔離」から [layout.md「利用側のリポジトリ」](layout.md#利用側のリポジトリ)へ改める。
- [rules.md「文章と記録」](../rules.md#文章と記録)の番号の回答を名前で言い直して実行する規則を[回答の受理](guarantees.md#回答の受理)への参照に置き換え、定型に一致しない回答は実行せず「問い ID 選択肢」だけの1行での再入力を求める形に改める。

### meta-loop

- [meta-loop.md](../meta-loop.md) の入口と教訓の反映先に[人への確認対象の分類順](guarantees.md#人への確認対象)を参照してループの雛形(`.devkit/plans/`)を加える。
- [meta-loop.md「振り返り」](../meta-loop.md#メタループ)の課題を分ける規則を[人への確認対象](guarantees.md#人への確認対象)への参照に置き換え、メタループの編集は雛形で宣言したステージで行い、宣言のないステージでは人に確認する形に改める。
- [meta-loop.md「設計の2〜3割に生じる問題」](../meta-loop.md#設計の23割に生じる問題)末尾の採用しなかった設計の5分類の記録先を[決定と証拠](data.md#決定と証拠-decision--evidence)へ合わせ、決定のスキーマの「却下した案とその分類」に記録する形に改める。

### brief

- [brief.md](../parts/brief.md) の入力と引き継ぎを [brief](modules.md#brief)と[作業の指示](flow.md#作業の指示)へ合わせ、ステージ定義・前のステージの出力・リポジトリの事実(配置ガイド、rules.md の元に戻せないもの、利用可能な check)から組み立て、修正ラウンドには前回の指摘と指摘 ID を含める形に改める。
- [brief.md](../parts/brief.md) の用途とバージョンを [brief](modules.md#brief)と[定型の依頼](guarantees.md#判定結果の受理)へ合わせ、判定者・計画者への依頼文も brief とし、起動は定型の1行、本文は `brief/<指示 ID>.md`、バージョンは内容ハッシュに改める。
- [brief.md「停止条件」](../parts/brief.md)を [plan の上限](modules.md#plan)の表示だけに改め、停止の判断は [engine](modules.md#engine)と [router](modules.md#router)への参照に置き換える。
- [brief.md「返却内容」](../parts/brief.md)を[進捗](data.md#進捗-progress)の項目へ置き換え、決定は [note](data.md#決定と証拠-decision--evidence)、人の判断が必要な事項は終了の種別で表し、「参照しなかった入力」は進捗に含める形に改める。
- [brief.md](../parts/brief.md) の「ループ計画は brief の一種」と「ステージ N の出力がステージ N+1 の brief になる」を削り、[plan の型](modules.md#plan)と [brief の組み立て](modules.md#brief)への参照に置き換える。

### skill の手順

- kit の skill の作業の進め方は[処理の流れ](flow.md)、人への確認は[人の回答と再開](flow.md#人の回答と再開)、判定者・計画者の起動は[定型の依頼](guarantees.md#判定結果の受理)に合わせる。
- kit の skill のコマンドの呼び方は[権限の境界](guarantees.md#権限の境界)、コミットの扱いは[完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)への参照にする。
- [ledger.md「決定」の調査の記録順](../parts/ledger.md#決定)を kit の skill の手順に移し、再現手順の保存先とバージョンは修正開始前、仮説の結果は次の仮説を始める前に [devkit note](data.md#決定と証拠-decision--evidence)で記録する形に改める。

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
