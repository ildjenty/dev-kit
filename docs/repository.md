# 利用側リポジトリの構成

対象は、個人開発と、複数人が同じリポジトリで作業するチーム開発の両方である。

## リポジトリ側の構成

`init` が配布物の文章の規範をリポジトリの所有する場所へコピーし、作業ごとに育てる。資産の既定の配置は `devkit.toml` の `[paths]` で変更できる。

規範のパス同士の重なりと、成果のパス(requirements・structure・detect)が規範のパスと同じか配下にある設定は拒否する。

- `principles = ".devkit/principles"`: 原則。
- `stages = ".devkit/stages"`: ステージ定義。
- `checks = ".devkit/checks"`: 観点ファイルと `<観点ID>/` の検出試験の例。
- `plans = ".devkit/plans"`: `default`・`fix`・`docs` を起点に育てる雛形。
- `templates = ".devkit/templates"`: brief と form のテンプレート。
- `rules = ".devkit/rules.md"`: 複数の作業で守る決定と強制手段。
- `requirements = ".devkit/requirements.md"`: リポジトリが満たすべき要件の正本。
- `structure = ".devkit/structure.json"`: 構造の記録と配置ガイド。
- `detect = ".devkit/detect.json"`: 検出試験の合格の索引。

各リポジトリは資産と実行の記録を分けて管理する。

- **指示ファイル(AGENTS.md と CLAUDE.md)**: 管理・更新方針と判定の規範としての扱いは[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)に従う。
- **配置ガイド**: 構造の記録に、置くもの・置かないもの・配置先の案内を残す。構造のステージが更新し、brief には関係する構造の記録を入れる。
- **元に戻せないものと外部操作**: 元に戻せないものの共有項目は規則、課題固有の項目は[計画](architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)で扱う。外部操作(外部への書き込みの行為)の登録は [config](architecture/modules.md#config)、宣言と承認は[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従う。
- **育てるもの**: 原則・ステージ・観点と例・雛形・テンプレート・規則をリポジトリの規範として育て、git で共有する。kit の埋め込みとの一致は要求しない。
- **要件**: 目的のステージが、人が承認した完了条件と既存の要件を照らして更新する。目的がない雛形や目的を外した計画では、振り返りが要件に当たる完了条件を反映する。目的で反映済みなら重ねて更新する必要はない。後の作業の計画者・作業者・判定者の brief に必ず入れる。
- **規則と強制手段**: 複数の作業で守る変更困難な決定を残す。更新はループ自身の変更として人が確認する。「手段: 未実装」と振り返りで繰り返し見つかる違反は、機械の検査へ移す候補にする。
- **設定ファイル**: `devkit.toml` の項目は [config](architecture/modules.md#config)、配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。[判定コマンド](architecture/modules.md#config)(`[judges]`)は別エンジンによる判定に使う。[検証コマンド](architecture/modules.md#config)(`[commands]`)はテスト・型検査などに使い、`command:<name>` の [check](parts/check.md)(評価と判定の部品)から呼ぶ。実行の扱いは[作業結果の判定](architecture/flow.md#作業結果の判定)に従う。
- **実行の記録と生成物**: `.devkit/run/` に固定し、git 管理しない。サブエージェント定義・hook・保護の設定・ignore がこの場所を前提にし、途中で変えると記録を見失うためである。後の作業に効かせたい決定は要件・構造・規則へ残す。記録から組み立てる [checkpoint](architecture/data.md#checkpoint)(再開用の派生状態)は保存する記録と区別する。
- **単位の作業場所**: `.devkit/worktrees/<単位の指示 ID>/` に detached の worktree を作り、ignore する。記録の保護と重ならないよう `.devkit/run/` と分ける。中断中は保持し、中止・作業完了時は変更のない worktree だけを削除する。変更の残るものは保持して場所を知らせ、破棄は人が git で行う。
- **バイナリが持つもの**: 記録とバイナリの約束である JSON Schema、分類表とロールの既定、ホスト設定の断片を持つ。人への確認の分類は `devkit.toml` の `[router] human_confirmation` で上書き・追加・削除できる。check の結果から次の行動への対応とロールは固定である。`kit_version` は記録形式とバイナリの互換性の識別であり、各作業に記録する。

規範の識別に含めるのは、principles・stages・checks・plans・templates・rules・`devkit.toml`・サブエージェント定義・AGENTS.md・CLAUDE.md である。ラウンド開始時のスナップショットで固定する。requirements・structure・detect・`.devkit/run/` は含めず、これらの更新では規範の識別が変わらず、判定も失効しない。

ループ自身の変更(`meta_loop_edit`)は、規範・ホスト設定・skill・リポジトリ内で git が追跡する有効な pre-commit の設定の変更である。要件と構造の更新は作業の成果としてステージの判定で見る。既定の `[approval] meta_loop_edit = "confirm"` では、どのステージの変更でも効力を持つ前に人が確認する。新たな未確認の変更はラウンドを開く前に問いで確認し、最後のラウンドの変更は完了時のコミット前に確認する。却下された変更が同じまま残る場合は、作業対象を保持して取り消す作業ラウンドを指示し、再確認を求めない。`"allow"` も選べる。

バイナリの配布、導入時に `init` が用意するものと `uninstall` による削除は [install](architecture/modules.md#install)、hook の登録先と設定の追記は[権限の境界](architecture/guarantees.md#権限の境界)に従う。

導入後の照合は [doctor](architecture/modules.md#doctor-の確認項目)に従う。

`devkit init` は人の確認後に導入したファイルをコミットし、`--no-commit` で省略できる。HEAD がない場合や、作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合は、`start` が理由と直し方を返して拒否する。

## 実行の仕組みとの対応

- **skill**: 作業の進め方をセッションに伝える手順である。開始、`next`、`submit`、`pause`、`resume`、問いの見せ方、作業者・判定者・計画者のサブエージェントの起動、振り返りを扱う。教訓の反映先の提案は[メタループ](meta-loop.md#メタループ)に従う。配置と指示ファイルとの関係は[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)、手順は[処理の流れ](architecture/flow.md)、教訓の記録と改善は[メタループの入口](meta-loop.md#メタループ)を参照する。
- **[runtime](parts/runtime.md)(実行を担う部品)**: セッションからの CLI 呼び出しとホストのイベントを受け取り、進行の指示と観測の記録を返す。責務の分担は[全体構成](architecture.md#全体構成)、作業の提出と完了は[作業結果の提出](architecture/flow.md#作業結果の提出)と[完了条件](architecture/guarantees.md#完了条件)に従う。
- **ロールと実行主体**: ロール(作業や判断を担う役割)と実行主体(runner。check を実行する相手)を区別する。語彙と割り当ては[計画](architecture/data.md#計画-plan)、check の実行主体の指定は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。計画の作成は[作業の開始](architecture/flow.md#作業の開始)で計画者が担う。

skill は `.agents/skills/devkit/`、サブエージェント定義は `.claude/agents/` と `.codex/agents/` に置く。

`init` は単位の作業者 `devkit-worker` の定義も両ホストへ配る。雛形のステージの `split` と計画者が書く `units` で作業を分け、単位がなければ通常のステージとして進める。既定の雛形は分割を使わない。作業者は指定された worktree だけを編集し、判定者用の書き込み禁止 `[deny] judge` の対象外とする。結果をすべて受理した後の SHA の固定・合流・ステージ全体の判定は[分割した作業と合流](architecture/flow.md#分割した作業と合流)に従う。

ループ自体を並行して回す場合は、別の worktree で新しいセッションを開く。各チェックアウトは自分の `.devkit/run/` を持ち、作業中のチェックアウトでの2つ目の `start` は拒否する。

brief の本文には、そのステージ定義、その判定の観点と参照する原則、規則、要件、関係する構造の記録だけを入れる。他の規範は論理名とパスの一覧で渡し、必要に応じて読ませる。パスはラウンド開始時のスナップショット、ラウンド外の判定なら指示を出す時点の作業ブランチの `HEAD` を `snap/` に展開した場所である。編集中の規範を参照させない。作業の範囲はスコープの出力と規則で渡す。

成果の requirements と structure は設定済みの作業ツリー上のパスと存在の有無を示し、requirements が未作成なら作成先を案内する。規則は固定した参照元に必須である。出力スキーマは参照先をすべて同梱し、作業の brief には note の種別と各スキーマを含める。すべての作業の brief と判定依頼が指示発行時点の作業履歴ファイルを参照する。

規範にあるコード書式の `principles/…`・`checks/…`・`stages/…`・`templates/…`・`rules.md` は、リポジトリ側の資産の論理名である。brief が設定で解決した実際のパスとの対応を示す。`schemas/…` はバイナリが出力スキーマの本文として渡す。

skill の手順では、再現手順の保存先とバージョンを修正開始前に、仮説の結果を次の仮説を始める前に [devkit note](architecture/data.md#決定と証拠-decision--evidence)で記録する。

人への問いと回答はセッションのチャットで扱い、CLI が受理して進行を再開する([人の回答と再開](architecture/flow.md#人の回答と再開))。計画の作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

人の依頼でエージェントが呼ぶ、または人が直接実行する `devkit pause`・`devkit resume` の条件と、Stop の継続上限は[中断](architecture/guarantees.md#中断)に従う。

リポジトリごとの進行は `devkit.toml` で設定する。

- `[git] branch = "devkit/{work_id}"` は作業ブランチの名前の形であり、`{work_id}` を必ず含める。
- `[git] commit_message = "devkit: {stage} ({work_id})"` は完了時のコミットメッセージの形であり、`{stage}`・`{work_id}`・`{task}` を使える。
- `[git] allow_empty_commit = true` は成果のないステージでも空のコミットを作る。`false` なら変更のないステージはコミットせず、その時点の HEAD で完了を記録する。
- `[classification] dependency_files` は依存の宣言として扱うファイルの glob の一覧であり、既定の一覧を変更できる。分類表の追加・削除に未知の分類名を書けば設定エラーとする。
- `[limits] stop_continuation = 3` は Stop の継続上限であり、1以上で無効にはできない。計画の check のやり直しの上限(既定3)とラウンドの上限は雛形が持つ。
- `[parallel] max_workers = 3` は同時に動かす単位の作業者数の上限であり、1以上を必須とする。1でも同じ worktree と合流の手順で単位を1つずつ進める。
- 検証コマンドの期限は `[commands.<name>]` の登録ごとに必須とする。
