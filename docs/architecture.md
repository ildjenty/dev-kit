# dev-kit のアーキテクチャ

dev-kit は、リポジトリごとの開発ループを、Claude Code や Codex CLI のセッション内で回すための道具である。人は普段どおりセッションを開き、kit の skill で課題を渡す。セッションのエージェントが手を動かし、単一バイナリの `devkit` が「次に何をするか」「終わってよいか」「何を記録するか」を決める。進行の判断は `devkit` が担う。

## 全体構成

```mermaid
graph LR
  human[人] -- チャット --> agent[セッションのエージェント]
  agent -- "devkit next / submit / answer" --> devkit[devkit CLI]
  devkit -- 指示と brief --> agent
  devkit -- 記録 --> run[.devkit/run/]
  agent -- 起動 --> sub[サブエージェント<br/>判定者・計画者]
  sub -- 最終メッセージ --> hook[devkit hook]
  human -- 入力 --> hook
  hook -- 観測と結果ファイル --> run
  devkit -- 子プロセス --> cmd[git commit<br/>外部操作]
  host[ホスト] -- イベント --> hook
```

- **セッションのエージェント**は実行役である。brief を読み、コードを書き、終了ブロックを返す。判定者と計画者のサブエージェントを起動するが、依頼文は定型の1行に固定される。
- **`devkit`** は状態機械である。利用者が組んだループの雛形に従って、ループ計画(ステージの列と遷移表)と記録から次の指示を計算し、[ステージの完了時に `git commit` し](architecture/guarantees.md#完了条件)、判定者の結果と人の答えを受理し、記録を書く。エージェントが Bash から呼ぶが、sandbox の外で動く。
- **hook** は同じバイナリの別の呼び出し口である。セッションの開始時に状況を注入し、終了を止め、危険な操作を拒み、サブエージェントの結果と人の入力を観測してファイルに書く。
- **人**はチャットで問いに答える。`devkit` が受理するのは、hook が観測した入力が定型「問い ID 選択肢」に完全に一致するときだけである。

## 1ラウンドの流れ

1. `devkit start --plan <name> <課題>` が作業とブランチ `devkit/<作業 ID>` を作る。計画者が雛形から計画を作り(完了条件を埋め、関係のないステージを外す)、計画の check と人の承認を経て実行可能になる。
2. `devkit next` がステージの brief を返す。このとき規範(観点ファイル、登録コマンド、deny)をスナップショットに固定する。
3. エージェントが作業し、終了ブロックを `devkit submit` で渡す。`devkit` が返却後のスナップショットを取る。
4. ステージに置いたコマンドの check(テスト、型検査など)を `devkit` が実行する。必須のコマンドが `FAIL` なら判定者を起動せずに修正へ回す。詳細は [Plan](architecture/data.md#計画-plan) に従う。
5. `devkit next` が判定の指示を返し、エージェントが判定者を起動する。結果は hook が受け取り、次の `next` が受理する。
6. router が「修正する」「人に確認する」「前のステージに戻る」「完了する」を判断する。修正なら 2 へ、確認なら人の答えを待って `answer` で記録、完了なら `devkit` が `git commit` し、リポジトリの pre-commit がテストと lint を走らせる。失敗なら修正へ、成功なら次のステージへ進む。詳細は[完了条件](architecture/guarantees.md#完了条件)に従う。
7. セッションをまたぐ再開は、ホストのセッション再開と SessionStart の注入で行う。途中で終わっても作業は「作業中」のまま残る。

段階ごとの詳細は [flow.md](architecture/flow.md)。

## 保証の範囲

- **信頼の前提**: 作業者(エージェント)と評価対象のコードに悪意はない。防ぐのは事故(誤った場所への書き込み、元に戻せない操作、二重適用)、自己評価の偏り(自己 `PASS`、完了の過大申告、手順の省略、人の答えの都合のよい解釈)、再開の食い違い、バージョンの不一致である。偽造や資格情報の窃取は範囲外で、チームでは git 管理と PR レビューで見えるようにする。
- **保証**: `devkit` が保証するのは「記録上、正しくない完了・承認・外部操作が成立しない」こと。セッションを終わらせない Stop hook は補助である。
- **実装しないもの**: git と、両ホストにある機能で賄えるものは kit で実装しない。ブランチ・worktree・履歴は git に、sandbox・permissions・hook・skill・サブエージェント・セッション再開・人とのやり取りはホストに任せる。
- 失敗ごとの対策の対応は [guarantees.md](architecture/guarantees.md)。

## 構成要素

- **データ**: 作業、ループの雛形、計画、ステージの実行、ラウンド、対象のバージョン(スナップショット)、Judgment、問いと回答、外部操作、進捗。実行の記録は作業ごとに追記し、書き手は `devkit` だけ。[data.md](architecture/data.md)
- **モジュール**: plan・brief・check・router・form(ドメインの核)、ledger と fsstore(記録)、engine(状態機械)、external、hook、workspace(git)、config、install、kit、view、cmd。依存は一方向で、interface は持たない。[modules.md](architecture/modules.md)
- **配置**: 利用側は `devkit.toml`、`.devkit/`(育てるもの)、`.devkit/run/`(記録。ignore)、ホストの設定、`.agents/skills/`。[layout.md](architecture/layout.md)
- **hook**: SessionStart・Stop・PreToolUse・SubagentStop・UserPromptSubmit の5イベント。[hooks.md](architecture/hooks.md)

## 設計上の決定

- 仮決定、不採用の案、未確認の前提: [decisions.md](architecture/decisions.md)
- 部品の契約への改訂: [contract-changes.md](architecture/contract-changes.md)
