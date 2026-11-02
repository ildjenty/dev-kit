# 設計上の決定

実装前の選択は仮決定として番号を付け、保存済みデータや利用側の設定に影響する選択は不可逆な決定として扱う。不採用の案には、採らない理由を残す。ホストの挙動など確認が必要な条件は、未確認の前提として切り分ける。

## 仮決定

1. 言語は Go とする。パッケージの区分は[モジュール](modules.md#モジュールごとの契約)に従う。
2. バイナリは単一の `devkit`、設定ファイルは `devkit.toml` とする。設定は TOML、計画・記録・指示・終了ブロック・結果・決定と証拠は JSON とし、JSON Schema で形式を制約する。
3. セッション内の skill から作業を始める。[作業の開始](flow.md#作業の開始)と [CLI の呼び分け](modules.md#cmddevkit)を採用する。
4. 進行を CLI に集める。[手順の省略と完了の過大申告](guarantees.md#手順の省略と完了の過大申告)、[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。
5. サブエージェントを定型の依頼で起動し、結果を hook 経由で受ける。[判定結果の受理](guarantees.md#判定結果の受理)に従う。別系統のエンジンによる判定は登録コマンドとして表し、`codex exec --output-schema` などを `devkit` が子プロセスとして実行する。
6. 人の回答は観測を根拠にする。[回答の受理](guarantees.md#回答の受理)、[回答・承認](data.md#回答承認-answer--approval)、[UserPromptSubmit](hooks.md#userpromptsubmit)に従う。
7. CLI の実行権限と呼び方は[権限の境界](guarantees.md#権限の境界)、子プロセスは[信頼の前提](guarantees.md#信頼の前提)に従う。
8. 作業場所は[利用側のリポジトリ](layout.md#利用側のリポジトリ)、識別は[セッションとブランチ](data.md#セッションとブランチ)、観測は [SessionStart](hooks.md#sessionstart)に従う。
9. 保存と書き手は[記録の書き込みと排他](data.md#記録の書き込みと排他)、保存場所と ignore は[利用側のリポジトリ](layout.md#利用側のリポジトリ)に従う。
10. スナップショットは[対象のバージョン](data.md#対象のバージョン-targetversion)、読み出しは[規範の固定](guarantees.md#規範の固定)、展開と確定は[判定対象の展開と完了時の永続化](guarantees.md#判定対象の展開と完了時の永続化)に従う。
11. 判定の有効性と規範の識別は [Judgment](data.md#check-の結果-judgment)に従う。
12. 検証はリポジトリの pre-commit に任せ、`devkit` はステージ完了時に本物の `git commit` を行い、その成否を完了条件に使う。判定の指示の前にも `git hook run pre-commit` で同じ hook を実行し、失敗なら判定者を起動しない(git 2.36 以降。使えなければ完了時だけ)。`DEVKIT_STAGE` と `DEVKIT_WORK` を環境変数で渡す。詳細は[完了条件](guarantees.md#完了条件)、判定の登録内容と準備・掃除の責務は [config](modules.md#config)に従う。
13. 承認者、承認の同一性、訂正は[回答・承認](data.md#回答承認-answer--approval)に従う。
14. 外部操作は[承認](guarantees.md#外部操作の承認)と[二重適用の防止](guarantees.md#外部操作の二重適用)に従う。
15. 停止の補助は [Stop](hooks.md#stop)、サブエージェント内の扱いは [hook の共通処理](hooks.md#共通処理)に従う。
16. deny は [PreToolUse](hooks.md#pretooluse)、読み出し元は[規範の固定](guarantees.md#規範の固定)に従う。
17. 束縛の有無による処理は [hook の共通処理](hooks.md#共通処理)、稼働確認は[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)に従う。
18. 再開用の現在地は [checkpoint](data.md#checkpoint)、注入の上限は[性能と出力](guarantees.md#性能と出力の上限)に従う。
19. 指示ファイルと skill、playbook の扱いは[指示とホスト設定](layout.md#指示ファイルとホスト設定)に従う。
20. 固定物は [kit](modules.md#kit)、埋め込みは[ルートパッケージ](modules.md#ルートパッケージ)に従う。
21. 表示は [view](modules.md#view)、閲覧と確認済みの集合は[状態変更](data.md#決定と証拠-decision--evidence)、問いの停止範囲は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。
22. 構造の資産は[構造の記録](data.md#構造の記録-structure)に従う。
23. 観点の検出試験は[実行の条件](guarantees.md#検出試験)と[記録の識別](data.md#検出試験の記録-detectiontest)に従う。
24. 確認事項の既定は[人への確認対象](guarantees.md#人への確認対象)、hook の稼働不足は[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)で扱う。
25. 記録形式の互換性は[ロールアウト](guarantees.md#ロールアウト)に従う。
26. メタループは専用モジュールを持たず、kit の固定物と router の「教訓の反映先」で表す。
27. インストーラによるバイナリの配置を含む配布方法は未決定とする。
28. ホスト設定は[権限の境界](guarantees.md#権限の境界)、導入と既存設定の保持は [install](modules.md#install)に従う。
29. 終了の種別と外部操作との区別は[進捗](data.md#進捗-progress)に従う。
30. 同時実行は[作業の排他制御](data.md#作業の排他制御)に従う。
31. 途中の記録は[決定と証拠](data.md#決定と証拠-decision--evidence)、入力 JSON の場所は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。
32. 指示の同一性と出し直しは[指示](data.md#指示-instruction)に従う。
33. 同じ指示の結果の選択と食い違いは[判定結果の受理](guarantees.md#判定結果の受理)に従う。

## 不可逆な決定

- **記録の形式**: 記録ごとの独立ファイル、JSON、記録 ID の形式、参照の列は保存済みデータに影響する。形式のバージョンと読み取り可能な範囲の宣言方法を定める。
- **計画の JSON Schema**: スキーマにバージョンを持たせる。
- **設定ファイルの名前と形式**、**利用側のディレクトリ名**、**ブランチ名の規則** `devkit/<作業 ID>`: 導入済みリポジトリに影響する。
- **CLI のサブコマンド名と `next`・`submit`・`answer` の入出力の形式、回答の定型**: skill と hook から呼ばれ、人がチャットで使う公開契約である。
- **サブエージェントの定義の名前(`devkit-` 接頭辞)と結果の形式**: ホストの設定と判定者の出力に影響する。
- **外部書き込みと判定の登録形式**: `devkit.toml` の節はリポジトリ側の実装との契約になる。
- **ホストの設定への依存**: [PreToolUse](hooks.md#pretooluse)の規則の構文と起動制限、SubagentStop・UserPromptSubmit の入力項目に依存する。
- **変更可能な内部の決定**: モジュールの内部構造、view の見た目、ledger/fsstore の最適化は、後から変更できる。

## 不採用の案

- **`devkit` が検証コマンドを登録・実行する方式**: git の pre-commit で賄える。検証結果の記録も[コミット](guarantees.md#完了条件)で賄える。
- **ラウンド ID・種別・出し直し回数だけで指示を識別する方式**: ラウンド前の計画や検出試験、同じラウンドの異なる観点を識別できない。採用した識別は[指示](data.md#指示-instruction)に従う。
- **ブランチが一致しない場合も書く操作を許す方式**: 異なるチェックアウトで結果の記録・検証・外部操作・完了が進んでしまう。[セッションとブランチ](data.md#セッションとブランチ)の境界を使う。
- **完了時にブランチの参照だけを進める方式**: index が受理済みのスナップショットと一致せず、pre-commit も実行されない。[完了条件](guarantees.md#完了条件)に従う。
- **未回答の問いがあるときだけ入力を観測する方式、承認後の訂正を期限なく受理する方式**: 回答後の訂正を見落とすか、実行済みの操作・開始済みのステージの承認を後から置き換えてしまう。[UserPromptSubmit](hooks.md#userpromptsubmit)と[回答の受理](guarantees.md#回答の受理)で扱う。
- **commit 直前にツリーが変わっても返却後のスナップショットを取り直さない方式**: 判定・完了・コミットの対象が一致しない。[完了条件](guarantees.md#完了条件)に従う。
- **外側のプロセスが `claude -p` や `codex exec` を起動してループを回す方式(前のバージョン)**: ホストのサブエージェント、セッション再開、compaction、チャットでの確認を子プロセスと結果ファイルで再実装し、他の skill や plugin と同じセッションで使えない。
- **セッションのエージェントが進行を決める方式**: 計画の順序と完了条件をエージェントに委ね、決定的だった部分の性質が損なわれる。
- **判定者が結果ファイルを自分で書く方式**: 読み取り専用の判定者は書き込めず、作業者が同じファイルを書いても区別できない。[SubagentStop](hooks.md#subagentstop)を使う。
- **エージェントが人の答えを転記するだけの `answer`**: 曖昧な答えを都合よく解釈する偏りを防げない。[回答の受理](guarantees.md#回答の受理)に従う。
- **Stop hook を唯一の強制手段とする言い方**: 他の hook の優先、継続回数のリセット、hook 停止の未検知がある。[Stop](hooks.md#stop)と[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)の役割に分ける。
- **`devkit` を sandbox の中で動かす方式**: Codex の workspace-write は `.git` を再帰的に読み取り専用にし、ネットワークも既定で無効なので、スナップショット・refs・コミット・外部操作ができない。[権限の境界](guarantees.md#権限の境界)に従う。
- **規範を返却後のスナップショットから読む方式**: ラウンド中の書き換えがそのラウンドに適用される。[規範の固定](guarantees.md#規範の固定)に従う。
- **SQLite による ledger、Markdown を正本とする計画、メタループ専用モジュール、hook 内でモデルに判定させる check、ホストごとに分けた skill の正本、checkpoint の上書き記録、問い ID だけで同定する承認、担当のツールに任せる外部への書き込み、共有の記録を ledger に置く方式、実行の記録の git 管理**: 前のバージョンから引き続き不採用とする。
- **kit 自身による隔離と能力の検出試験、ステージング経由の記録、基点コミットからの規範の実体化、端末での外部操作の開始確定、状態ディレクトリ全体の flock、構造の current ポインタと履歴の連鎖、実行の記録をチェックアウトの外に置く方式、埋め込み固定物の実体化、上書きするカーソル、環境の記録、ホストの設定を起動時に雛形で上書きする方式、部品ごとの登録ファイル、kit が worktree を作る方式、記録形式の `migrate`、依存物の有効条件・再構築・掃除を kit で管理する方式、`devkit progress`・`record`・`check`・`detect`・`ledger` のサブコマンド**: [信頼の前提](guarantees.md#信頼の前提)と[実装対象外の機能](guarantees.md#実装対象外の機能)により採用しない。
- **Judgment の規範の内容ハッシュ(ツリーに含まれないものの照合)**: 規範はすべて開始時のスナップショットに含まれる。[Judgment の規範の識別](data.md#check-の結果-judgment)を使う。
- **リポジトリ全体の `permissions.deny` に kit の `[deny]` を足す方式**: 束縛のないセッションや他の skill にも効く。[PreToolUse](hooks.md#pretooluse)で適用する。
- **`.claude/skills` 全体をリンクにする方式**: 既存の skill と衝突する。[skill 単位の配置](layout.md#指示ファイルとホスト設定)を使う。
- **サブエージェントの依頼文をエージェントに書かせる方式**: 作業者の説明や自己評価が依頼に混ざる。[判定結果の受理](guarantees.md#判定結果の受理)に従う。
- **観測した入力に定型が含まれるだけで受理する方式**: 引用や説明、サブエージェントの報告を承認として扱えてしまう。[回答の受理](guarantees.md#回答の受理)に従う。
- **ブランチ名だけでセッションを束縛する方式**: 同じチェックアウトの別セッションにも hook が効き、ブランチの切り替えで無効になる。[セッションとブランチ](data.md#セッションとブランチ)に従う。
- **`git archive` による対象の展開**: 属性によって内容が変わる。[判定対象の展開](guarantees.md#判定対象の展開と完了時の永続化)を使う。
- **リポジトリ全体のツリーハッシュを規範の識別にする方式**: 無関係な変更で判定が失効する。[Judgment の規範の識別](data.md#check-の結果-judgment)を使う。

## 未確認の前提

- `git hook run pre-commit` が、`core.hooksPath` や husky・pre-commit フレームワークの構成でも commit 時と同じ hook を実行すること。
- husky・pre-commit フレームワークと `core.hooksPath` の共存。
- Codex で escalation を要さない呼び出しにも rules の allow が適用されること。
- Codex の SubagentStop の `agent_type` の値の形式が、サブエージェント定義の名前と照合できること。
- 両ホストの `last_assistant_message` に必ず結果の本文が入ること。Codex では null になりうる。
- Codex の UserPromptSubmit が、回答後の訂正を含めて入力のたびに発火すること。
- Codex の UserPromptSubmit が、人以外のターンでも発火するか。
- Codex で SessionStart hook から後続のコマンドへ環境変数を渡せること。渡せなければ、Codex では同じチェックアウトで同時に複数のセッションを開かないことを前提とする。
- Codex に履歴を継承するサブエージェントの起動があるか、あるなら禁止できるか。禁止できなければ transcript の照合だけで代える。
- 両ホストの `agent_transcript_path` の形式から、最初の prompt を取り出せること。
- Codex の PreToolUse が、サブエージェントの起動を捕まえること。
- Claude Code で `Agent(effort:*)` が effort の上書きを禁止できること。
- Claude Code の hook の登録に `onFailure: "block"` 相当の設定を使えること。
- settings の Stop hook がサブエージェントの終了で発火するか。発火する場合は `agent_id` で無視する。
- Codex の Stop の継続上限と、Bash の出力上限。
- Claude Code の skill がシンボリックリンクされたディレクトリをたどること。
- `features.shell_tool = false` の Codex の判定者が、展開先のファイルを読めること。読めなければ shell を残し、編集しない指示と `[deny]` で代える。
