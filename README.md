# dev-kit

dev-kit はこのリポジトリ、kit は提供物の総称、`devkit` はバイナリである。

リポジトリの持ち主がステージ・観点・ゲート・検証コマンドをループの雛形として組み、Claude Code や Codex CLI のセッション内で、リポジトリ専用の開発ループを回すための道具。kit は部品（brief、check、router、ledger、form、view、runtime の7つ）と原則、既定の雛形、`devkit` からなる。現在は設計段階で、コードはまだない。

## 参照順序

1. [docs/architecture.md](docs/architecture.md): 構成と動作の概要。作業の流れと保証の範囲を示す。
2. [docs/design.md](docs/design.md): 基本方針と、7つの部品の役割。
3. [principles/README.md](principles/README.md): check が評価基準とする原則の一覧と、原則の書き方。

設計の正本は [docs/architecture.md](docs/architecture.md) と [docs/architecture/](docs/architecture/)。`docs/parts` と loop-plan・repository・stages・checks の記述が食い違うときは [architecture/contract-changes.md の改訂](docs/architecture/contract-changes.md)に従う。

その先は、必要な箇所を読む。

## ファイル構成

- `docs/architecture.md`: 設計の概要。
- `docs/architecture/`: 設計の詳細。`flow.md`（開始から完了までの流れ）、`data.md`（記録と状態）、`modules.md`（モジュールと依存関係）、`layout.md`（ディレクトリ構成）、`hooks.md`（5つの hook）、`guarantees.md`（どの失敗をどの仕組みで防ぐか）、`decisions.md`（仮決定、不採用の案、未確認の前提）、`contract-changes.md`（部品の契約の改訂と試験）。
- [docs/architecture/examples.md](docs/architecture/examples.md): 雛形・計画・設定・観点・pre-commit の最小の例。
- `docs/design.md`: 基本方針と部品の一覧。
- `docs/parts/`: 部品ごとの契約。`brief.md`、`check.md`、`router.md`、`ledger.md`、`form.md`、`view.md`、`runtime.md`。
- `docs/loop-plan.md`: 課題ごとに作るループ計画の形式。`docs/stages.md` は雛形に置くステージの候補、`docs/checks.md` は観点ファイルの候補。
- `docs/rules.md`: 対象のコードと文書に課す規則と、それを守らせる手段。
- `docs/repository.md`: 利用側の配置（正本は [architecture/layout.md](docs/architecture/layout.md)）。
- `docs/meta-loop.md`: ループ自体を改善するループ。
- `principles/`: 原則。適用する場面（設計、構造、データ、実装、テスト、確認、進め方、文脈、harness、UI、文章）ごとにファイルを分けている。
