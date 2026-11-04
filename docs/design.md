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
- 雛形の `split` と計画の `units` で、1ステージの作業を複数の作業者に分けられる。並列にするのは作業だけであり、合流した成果を通常のステージのラウンドで判定する。ゲート・確認・上限・戻り・再計画・問いもステージ単位である。既定の雛形は分割を使わない。別の worktree で新しいセッションを開けば、ループ自体も並行して回せる。
- ループの外にいる人が、次の判断に必要な現在の前提を把握できていない状態を「理解の負債」と呼ぶ。その返済とは、現在の前提を把握することである。人が戻ってきたときは、現在の要約と自分がすべきことを最初に読み、差分は経過を追うための補助として使う。
- 規則と原則の分担は [principles](../assets/principles/README.md#principles)に従う。利用側の規則の正本と配置は [Rules](architecture/data.md#規則と強制手段-rules)、規則の既定の供給は [kit](architecture/modules.md#kit)に従う。
- 計画の承認を通して要件を人と合意し、承認された完了条件を完了の基準にする。最初の計画、完了条件の ID または文面を変える計画、雛形から逸脱する計画、外部操作を宣言する計画を既定では人が承認する。同じ合意の範囲の再計画は再確認しない。完了条件ごとの確認手段は[計画](architecture/data.md#計画-plan)に従う。`open_questions` が空でない計画は、承認方針によらず人の承認を必須とする。
- 毎回の振り返りで問題と見逃しを調べ、機械の検査、雛形、観点、原則、skill の順に再発防止の反映先を選ぶ。優先順は `[lessons] destinations` で変更できる。規範全体の棚卸しは必要が見えたときだけ行う。
- 規範はリポジトリが所有する。原則・ステージ・観点・雛形・テンプレート・規則の既定の配置は `.devkit/principles/`・`.devkit/stages/`・`.devkit/checks/`・`.devkit/plans/`・`.devkit/templates/`・`.devkit/rules.md` であり、`[paths]` で変更できる。kit の埋め込みとの一致を要求しない。JSON Schema、分類表とロールの既定、ホスト設定の断片はバイナリが持つ。`kit_version` は記録形式とバイナリの互換性の識別である。

## 部品

部品の提供単位と構成は[モジュールごとの契約](architecture/modules.md#モジュールごとの契約)に従う。計画が取捨するのは雛形に定めた要素であり、提供単位である部品は選択の対象にしない。

- [brief](parts/brief.md) — 作業依頼のフォーマット
- [check](parts/check.md) — 評価と判定
- [router](parts/router.md) — 次の行動の分類
- [ledger](parts/ledger.md) — 状態の外部保存
- [form](parts/form.md) — 固定フォーマット
- [view](parts/view.md) — 人向けの表示
- [runtime](parts/runtime.md) — 実行

[検証コマンド](architecture/modules.md#config)(`[commands]` に登録するテスト・型検査など)を呼ぶ `command:` の check は [config](architecture/modules.md#config)、router の4分類をキーにする遷移表の形式は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。

各部品の実装モジュール(計画の検証・遷移・上限判定を扱う [plan](architecture/modules.md#plan) を含む)、資産の配置、依存は [architecture.md](architecture.md) に従う。正本と優先関係は [README の参照順序](../README.md#reading-order)に従う。
