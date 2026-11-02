# dev-kit の設計

dev-kit は、リポジトリ専用の開発ループを回すための道具である。Claude Code や Codex に開発を任せると、手順を飛ばす、[check](parts/check.md)(評価と判定の部品)の判定を自分で `PASS`(合格)にする、未完了でも完了を主張する、セッションをまたぐと文脈を失うことがある。判定値の定義は [check の結果](architecture/data.md#check-の結果-judgment)に従う。

リポジトリの持ち主が「どの順で進み、どこで check を通し、どこで人が見るか」を[ループの雛形](architecture/data.md#ループの雛形-plantemplate)(リポジトリごとのループの定義)として組む。エージェントは普段のセッションでそのループに従い、`devkit` が順序・完了・記録を決定的に守らせる。指示ファイルや skill で頼むだけでは、順序と完了条件を守らせる力がない。

kit は、下記の7つの部品、原則、固定物(既定の雛形・観点ファイル・skill・サブエージェント定義など)、`devkit` を提供する。部品と組み立てが満たす原則は [`principles/harness.md`](../principles/harness.md)、固定物の範囲は [kit](architecture/modules.md#kit)に従う。部品はループを支える機能の単位であり、雛形に組み込む要素の形式と既定は kit が用意する。検証コマンドの呼び出し口である `command:` の check は [config](architecture/modules.md#config)、[router](parts/router.md)(次の行動を分類する部品)の4分類をキーにする遷移表の形式は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。

## 基本方針

- 同じモデルを使っても、ループの組み方によって結果は変わる。うまくいかないときは、モデルを替える前にループを見直す。
- 手続きは、複雑にすればよいわけではない。既定では最も簡素な手順を採用し、必要な場合に限って追加する。
- 利用者が[ステージ](stages.md)(作業を分ける段階)、check、承認ゲート(人の承認を待つ区切り)をループの雛形として組む。課題ごとの[計画](architecture/data.md#計画-plan)(雛形から作る実行内容)を作る手順と取捨の範囲は[作業の開始](architecture/flow.md#作業の開始)、承認は[計画の check と承認ゲート](architecture/flow.md#計画の-check-と承認ゲート)に従う。
- ループの外にいる人が、次の判断に必要な現在の前提を把握できていない状態を「理解の負債」と呼ぶ。その返済とは、未読の変更や決定をなくすことではなく、現在の前提を把握することである。人が戻ってきたときは、現在の要約と自分がすべきことを最初に読み、差分は経過を追うための補助として使う。
- 機械で決定的に判定できる規則は原則にしない。ループの仕組みに関するものは[設計の正本](architecture.md)に、対象のコードと文書に関するものは[規則と強制手段](rules.md)の設計上の一覧に置く。利用側の規則の正本と配置は [Rules](architecture/data.md#規則と強制手段-rules)、規則の既定の供給は [kit](architecture/modules.md#kit)に従う。

## 部品

部品は、契約、テンプレート、試験を持つ提供単位である。部品は `principles/harness.md` の原則を満たすように作り、契約は「責務 / 入力と出力 / 保証 / 試験項目」の見出し構成に揃える。仕組みは設計の正本で定め、契約から該当する節へ参照する。計画が取捨するのは雛形に定めた要素であり、提供単位である部品は選択の対象にしない。部品は次の7つとし、試験のない部品は kit に含めない。

- [brief](parts/brief.md) — 作業依頼のフォーマット
- [check](parts/check.md) — 評価と判定
- [router](parts/router.md) — 次の行動の分類
- [ledger](parts/ledger.md) — 状態の外部保存
- [form](parts/form.md) — 固定フォーマット
- [view](parts/view.md) — 人向けの表示
- [runtime](parts/runtime.md) — 実行

各部品の実装モジュール(計画の検証・遷移・上限判定を扱う [plan](architecture/modules.md#plan) を含む)、kit の固定物、配置、依存は [architecture.md](architecture.md) で定める。設計の正本は [architecture.md](architecture.md) と [architecture/](architecture/) である。
