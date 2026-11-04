# 利用側リポジトリの構成

対象は、個人開発と、複数人が同じリポジトリで作業するチーム開発の両方である。

## リポジトリ側の構成

`init` が既定のソースの初期内容をリポジトリへコピーし、以後はリポジトリが所有する知識として育てる。接続先と配置は `devkit.toml` の `[sources.<名前>]` で変更できる。アダプタは必須の「選ぶ」と元のバージョンの識別子を扱い、「書く」は比較と更新を一体で行えることを保証できるアダプタのための口として定める。[知識のソース](architecture/data.md#知識のソース)の契約に従う。

- 規範(`norm = true`): principles は `.devkit/principles`、stages は `.devkit/stages`、plans は `.devkit/plans`、rules は `.devkit/rules.md`、git は `.devkit/git.md` をそれぞれ1文書として、templates は `.devkit/templates` を既定とする。
- 成果(`norm = false`): requirements は `.devkit/requirements.md`、structure は `.devkit/structure.json` を既定とする。

規範のソースは空の選び方で全体を列挙し、固定と変化の確認を全体に対して行う。成果のソースは選んだ範囲を固定する。リポジトリ内の `files` は git のスナップショットから読み、外部はその時点の本文を写す。リポジトリ外のソースは読み取り専用であり、`writable = true` を書くと設定エラーになる。書き込みはリポジトリ内の `writable = true` のソースに限る。

各リポジトリは資産と実行の記録を分けて管理する。

- **指示ファイル(AGENTS.md と CLAUDE.md)**: 管理・更新方針と判定の規範としての扱いは[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)に従う。
- **配置ガイド**: 構造の記録に、置くもの・置かないもの・配置先の案内を残す。構造のステージが更新し、brief には関係する構造の記録を入れる。
- **元に戻せないものと外部操作**: 元に戻せないものの共有項目は規則、課題固有の項目は[計画](architecture/data.md#計画-plan)(パターンを参考に課題ごとに組む実行内容)で扱う。外部操作(外部への書き込みの行為)の登録は [config](architecture/modules.md#config)、宣言と承認は[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従う。
- **育てるもの**: 原則と用途別の判定基準・ステージ・パターン・テンプレート・規則をリポジトリの規範として育てる。リポジトリ内のファイルは git で共有する。kit の埋め込みとの一致は要求しない。
- **要件**: 目的のステージが、人が承認した完了条件と既存の要件を照らして更新する。目的がないパターンや目的を外した計画では、振り返りが要件に当たる完了条件を反映する。目的で反映済みなら重ねて更新する必要はない。後の作業の計画者・作業者・判定者へ渡す入力の選び方の既定値に含める。
- **規則と強制手段**: 複数の作業で守る変更困難な決定を残す。更新はループ自身の変更として人が確認する。「手段: 未実装」と振り返りで繰り返し見つかる違反は、機械の検査へ移す候補にする。
- **設定ファイル**: `devkit.toml` の項目は [config](architecture/modules.md#config)、配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。[判定コマンド](architecture/modules.md#config)(`[judges]`)は別エンジンによる判定に使う。[検証コマンド](architecture/modules.md#config)(`[commands]`)はテスト・型検査などに使い、`command:<name>` の [check](parts/check.md)(評価と判定の部品)から呼ぶ。実行の扱いは[作業結果の判定](architecture/flow.md#作業結果の判定)に従う。
- **実行の記録と生成物**: `.devkit/run/` に固定し、git 管理しない。サブエージェント定義・hook・保護の設定・ignore がこの場所を前提にし、途中で変えると記録を見失うためである。後の作業に効かせたい決定は要件・構造・規則へ残す。記録から組み立てる [checkpoint](architecture/data.md#checkpoint)(再開用の派生状態)は保存する記録と区別する。
- **単位の作業場所**: `.devkit/worktrees/<単位の指示 ID>/` に detached の worktree を作り、ignore する。記録の保護と重ならないよう `.devkit/run/` と分ける。中断中は保持し、中止・作業完了時は変更のない worktree だけを削除する。変更の残るものは保持して場所を知らせ、破棄は人が git で行う。
- **役割**: 作業・判定・計画・回答の担当を `devkit.toml` の `[roles.<id>]` で定義する。既定は `worker`・`judge`・`planner`・`human` の4つであり、上書き・追加できる。役割を足すときは、その agent の定義を `.claude/agents/`・`.codex/agents/` に置く。種類ごとの規則と職務の分離は[役割](architecture/data.md#役割-role)に従う。役割の定義と agent の定義は規範として固定し、変更はループ自身の変更として扱う。
- **バイナリが持つもの**: 記録とバイナリの約束である JSON Schema、分類表と役割の既定、ホスト設定の断片を持つ。人への確認の分類は `devkit.toml` の `[router] human_confirmation` で上書き・追加・削除できる。check の結果から次の行動への対応は固定である。`kit_version` は記録形式とバイナリの互換性の識別であり、各作業に記録する。

規範の識別は、規範のソース全体の全文書と、ソースの外の `devkit.toml`・サブエージェント定義・AGENTS.md・CLAUDE.md から求める。成果のソース・実行の記録は規範の識別に含めない。成果の更新は規範の変更による判定の失効を起こさないが、判定対象の識別には含める。判定・承認・反映・完了は、git のツリーに外部の成果の文書のソース名・ID・内容ハッシュの集合を加えた同じ識別を指す。

ループ自身の変更(`meta_loop_edit`)は、規範・ホスト設定・skill・リポジトリ内で git が追跡する有効な pre-commit の設定の変更である。要件と構造の更新は作業の成果としてステージの判定で見る。既定の `[approval] meta_loop_edit = "confirm"` では、どのステージの変更でも効力を持つ前に人が確認する。新たな未確認の変更はラウンドを開く前に問いで確認し、最後のラウンドの変更は完了時のコミット前に確認する。却下された変更が同じまま残る場合は、作業対象を保持して取り消す作業ラウンドを指示し、再確認を求めない。`"allow"` も選べる。

バイナリの配布、導入時に `init` が用意するものと `uninstall` による削除は [install](architecture/modules.md#install)、hook の登録先と設定の追記は[権限の境界](architecture/guarantees.md#権限の境界)に従う。

導入後の照合は [doctor](architecture/modules.md#doctor-の確認項目)に従う。

`devkit init` は人の確認後に導入したファイルをコミットし、`--no-commit` で省略できる。HEAD がない場合や、作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合は、`start` が理由と直し方を返して拒否する。

git の運用は規範のソース `git` の文書で定め、計画のステージの入力で選ぶ。既定の場所は `.devkit/git.md` であり、ソースの設定で変更できる。既定の規則は、作業ごとに `feat/`・`fix/`・`chore/`・`refactor/`・`docs/` と短い説明を組み合わせた作業ブランチを用意し、コミットメッセージにも同じ種類の接頭辞を付け、完了後に起点のブランチ(多くは main)へ統合するものである。セッションのまとめ役が `start` の前のブランチ準備と完了後の統合を担う。push と PR の作成は外部操作の経路を通す。

## 実行の仕組みとの対応

- **skill**: 作業の進め方をセッションに伝える手順である。開始、`next`、`submit`、`pause`、`resume`、問いの見せ方、作業者・判定者・計画者のサブエージェントの起動、振り返りを扱う。教訓の反映先の提案は[メタループ](meta-loop.md#メタループ)に従う。配置と指示ファイルとの関係は[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)、手順は[処理の流れ](architecture/flow.md)、教訓の記録と改善は[メタループの入口](meta-loop.md#メタループ)を参照する。
- **[runtime](parts/runtime.md)(実行を担う部品)**: セッションからの CLI 呼び出しとホストのイベントを受け取り、進行の指示と観測の記録を返す。責務の分担は[全体構成](architecture.md#全体構成)、作業の提出と完了は[作業結果の提出](architecture/flow.md#作業結果の提出)と[完了条件](architecture/guarantees.md#完了条件)に従う。
- **役割と実行主体**: [役割](architecture/data.md#役割-role)(作業や判断を担う担当)は `devkit.toml` で定義し、ステージの担当・check の実行主体(runner)・単位の担当に役割の ID を書く。runner には役割のほか判定コマンドと検証コマンドも書ける。割り当ては[計画](architecture/data.md#計画-plan)、check の実行主体は `[checks.<観点>]` の一覧に定める。計画の作成は[作業の開始](architecture/flow.md#作業の開始)で `[planning] role` の計画者が担う。

skill は `.agents/skills/devkit/`、サブエージェント定義は `.claude/agents/` と `.codex/agents/` に置く。

`init` は既定の役割 `worker` の agent `devkit-worker` の定義も両ホストへ配る。計画のステージの `split` と計画者が書く `units` で作業を分け、単位がなければ通常のステージとして進める。既定のパターンは複数単位への分割を使わない。`work` の役割の agent は指定された worktree だけを編集し、判定者用の書き込み禁止 `[deny] judge` の対象外とする。agent のある `work` の役割がステージの担当になったラウンドも、単位が1つの同じ手順で動かす。結果をすべて受理した後の SHA の固定・合流・ステージ全体の判定は[分割した作業と合流](architecture/flow.md#分割した作業と合流)に従う。

ループ自体を並行して回す場合は、別の worktree で新しいセッションを開く。各チェックアウトは自分の `.devkit/run/` を持ち、作業中のチェックアウトでの2つ目の `start` は拒否する。

本文で渡す知識は、`[planning] inputs`・`[plan_check] inputs`・計画のステージの `inputs` と、`[checks.<観点>] criteria` で選ぶ。要件と規則を渡す既定値を用意する。ステージの実体の `definition` でステージ定義を解決する。規範はソース全体を固定し、成果は選んだ本文を固定する。その他の規範はソース名・文書 ID・固定した写しのパスの一覧で渡し、編集中の原本を読ませない。文中の参照を解析して入力を補わず、必要な文書は選び方に明示する。変更の範囲は scope の出力と rules で渡す。

成果の requirements と structure は、固定した本文とソース名・文書 ID・編集先・存在の有無を示す。要件が未作成なら作成先を案内する。出力スキーマの参照先と check の `schemas` に列挙した入力契約のスキーマを同梱する。補助のスキーマはその和集合だけを渡す。依頼の経緯は `history` で決定的に絞った固定した索引から参照する。

規範にあるコード書式の `principles/…`・`stages/…`・`templates/…`・`rules.md` は、リポジトリ側の資産の論理名である。brief が設定で解決した実際のパスとの対応を示す。`schemas/…` はバイナリが出力スキーマの本文として渡す。

skill の手順では、再現手順の保存先とバージョンを修正開始前に、仮説の結果を次の仮説を始める前に [devkit note](architecture/data.md#決定と証拠-decision--evidence)で記録する。

人への問いと回答はセッションのチャットで扱い、CLI が受理して進行を再開する([人の回答と再開](architecture/flow.md#人の回答と再開))。計画の作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

人の依頼でエージェントが呼ぶ、または人が直接実行する `devkit pause`・`devkit resume` の条件と、Stop の継続上限は[中断](architecture/guarantees.md#中断)に従う。

リポジトリごとの進行は `devkit.toml` で設定する。

- ステージの完了時には、判定した対象を現在のブランチに devkit がコミットする。メッセージは作業者が終了ブロックの `commit_message` に git の規則に沿って書いた値を使う。省略時は `[git] commit_message`(既定 `"devkit: {stage} ({work_id})"`)のテンプレートで補い、`{stage}`・`{work_id}`・`{task}` を使える。
- `[git] allow_empty_commit = true` は成果のないステージでも空のコミットを作る。`false` なら変更のないステージはコミットせず、その時点の HEAD で完了を記録する。
- `[classification] dependency_files` は依存の宣言として扱うファイルの glob の一覧であり、既定の一覧を変更できる。分類表の追加・削除に未知の分類名を書けば設定エラーとする。
- `[limits] stop_continuation = 3` は Stop の継続上限であり、1以上で無効にはできない。計画の判定のやり直しは `[planning] max_rounds`(既定3)で定める。ステージの上限は計画に書き、`[limits] max_rounds_ceiling`(既定3)と `max_no_progress_ceiling`(既定2)を超えてはならない。
- 同時に動かす数は全体の `[parallel] max_concurrent`(既定3、1以上)と役割ごとの `[roles.<id>] max_concurrent`(指定時は1以上)の両方を満たす。数えるのは発行済みでまだ受理していないサブエージェントの指示である。互いに依存しない単位の作業、検証コマンド後の判定者の check、check ごとの判定直前の指示をまとめて返し、空いた分だけ次の `next` で発行する。役割の上限を省略した場合も全体の上限を守る。
- `[briefs] max_bytes = 200000` は依頼文の本文の上限である。超える分はファイルにして一覧で渡し、超過を記録する。
- `[limits] max_total_rounds` の既定値は40であり、作業全体の累計が達したときに人に確認し、続行ならその時点の累計に設定値を足した値を次の停止点とする。戻り・作り直し・担当変更でも累計を減らさない。
- 検証コマンドの期限は `[commands.<name>]` の登録ごとに必須とする。

メインのエージェントは壁打ちで合意した内容を `start` の課題に書く。`devkit start --task-file <ファイル>` は課題の本文をファイルから読む指定であり、課題引数と併用しない。途中の `devkit instruct <ファイル>` は指示を記録し、次の計画者・作業者へ渡す。判定者へは渡さない。計画や完了条件を変える場合は再計画と承認を通す。会話の写しは取らず、サブエージェントは1行の指示 ID だけで起動する。

check の中身は `[checks.<観点>]`、絶対に守らせる決まりは `[[loop.stage]]` と `[[loop.path]]` に置く。パターンは参考であり、計画者が毎回ループを組む。定義・参照の解決と経路の検証は[計画](architecture/data.md#計画-plan)に従う。

リポジトリ外のソースは読み取り専用であり、devkit は書き込まない。リポジトリ内の書き込めるソースのファイルは直接編集して git の差分を取り、完了時のコミットで反映する。リポジトリ外のソースと読み取り専用のソースへの改善は教訓として管理者に引き継ぐ。
