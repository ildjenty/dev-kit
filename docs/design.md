# dev-kit の設計

dev-kit は、リポジトリ専用の開発ループを回すための道具である。Claude Code や Codex に開発を任せると、手順を飛ばす、[check](parts/check.md)(評価と判定の部品)の判定を自分で `PASS`(合格)にする、未完了でも完了を主張する、セッションをまたぐと文脈を失うことがある。判定値の定義は [check の結果](architecture/data.md#check-の結果-judgment)に従う。

リポジトリの持ち主が「どの順で進み、どこで check を通し、どこで人が見るか」を[ループの雛形](architecture/data.md#ループの雛形-plantemplate)(リポジトリごとのループの定義)として組む。エージェントは普段のセッションでそのループに従い、`devkit` が順序・完了・記録を決定的に守らせる。指示ファイルや [skill](architecture/layout.md#指示ファイルとホスト設定)(セッションに伝える作業手順)で頼むだけでは、順序と完了条件を守らせる力がない。

kit は、7つの部品、規範の既定、`devkit` を提供する。`init` は文章の規範をリポジトリが所有する場所へコピーし、以後はそのリポジトリの資産として育てる。部品と組み立てが満たす原則の既定は [`assets/principles/harness.md`](../assets/principles/harness.md)である。部品はループを支える機能の単位であり、雛形に組み込む要素の形式と既定は kit が用意する。

## 基本方針

- まっさらなリポジトリでも、利用者は要求を伝えるだけで、エージェントが要件定義・設計・実装・テストを安全に回せるようにする。作業者・計画者・判定者を信用せず、人の合意、機械の検査、独立した判定、記録で品質を揃える。
- 同じモデルを使っても、ループの組み方によって結果は変わる。うまくいかないときは、モデルを替える前にループを見直す。
- 手続きは、複雑にすればよいわけではない。既定では最も簡素な手順を採用し、必要な場合に限って追加する。
- 通常の LLM が間違えないことに仕組みを作らず、安全な方向の操作に確認を付けない。リポジトリの都合で変えるものは既定値付きの設定にする。人の中断、有限の上限、JSON Schema、判定結果の受理境界、外部操作の承認と二重適用の防止は設定で無効にしない。
- 利用者が[ステージ](stages.md)(作業を分ける段階)、check、承認ゲート(人の承認を待つ区切り)をループの雛形として組む。課題ごとの[計画](architecture/data.md#計画-plan)(雛形から作る実行内容)を作る手順と取捨の範囲は[作業の開始](architecture/flow.md#作業の開始)、承認は[計画の check と承認ゲート](architecture/flow.md#計画の-check-と承認ゲート)に従う。
- 雛形の `split` と計画の `units` で、1ステージの作業を複数の作業者に分けられる。互いに依存しない単位の作業、検証コマンド後の判定、check ごとの判定直前の指示を並行して進められ、合流した成果を通常のステージのラウンドで判定する。ゲート・確認・上限・戻り・再計画・問いもステージ単位である。既定の雛形は複数単位への分割を使わない。別の worktree で新しいセッションを開けば、ループ自体も並行して回せる。
- ループの外にいる人が、次の判断に必要な現在の前提を把握できていない状態を「理解の負債」と呼ぶ。その返済とは、現在の前提を把握することである。人が戻ってきたときは、現在の要約と自分がすべきことを最初に読み、差分は経過を追うための補助として使う。
- 規則と原則の分担は [principles](../assets/principles/README.md#principles)に従う。利用側の規則の正本と配置は [Rules](architecture/data.md#規則と強制手段-rules)、規則の既定の供給は [kit](architecture/modules.md#kit)に従う。
- 計画の承認を通して要件を人と合意し、承認された完了条件を完了の基準にする。最初の計画、完了条件の ID または文面を変える計画、雛形から逸脱する計画、外部操作を宣言する計画を既定では人が承認する。同じ合意の範囲の再計画は再確認しない。完了条件ごとの確認手段は[計画](architecture/data.md#計画-plan)に従う。`open_questions` が空でない計画は、承認方針によらず人の承認を必須とする。
- 毎回の振り返りで問題と見逃しを調べ、機械の検査、雛形、用途別の判定基準、原則、skill の順に再発防止の反映先を選ぶ。リポジトリ内の書き込めるソースにだけ反映し、リポジトリ外のソースと読み取り専用のソースへの改善は教訓として管理者へ引き継ぐ。優先順は `[lessons] destinations` で変更できる。規範全体の棚卸しは必要が見えたときだけ行う。
- 規範はリポジトリが所有し、名前付きの[知識のソース](architecture/data.md#知識のソース)として管理する。接続先と配置は `[sources.<名前>]` で変えられる。既定は `.devkit/principles/`・`.devkit/stages/`・`.devkit/plans/`・`.devkit/rules.md`・`.devkit/checks/`・`.devkit/templates/` である。成果の要件と構造は `.devkit/requirements.md`・`.devkit/structure.json` に置く。合格の索引はソースではなく `[state] detect`(既定 `.devkit/detect.json`)で設定する。リポジトリ外のソースは読み取り専用であり、devkit は書き込まない。JSON Schema、分類表と役割の既定、ホスト設定の断片はバイナリが持つ。`kit_version` は記録形式とバイナリの互換性の識別である。
- 誰が作業・判定・計画・回答を担うかは、リポジトリが `devkit.toml` の[役割](architecture/data.md#役割-role)で定義し、雛形と計画に役割の ID で書く。devkit は役割の中身を解釈せず、種類(`work`・`judge`・`plan`・`human`)ごとの規則と職務の分離だけを守らせる。既定の4つの役割(`worker`・`judge`・`planner`・`human`)で、追加の設定なしに動く。修正が続いたときに担当を変える分岐は、記録から決まる事実を条件にする規則の一覧として書き、条件式の言語は持たない。

check の基準は `criteria`、検出試験の例は `examples` の選び方で明示する。観点 ID は判定の名前であり、確認事項や判定範囲は原則のソースの用途別文書で定める。判定規則と出力の約束は判定依頼テンプレートに集約する。規範はソース全体を固定し、判定・承認・反映・完了は git のツリーと外部の成果の文書を合わせた同じ識別に結び付ける。

既定の `worker` は `kind = "work"`、`agent = "devkit-worker"` であり、devkit が作る自分の worktree で1つの単位として作業し、合流後に判定する。セッションのエージェントは起動・待機・人への問い・git の規則に従う統合を担う。agent を空にした `work` の役割を定義すればセッション自身の作業も使える。git の運用は既定 `.devkit/git.md` の散文の規則を雛形の入力で選び、配置は設定で変更できる。devkit は `start` でブランチを作らず、作業はチェックアウトの記録で特定する。

依頼文の[入力](architecture/data.md#依頼の入力)は採用された成果・今の材料・経緯を分け、同じ本文を1回だけ渡す。`report` は作業者の申告であり、証拠や完了の根拠にしない。

## 部品

部品の提供単位と構成は[モジュールごとの契約](architecture/modules.md#モジュールごとの契約)に従う。計画が取捨するのは雛形に定めた要素であり、提供単位である部品は選択の対象にしない。

- [brief](parts/brief.md) — 作業依頼のフォーマット
- [check](parts/check.md) — 評価と判定
- [router](parts/router.md) — 次の行動の分類
- [ledger](parts/ledger.md) — 状態の外部保存
- [form](parts/form.md) — 固定フォーマット
- [view](parts/view.md) — 人向けの表示
- [runtime](parts/runtime.md) — 実行

[検証コマンド](architecture/modules.md#config)(`[commands]` に登録するテスト・型検査など)を呼ぶ `command:` の check は [config](architecture/modules.md#config)、router の4分類をキーにする遷移表と、`fix` の遷移に書く担当の規則の形式は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。

各部品の実装モジュール(計画の検証・遷移・上限判定を扱う [plan](architecture/modules.md#plan) を含む)、資産の配置、依存は [architecture.md](architecture.md) に従う。正本と優先関係は [README の参照順序](../README.md#reading-order)に従う。
