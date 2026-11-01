# dev-kit の設計

dev-kit は、課題に応じた開発ループ（harness）を組み立てるための語彙を提供する。完成済みのループを配布するものではない。必要なループは課題ごとに異なるため、kit が提供するのは、ループを構成する部品、部品と組み立てが満たす原則（[`principles/harness.md`](../principles/harness.md)）、そして課題から[ループ計画](loop-plan.md)を作る手順に限る。

## 基本方針

- 同じモデルを使っても、ループの組み方によって結果は変わる。うまくいかないときは、モデルを替える前にループを見直す。
- 手続きは、複雑にすればよいわけではない。既定では最も簡素な手順を採用し、必要な場合に限って追加する。
- 必要な[ステージ](stages.md)、[check](parts/check.md)、人によるレビューゲートは課題ごとに異なる。課題に着手するときに決める。
- ループの外にいる人が、次の判断に必要な現在の前提を把握できていない状態を「理解の負債」と呼ぶ。その返済とは、未読の変更や決定をなくすことではなく、現在の前提を把握することである。人が戻ってきたときは、現在の要約と自分がすべきことを最初に読み、差分は経過を追うための補助として使う。

## 部品

部品は、契約（入力と出力の形式）、テンプレート、試験を持つ提供単位である。部品は `principles/harness.md` の原則を満たすように作り、どう満たすか（方式）は各部品の契約で定める。ループ計画が選ぶのは、ステージ、[観点](checks.md)、担当、レビューゲートであり、部品そのものではない。部品は次の7つとする。各部品には、契約・テンプレート・試験を用意し、試験のない部品は kit に含めない。

- [brief](parts/brief.md) — 作業依頼のフォーマット
- [check](parts/check.md) — 評価と判定
- [router](parts/router.md) — 次のアクションの決定
- [ledger](parts/ledger.md) — 状態の外部保存
- [form](parts/form.md) — 固定フォーマット
- [view](parts/view.md) — 人向けの表示
- [runtime](parts/runtime.md) — 実行

## ディレクトリ

```text
dev-kit/
├── README.md
├── docs/
│   ├── design.md
│   ├── parts/
│   │   ├── brief.md  check.md  router.md  ledger.md
│   │   └── form.md  view.md  runtime.md
│   ├── stages.md  checks.md  loop-plan.md
│   └── repository.md  meta-loop.md
├── principles/
│   ├── engineering.md  design.md  testing.md  verification.md
│   ├── working.md  ui.md  writing.md  harness.md
│   └── review.md          ← 生成物
├── parts/
│   ├── brief/  check/  router/  ledger/  form/  view/  runtime/
├── vocabulary/
│   ├── stages.md          ← ステージの候補
│   ├── checks/            ← 観点ファイル
│   └── routes/            ← router の表
├── plan/                  ← ループ計画を作る skill と、計画の形・例
├── assemblies/
│   ├── minimal/
│   └── issue-pr/
└── tools/
```

各 `parts/<name>/` は、契約を記載した `README.md`、テンプレートまたはスクリプト、試験で構成する。
