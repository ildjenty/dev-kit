# ディレクトリ構成

実装と埋め込み元は dev-kit のリポジトリに置き、利用側には設定などの[育てるもの](data.md#所有と生存期間)(利用側で更新する git 管理の資産)、作業ごとの実行の記録と生成物を置く。利用側の規範の置き場所は `devkit.toml` の `[paths]` で変更でき、実行の記録と生成物は `.devkit/run/` に固定する。

## dev-kit のリポジトリ

```text
dev-kit/
  cmd/devkit/             サブコマンドの定義と各実装の接続
  internal/
    plan/                計画の意味論
    brief/               brief と判定依頼の組み立て
    check/               観点ファイル、判定、集約、適用除外、検出試験の仕様
    router/              分類表と判断
    form/                固定フォーマットのスキーマと描画
    ledger/              記録の型と問い合わせ
      fsstore/           記録ごとに独立したファイルを作る実装。生成物の公開も担う
    engine/              状態機械の進行、指示の計算、判定コマンドと検証コマンドの実行、結果の受理
    external/            外部への書き込みの唯一の実行経路
    hook/                ホストのイベント処理と観測の書き出し
    host/event/          イベント JSON の変換と出力の整形
    workspace/           git の操作
    config/              渡された TOML の解析と検証
    install/             導入・削除と doctor
    kit/                 配布物、スキーマ、分類表とロールの既定値の供給
    view/                HTML の生成
  assets/                配布する固定物の埋め込み元
    embed.go             固定物の埋め込みと公開(package assets)
    plans/               既定の雛形: default、fix、docs
    stages/              ステージ定義
    checks/              観点ファイル
    templates/           brief と form のテンプレート
    router/              分類表の既定
    rules/               規則の既定
    schemas/             JSON Schema
    skills/              skill の正本(devkit の進め方)
    hosts/               ホスト設定、サブエージェント定義、rules の断片
    examples/            検出試験の違反例・適合例・許容例
    principles/          既存の原則
  docs/                  既存の設計書
    parts/               部品の契約
    architecture/        判定と進行の規則
    *.md                 契約から相対リンクでたどる設計書
```

バイナリの配布と利用側への導入は [install](modules.md#install)に従う。配布物はすべて `assets/` に置く。埋め込みの範囲は [assets パッケージ](modules.md#assets-パッケージ)、バイナリからの供給は [kit](modules.md#kit)、リポジトリへの規範のコピーは [install](modules.md#install)に従う。`docs/` とリポジトリ直下の `README.md` は配布しない。

各ディレクトリの配置ガイドは、`doc.go` の先頭コメントまたは README に置く。[`assets/principles/structure.md`](../../assets/principles/structure.md) の原則1に従う。

## 利用側のリポジトリ

規範と作業の成果の置き場所は `[paths]` の既定値である。principles・stages・checks・plans・templates・rules・requirements・structure・detect は設定で変更できる。

```text
<repo>/
  devkit.toml                互換性の識別、配置、承認、git、分類、上限、教訓、雛形、コマンドの設定。git 管理
  .claude/settings.json      hook の登録、excludedCommands、記録・結果への Edit の deny、Bash(devkit *) の allow の提案。init が項目を足す。git 管理
  .claude/agents/devkit-*.md 判定者・計画者のサブエージェント定義。init が書く。git 管理
  .claude/skills/devkit -> ../../.agents/skills/devkit   skill 単位のリンク。git 管理
  .codex/hooks.json          hook の登録。init が項目を足す。git 管理
  .codex/config.toml         init が sandbox の項目を足す。git 管理
  .codex/rules/devkit.rules  devkit を sandbox の外で動かす allow。git 管理
  .codex/agents/devkit-*.toml 判定者・計画者のサブエージェント定義。git 管理
  .agents/skills/devkit/     skill の正本。Codex の置き場所
  .devkit/                   git 管理(run/ を除く)
    principles/              原則。init が初期内容をコピーし、リポジトリが所有
    stages/                  ステージ定義。init が初期内容をコピー
    plans/<name>.json        ループの雛形。init が既定の default・fix・docs をコピー
    checks/<観点ID>.md        観点ファイル。違反例・適合例・許容例は checks/<観点ID>/ の下
    templates/               brief と form のテンプレート
    rules.md                 規則と強制手段
    requirements.md          リポジトリが満たすべき要件の正本
    detect.json              検出試験の合格記録の索引
    structure.json           構造ステージを使うときだけ
    run/                     実行の記録と生成物。.gitignore で除外
      sessions/<session_id>/<時刻>-<イベント>.json   セッションの観測。作業の外にチェックアウトごとに置く
      work/<作業ID>/
        records/<ULID>.json  追記のみの記録
        run.lock
        inbox/               終了ブロック・計画・note の入力 JSON の既定の場所
        prompts/             人の入力の観測(UserPromptSubmit が書く)
        stops/<ULID>.json    作業単位の継続回数と上限到達時の問い(Stop が書く)
        brief/               next が書く brief、判定依頼、差分ファイル
        snap/<対象のバージョン>/   判定対象と固定した規範の展開(read-tree と checkout-index --prefix)
        results/<指示ID>-<agent_id>.json   判定者・計画者のサブエージェントの結果(SubagentStop が書く)
        view.html
```

UserPromptSubmit は束縛の有無によらず、人の入力ごとにセッションの最新入力1件を更新し、束縛後は作業ごとの観測も残す。タスクの完了通知は入力に含めない。`start` はその原文を作業に結び付け、観測がなければ原文なしと記録する。

作業履歴ファイルは指示発行時点の内容で固定した作業の生成物であり、すべての作業の brief と判定依頼がそのパスを参照する。

入力ファイルの場所と内容の保存は[状態遷移と受理条件](data.md#状態遷移と受理条件)、記録 ID の形式は[記録の範囲](data.md#記録の範囲)、停止の観測の保持条件は[停止の観測](data.md#停止の観測-stopobservation)に従う。

配置ガイドは各ディレクトリの README に置き、[brief の入力](modules.md#brief)に使う。

チェックアウトのルートは `git rev-parse --show-toplevel` で取得し、直下の `devkit.toml` を入口にする。作業はセッションの cwd であるチェックアウトで、[Work のブランチ](data.md#作業-work)を使う。実行の記録と生成物の場所は `.devkit/run/` に固定し、`init` が `.gitignore` にこの1行を足す。サブエージェント定義・hook・保護設定・ignore がこの場所を前提とする。実行の記録を git 管理する選択肢は設けず、ignore された未追跡ファイルなので対象のバージョンにも含まれない。

規範の識別には principles・stages・checks・plans・templates・rules・`devkit.toml`・サブエージェント定義・AGENTS.md・CLAUDE.md を含める。requirements・structure・detect・`.devkit/run/` は含めず、これらを変えても判定は失効しない。JSON Schema、分類表とロールの既定値、ホスト設定の断片はバイナリが供給する。規範内のコード書式の参照は論理名であり、brief が論理名と `snap/` 内のパスの対応を渡す。`schemas/…` は出力スキーマの本文として渡す。

`init` は導入するファイルを人が確認したあとコミットする。`--no-commit` を指定した場合は手動でコミットする。HEAD がない場合、または作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合、`start` は理由と直し方を返して拒否する。

同じ機械で複数の作業を並行するときは、ホストや git の worktree 機能で別のチェックアウトを開く。それぞれが自分の `.devkit/run/` を持つ。育てるものの読み出しは[規範の固定](guarantees.md#規範の固定)に従う。

既存の git hook の扱いと実行は[完了条件](guarantees.md#完了条件)に従う。

## 指示ファイルとホスト設定

kit はリポジトリの AGENTS.md と CLAUDE.md に触れない。devkit の動かし方(開始・`next`・`submit`・`pause`・`resume`・問いの見せ方・サブエージェントの起動)はすべて kit の skill に閉じ、人は `/devkit` の skill で始める。指示ファイルはリポジトリ自身のものであり、判定者の規範としては読む。skill の正本は `.agents/skills/devkit/` とし、`.claude/skills/devkit` を git 管理のシンボリックリンクにする。操作手順は skill に集約し、独立した playbook(操作手順書)は設けない。

Claude Code が `.claude/skills/<name>` から `.agents/skills/<name>` へのシンボリックリンクをたどって skill を読み込むことは確認済みである。Codex も skill のシンボリックリンクをたどる。

人がチャットで止めるよう頼んだら `devkit pause`、再開を頼んだら `devkit resume` をエージェントが呼ぶ手順も skill に含め、条件は[中断](guarantees.md#中断)を参照する。

判定者・計画者のサブエージェントは `.claude/agents/devkit-<ロール>.md`・`.codex/agents/devkit-<ロール>.toml` で定義し、`init` が既定の定義を書く。名前には `devkit-` の接頭辞を付け、Codex の組み込み名と重複させない。

hook の登録では `init` が kit のバイナリを登録する。設定の正本と追記時の確認、Codex の導入後の信頼手順、インタプリタの指定は[ホスト設定と実行権限](guarantees.md#ホスト設定と実行権限)、束縛のないセッションでの扱いは [hook の共通処理](hooks.md#共通処理)、導入後の確認は [doctor](modules.md#doctor-の確認項目)に従う。
