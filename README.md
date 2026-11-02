# dev-kit

Claude Code や Codex CLI のセッション内で、リポジトリ専用の開発ループを回すための道具。ループを構成する部品（brief、check、router、ledger、form、view、runtime）、部品が従う原則、課題ごとのループ計画、ループを実行する単一バイナリ `devkit` からなる。現在は設計段階で、コードはまだない。

## 参照順序

1. [docs/architecture.md](docs/architecture.md): 構成と動作の概要。1〜2画面で全体像をつかめる。
2. [docs/design.md](docs/design.md): 基本方針と、7つの部品の役割。
3. [principles/README.md](principles/README.md): check が評価基準とする原則の一覧と、原則の書き方。

その先は、必要な箇所を読む。

## ファイル構成

- `docs/architecture.md`: 設計の概要。
- `docs/architecture/`: 設計の詳細。`flow.md`（開始から完了までの流れ）、`data.md`（記録と状態）、`modules.md`（モジュールと依存関係）、`layout.md`（ディレクトリ構成）、`hooks.md`（5つの hook）、`guarantees.md`（どの失敗をどの仕組みで防ぐか）、`decisions.md`（仮決定、不採用の案、未確認の前提）、`contract-changes.md`（部品の契約の改訂と試験）。
- `docs/design.md`: 基本方針と部品の一覧。
- `docs/parts/`: 部品ごとの契約。`brief.md`、`check.md`、`router.md`、`ledger.md`、`form.md`、`view.md`、`runtime.md`。
- `docs/loop-plan.md`: 課題ごとに作るループ計画の形式。`docs/stages.md` は計画で選ぶステージの候補、`docs/checks.md` は check の観点ファイルの候補。
- `docs/rules.md`: 対象のコードと文書に課す規則と、それを守らせる手段。
- `docs/repository.md`: 利用側のリポジトリに置くもの。
- `docs/meta-loop.md`: ループ自体を改善するループ。
- `principles/`: 原則。適用する場面（設計、構造、データ、実装、テスト、確認、進め方、文脈、harness、UI、文章）ごとにファイルを分けている。
