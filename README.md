# dev-kit

dev-kit は、Claude Code や Codex CLI のセッション内で、リポジトリ専用の開発ループを回すための道具である。

dev-kit はこのリポジトリ、kit は提供物の総称、`devkit` はバイナリである。リポジトリの持ち主が[ループの雛形](docs/architecture/data.md#ループの雛形-plantemplate)(作業の段階と判定・承認の組み合わせ)を組む。kit は[7つの部品](docs/design.md#部品)、原則、[固定物](docs/architecture/modules.md#kit)(バージョン付きの資産)、`devkit` を提供する。

## Installation

Install Go (the version required by `go.mod`) and Git, then clone this repository. Run the script from the repository you want to initialize:

```sh
export PATH="$HOME/.local/bin:$PATH"
cd /path/to/your-repository
/path/to/dev-kit/install.sh
```

The script builds `devkit`, installs it in `~/.local/bin`, and runs `devkit init` in the current checkout. Pass a different directory on `PATH` as its optional argument. Existing repository settings and assets are preserved; missing kit settings, including deny defaults, are merged.

In Codex, trust the project interactively and trust each project hook through `/hooks`. In Claude Code, accept workspace trust. Start a new host session and run `devkit doctor`, then use `/devkit` to begin. Run `devkit help` for command arguments. See [install](docs/architecture/modules.md#install) and [doctor](docs/architecture/modules.md#doctor-の確認項目) for the design.

## 参照順序

1. [docs/architecture.md](docs/architecture.md): 構成と動作の概要。作業の流れ、保証の範囲。
2. [docs/design.md](docs/design.md): 基本方針と、7つの部品の役割。
3. [principles/README.md](principles/README.md): [check](docs/parts/check.md)(評価と判定の部品)が評価基準とする原則の一覧と、原則の書き方。

設計の正本は [docs/architecture.md](docs/architecture.md) と [docs/architecture/](docs/architecture/)である。`docs/parts/` と `docs/` 直下の各文書の記述が正本と食い違う場合は、正本を優先する。作業手順は[処理の流れ](docs/architecture/flow.md)に従う。

中断・再開の操作と条件、Stop の継続上限は[中断](docs/architecture/guarantees.md#中断)に従う。

## ファイル構成

- [docs/architecture.md](docs/architecture.md): 設計の概要。
- [docs/architecture/](docs/architecture/): 設計の詳細。
  - [flow.md](docs/architecture/flow.md): 開始から完了までの流れ。
  - [data.md](docs/architecture/data.md): 記録と状態。
  - [modules.md](docs/architecture/modules.md): モジュールと依存関係。
  - [layout.md](docs/architecture/layout.md): ディレクトリ構成。
  - [hooks.md](docs/architecture/hooks.md): 5つの hook。
  - [guarantees.md](docs/architecture/guarantees.md): 失敗ごとの防止の仕組み。
  - [examples.md](docs/architecture/examples.md): 雛形・計画・設定・観点・pre-commit の最小の例。
- [docs/design.md](docs/design.md): 基本方針と部品の一覧。
- [docs/parts/](docs/parts/): 部品ごとの契約。
- [docs/loop-plan.md](docs/loop-plan.md): ループの雛形(利用者が組む定義)と課題ごとの計画の形式・作成規則。
- [docs/stages.md](docs/stages.md): 雛形に置くステージの候補。
- [docs/checks.md](docs/checks.md): [観点ファイル](docs/architecture/modules.md#check)(check の判定基準を記すファイル)の候補。
- [docs/rules.md](docs/rules.md): 対象のコードと文書に課す規則と強制手段の設計上の一覧。利用側の正本は [Rules](docs/architecture/data.md#規則と強制手段-rules)に従う。
- [docs/repository.md](docs/repository.md): 利用側の配置と実行の仕組みとの対応。
- [docs/meta-loop.md](docs/meta-loop.md): ループ自体を改善する手順。
- [principles/](principles/): 原則。適用場面ごとにファイルを分けている。
