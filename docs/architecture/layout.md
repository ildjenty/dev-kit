# ディレクトリ構成

実装と埋め込み元は dev-kit のリポジトリに置き、利用側には設定などの[育てるもの](data.md#所有と生存期間)(利用側で更新する git 管理の資産)、作業ごとの実行の記録と生成物を置く。利用側の規範の置き場所は `devkit.toml` の `[sources.<名前>]` で変更でき、実行の記録と生成物は `.devkit/run/` に固定する。

## dev-kit のリポジトリ

```text
dev-kit/
  cmd/devkit/             サブコマンドの定義と各実装の接続
  internal/
    plan/                計画の意味論
    brief/               brief と判定依頼の組み立て
    check/               選んだ基準による判定、集約、適用除外
    router/              分類表と判断
    form/                固定フォーマットのスキーマと描画
    ledger/              記録の型と問い合わせ
      fsstore/           記録ごとに独立したファイルを作る実装。生成物の公開も担う
    engine/              状態機械の進行、単位の指示と合流の管理、コマンドの実行、結果の受理
    external/            外部への書き込みの唯一の実行経路
    hook/                ホストのイベント処理と観測の書き出し
    host/event/          イベント JSON の変換と出力の整形
    workspace/           git の操作、単位の worktree、SHA の固定とコミットしないマージ
    config/              渡された TOML の解析と検証
    install/             導入・削除と doctor
    kit/                 配布物、スキーマ、分類表と役割の既定値の供給
    view/                HTML の生成
  assets/                配布する固定物の埋め込み元
    embed.go             固定物の埋め込みと公開(package assets)
    plans/               既定のパターン: default、fix、docs
    stages/              ステージ定義
    templates/           brief と form のテンプレート
    router/              分類表の既定
    rules/               規則の既定
    schemas/             JSON Schema
    skills/              skill の正本(devkit の進め方)
    hosts/               ホスト設定、サブエージェント定義、rules の断片
    principles/          既存の原則
  docs/                  既存の設計書
    parts/               部品の契約
    architecture/        判定と進行の規則
    *.md                 契約から相対リンクでたどる設計書
```

バイナリの配布と利用側への導入は [install](modules.md#install)に従う。配布物はすべて `assets/` に置く。埋め込みの範囲は [assets パッケージ](modules.md#assets-パッケージ)、バイナリからの供給は [kit](modules.md#kit)、リポジトリへの規範のコピーは [install](modules.md#install)に従う。`docs/` とリポジトリ直下の `README.md` は配布しない。

各ディレクトリの配置ガイドは、`doc.go` の先頭コメントまたは README に置く。[`assets/principles/structure.md`](../../assets/principles/structure.md) の原則1に従う。

## 利用側のリポジトリ

規範と成果のソースの既定の置き場所である。接続先と配置は `[sources.<名前>]` で変更できる。既定の規範のソースは principles・stages・plans・rules・git・templates、成果は requirements・structure である。

```text
<repo>/
  devkit.toml                互換性の識別、配置、役割、承認、git、check の一覧、経路と定義の決まり、計画、上限、並列、教訓、コマンドの設定。git 管理
  .claude/settings.json      hook の登録、excludedCommands、記録・結果への Edit の deny、Bash(devkit *) の allow の提案。init が項目を足す。git 管理
  .claude/agents/devkit-*.md 役割の agent のサブエージェント定義。init が既定の3つを書く。git 管理
  .claude/skills/devkit -> ../../.agents/skills/devkit   skill 単位のリンク。git 管理
  .codex/hooks.json          hook の登録。init が項目を足す。git 管理
  .codex/config.toml         init が sandbox の項目を足す。git 管理
  .codex/rules/devkit.rules  devkit を sandbox の外で動かす allow。git 管理
  .codex/agents/devkit-*.toml 役割の agent のサブエージェント定義。git 管理
  .agents/skills/devkit/     skill の正本。Codex の置き場所
  .devkit/                   既定の files ソースと状態。git 管理(run/・worktrees/ を除く)
    principles/              原則と review/test.md などの用途別の判定基準。principles ソース
    stages/                  ステージ定義。init が初期内容をコピー
    plans/<name>.json        ループのパターン。init が既定の default・fix・docs をコピー
    templates/               brief と form のテンプレート
    rules.md                 規則と強制手段
    git.md                   ブランチ・worktree・PR・統合の散文の規則。git ソース
    requirements.md          リポジトリが満たすべき要件の正本
    structure.json           構造の知識。形式は自由であり、所定の JSON のときだけ図を描く
    worktrees/<単位の指示ID>/  単位ごとの detached の作業場所。.gitignore で除外
    run/                     実行の記録と生成物。.gitignore で除外
      sessions/<session_id>/<時刻>-<イベント>.json   セッションの観測。作業の外にチェックアウトごとに置く
      work/<作業ID>/
        records/<ULID>.json  追記のみの記録
        run.lock
        inbox/               終了ブロック・計画・note の入力 JSON の既定の場所
        prompts/             人の入力の観測(UserPromptSubmit が書く)
        stops/<ULID>.json    作業単位の継続回数と上限到達時の問い(Stop が書く)
        brief/               next が書く brief、判定依頼、差分ファイル
        snap/<対象のバージョン>/   git の判定対象と固定した規範・成果の写し
        results/<指示ID>-<agent_id>.json   役割の agent の結果(SubagentStop が書く)
        view.html
```

UserPromptSubmit は束縛された作業の回答と訂正の入力を観測する。会話の写しと束縛前の最新入力は保持しない。メインのエージェントが合意した内容を start の課題へまとめ、長い課題は `--task-file`、途中の指示は `instruct` で渡す。

経緯の索引は指示発行時点の内容で固定する生成物であり、ステージ・一覧の check の `history` に従う範囲を記録から決定的に絞る。採用された成果と今の材料は区別し、本文の成果は `outputs` で選ぶ。既定の範囲と必須の判定材料は[依頼の入力](data.md#依頼の入力)に従う。

入力ファイルの場所と内容の保存は[状態遷移と受理条件](data.md#状態遷移と受理条件)、記録 ID の形式は[記録の範囲](data.md#記録の範囲)、停止の観測の保持条件は[停止の観測](data.md#停止の観測-stopobservation)に従う。

リポジトリ外のソースは読み取り専用であり、固定した本文の写しを `.devkit/run/` の実行の記録の下に置いて、判定者と作業者に読み取り用として渡す。編集用の写しは作らず、devkit は原本に書き込まない([書き込みの流れ](data.md#書き込みの流れ))。

配置ガイドは各ディレクトリの README に置き、[brief の入力](modules.md#brief)に使う。

チェックアウトのルートは `git rev-parse --show-toplevel` で取得し、直下の `devkit.toml` を入口にする。作業はセッションの cwd のチェックアウトの記録で特定する。セッションのまとめ役が git の規則に従って開始前に作業ブランチを用意する。既定の作業者は devkit が作る単位用の worktree で作業する。実行の記録と生成物の場所は `.devkit/run/` に固定し、`init` が `.gitignore` に追加する。サブエージェント定義・hook・保護設定・ignore がこの場所を前提とする。実行の記録を git 管理する選択肢は設けず、ignore された未追跡ファイルなので対象のバージョンにも含まれない。

単位の作業場所 `.devkit/worktrees/<単位の指示 ID>/` も ignore し、記録を守る deny の対象と重ならないよう `.devkit/run/` の外に置く。devkit がラウンド開始時のスナップショットのコミットから detached で作り、物理的な名前には計画者の単位 ID を使わない。中断中は保持する。中止・作業完了時は変更のない worktree だけを削除し、変更の残るものは保持して場所を知らせる。破棄は人が git で行う。

規範の識別には `norm = true` のソースの全文書と、ソースの外の `devkit.toml`(`[roles]` を含む)・サブエージェント定義・AGENTS.md・CLAUDE.md を含める。requirements・structure は成果であり、実行の記録とともに規範の識別には含めない。成果の更新は規範の変更による失効を起こさないが、対象の識別では git のツリーに外部の成果の文書のソース名・ID・内容ハッシュの集合を加える。brief は固定した写しのパスを示す。`schemas/…` はバイナリが出力スキーマの本文として渡す。

`init` は導入するファイルを人が確認したあとコミットする。`--no-commit` を指定した場合は手動でコミットする。HEAD がない場合、または作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合、`start` は理由と直し方を返して拒否する。

同じ機械で複数のループを並行するときは、ホストや git の worktree 機能で別のチェックアウトを開き、新しいセッションを開始する。それぞれが自分の `.devkit/run/` を持つ。作業中のチェックアウトでの2つ目の `start` は拒否する。1ステージ内の分割した作業では、devkit が単位用の worktree を作り、[合流](flow.md#分割した作業と合流)を管理する。育てるものの読み出しは[規範の固定](guarantees.md#規範の固定)に従う。

既存の git hook の扱いと実行は[完了条件](guarantees.md#完了条件)に従う。

## 指示ファイルとホスト設定

kit はリポジトリの AGENTS.md と CLAUDE.md に触れない。devkit の動かし方(開始・`next`・`submit`・`pause`・`resume`・問いの見せ方・サブエージェントの起動)はすべて kit の skill に閉じ、人は `/devkit` の skill で始める。指示ファイルはリポジトリ自身のものであり、判定者の規範としては読む。skill の正本は `.agents/skills/devkit/` とし、`.claude/skills/devkit` を git 管理のシンボリックリンクにする。操作手順は skill に集約し、独立した playbook(操作手順書)は設けない。

Claude Code が `.claude/skills/<name>` から `.agents/skills/<name>` へのシンボリックリンクをたどって skill を読み込むことは確認済みである。Codex も skill のシンボリックリンクをたどる。

人がチャットで止めるよう頼んだら `devkit pause`、再開を頼んだら `devkit resume` をエージェントが呼ぶ手順も skill に含め、条件は[中断](guarantees.md#中断)を参照する。

[役割](data.md#役割-role)の agent は `.claude/agents/<agent>.md`・`.codex/agents/<agent>.toml` で定義する。`init` は既定の役割の agent である `devkit-worker`・`devkit-judge`・`devkit-planner` の定義を書く。リポジトリが役割を足すときは、`devkit.toml` の `[roles.<id>]` とともに両ホストの定義を置く。agent の名前は `devkit-` で始め、Codex の組み込み名と重複させない。hook はこの接頭辞の名前から役割の種類を引いて保護を効かせる。`work` の役割の agent には brief で作業場所の絶対パスを渡し、そこだけを編集させる。`[deny] judge` の書き込み禁止は `judge`・`plan` の役割の agent に適用し、`work` の役割の agent は対象外とする。Claude Code の `judge`・`plan` の役割の agent の定義の `tools` は `[roles] read_only_tools` の範囲に限る。

hook の登録では `init` が kit のバイナリを登録する。設定の正本と追記時の確認、Codex の導入後の信頼手順、インタプリタの指定は[ホスト設定と実行権限](guarantees.md#ホスト設定と実行権限)、束縛のないセッションでの扱いは [hook の共通処理](hooks.md#共通処理)、導入後の確認は [doctor](modules.md#doctor-の確認項目)に従う。
