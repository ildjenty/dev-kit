# 配置

実装と埋め込み元は dev-kit のリポジトリに置き、利用側には設定、育てるもの、作業ごとの実行の記録と生成物を置く。利用側のチェックアウトを単位に配置を完結させ、並行する作業は別のチェックアウトに分ける。ディレクトリのツリーは、それぞれの置き場所と役割を示す。

### dev-kit のリポジトリ

```text
dev-kit/
  cmd/devkit/            組み立ての根。サブコマンドの定義と配線だけ
  internal/
    plan/                計画の意味論
    brief/               brief と判定依頼の組み立て
    check/               観点ファイル、判定、集約、適用除外、検出試験の仕様
    router/              分類表と判断
    form/                固定フォーマットのスキーマと描画
    ledger/              記録の型と問い合わせ
      fsstore/           記録ごとに独立したファイルを作る実装。生成物の公開も担う
    engine/              状態機械の進行、指示の計算、検証の実行、結果の受理
    external/            外部への書き込みの唯一の実行経路
    hook/                ホストのイベント処理と観測の書き出し
    host/event/          イベント JSON の変換と出力の整形
    workspace/           git の操作
    config/              devkit.toml の読み込み
    install/             導入と doctor
    view/                HTML の生成
  kit/                   固定物の埋め込み元
    stages/              ステージ定義
    checks/              観点ファイル
    templates/           form のテンプレート
    router/              分類表の既定
    rules/               規則の既定
    schemas/             JSON Schema
    skills/              skill の正本(devkit の進め方)
    instructions/        AGENTS.md の断片
    hosts/               ホスト設定、サブエージェント定義、rules の断片
    examples/            検出試験の違反例・適合例・許容例
  principles/            既存の原則。埋め込み対象
  docs/                  既存の設計書
    parts/               部品の契約。直接埋め込む正本
  embed.go               principles、docs/parts、kit の埋め込み
```

各ディレクトリの配置ガイドは、`doc.go` の先頭コメントまたは README に置く。[`principles/structure.md`](../../principles/structure.md) の原則1に従う。

### 利用側のリポジトリ

```text
<repo>/
  devkit.toml                kit のバージョン、検証コマンド、登録コマンド、外部操作、deny、分類表の追記。git 管理
  AGENTS.md                  指示の正本(既存のものに kit の案内を1行足す)。git 管理
  .claude/settings.json      hook の登録、excludedCommands、記録・結果・観測・展開先への Edit の deny、Bash(devkit *) の allow の提案。init が項目を足す。git 管理
  .claude/agents/devkit-*.md 判定者・計画者のサブエージェント定義。init が書く。git 管理
  .claude/skills/devkit -> ../../.agents/skills/devkit   skill 単位のリンク。git 管理
  .codex/hooks.json          hook の登録。init が項目を足す。git 管理
  .codex/rules/devkit.rules  devkit を sandbox の外で動かす allow。git 管理
  .codex/agents/devkit-*.toml 判定者・計画者のサブエージェント定義。git 管理
  .agents/skills/devkit/     skill の正本。Codex の置き場所
  .devkit/                   git 管理(run/ を除く)
    checks/<観点ID>.md        観点ファイル。違反例・適合例は checks/<観点ID>/ の下
    rules.md                 規則と強制手段。移行記録も項目として持つ
    detect.json              検出試験の合格記録の索引
    structure.json           構造ステージを使うときだけ
    kit/<バージョン>/         固定物のコピー。principles と部品の契約を含む
    run/                     実行の記録と生成物。.gitignore で除外
      sessions/<session_id>.json   セッションの観測。作業の外にチェックアウトごとに置く
      work/<作業ID>/
        records/<ULID>.json  追記のみの記録
        run.lock
        inbox/               エージェントが終了ブロック・計画・note の入力 JSON を書く唯一の場所
        prompts/             人の入力の観測(UserPromptSubmit が書く)
        brief/               next が書く brief と判定依頼
        snap/<対象のバージョン>/   判定者に読ませる対象の展開(read-tree と checkout-index --prefix)
        results/<指示ID>-<agent_id>.json   判定者の結果(SubagentStop が書く)
        view.html
```


チェックアウトのルートは `git rev-parse --show-toplevel` で取得し、直下の `devkit.toml` を入口にする。作業はセッションの cwd であるチェックアウトで、[Work のブランチ](data.md#作業-work)を使う。実行の記録と生成物の場所は `.devkit/run/` に固定し、`init` が `.gitignore` にこの1行を足す。実行の記録を git 管理する選択肢は設けず、ignore された未追跡ファイルなので対象のバージョンにも含まれない。

同じ機械で複数の作業を並行するときは、ホストや git の worktree 機能で別のチェックアウトを開く。それぞれが自分の `.devkit/run/` を持つ。育てるものの読み出しは[規範の固定](guarantees.md#規範の固定)に従う。

## 指示とホスト設定

AGENTS.md は既存のものに kit の案内を1行足すだけとし、CLAUDE.md は生成しない。skill の正本は `.agents/skills/devkit/` とし、`.claude/skills/devkit` を git 管理のシンボリックリンクにする。playbook は skill に統合する。

判定者・計画者は `.claude/agents/devkit-<ロール>.md`・`.codex/agents/devkit-<ロール>.toml` で定義し、`init` が既定の定義を書く。名前には `devkit-` の接頭辞を付け、Codex の組み込み名と重複させない。

hook の登録では `init` が kit のバイナリを登録する。設定の正本と追記時の確認は[権限の境界](guarantees.md#権限の境界)、束縛のないセッションでの扱いは [hook の共通処理](hooks.md#共通処理)、導入後の確認は [doctor](modules.md#導入の照合)に従う。
