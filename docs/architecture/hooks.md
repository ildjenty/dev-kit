# hook

hook はホストの5つのイベントを受け、セッションの現在地の注入、停止の補助、操作の拒否、判定結果と入力の観測を行う。イベントの処理は決定的にし、モデルに判断させない。hook は観測ファイルを[生成物](data.md#所有と生存期間)(再生成または削除できるファイル)として公開し、CLI がその内容を[実行の記録](data.md#所有と生存期間)(作業ごとに残す記録)として受理する。

## 共通処理

Claude Code と Codex CLI を対象とし、どちらも既定のホストとはしない。ホストは `devkit hook <event>` を呼び、標準入力の JSON を host/event が共通イベントへ変換する。出力も host/event がホストの形式と上限に合わせる。

束縛は[セッションの観測](data.md#セッションの観測-sessionobservation)から取得する。SessionStart の観測と UserPromptSubmit の最新入力の保持は、束縛がなくても行う。それ以外は束縛がなければ早期終了する。束縛がある UserPromptSubmit は、未回答の問いの有無にかかわらず、回答後の訂正も含めて本文を観測する。除外する本文は [UserPromptSubmit](#userpromptsubmit)に従う。これにより、他の skill や plugin と同じリポジトリで使える。

セッションの観測は `.devkit/run/sessions/<session_id>/<時刻>-<イベント>.json` に追記型で公開し、共有ファイルを上書きしない。`<イベント>` には hook のイベント名、または[束縛を書く CLI](data.md#セッションとブランチ)の操作名を使う。読み手は[識別したセッション](data.md#セッションとブランチ)の最新の観測を取る。

サブエージェント内(入力に `agent_id` がある場合)では、Stop と SessionStart は何もしない。両ホストのサブエージェントの PreToolUse には `agent_id` と `agent_type` が入り、[deny](#pretooluse)(操作の禁止規則)を適用する。ブランチの不一致は[セッションとブランチ](data.md#セッションとブランチ)、[ledger](modules.md#ledger)(記録を扱う部品)と生成物の書き分けは[記録の書き込みと排他](data.md#記録の書き込みと排他)に従う。

## SessionStart

- **入力**: 起動・再開・コンテキスト圧縮(compaction)後のセッション ID、ホスト名、cwd などのイベント。
- **判断**: ブランチに関係なく[共通処理](#共通処理)に従い、続行する場合は [checkpoint](data.md#checkpoint)を組み立てる。
- **出力**: checkpoint を注入する。中断の通知は[中断](guarantees.md#中断)に従う。失敗時は警告を注入する(SessionStart は block できない)。本文と要約の切り替えは[性能と出力](guarantees.md#性能と出力の上限)に従う。
- **書く生成物**: [共通処理](#共通処理)で定めるセッションの観測。

## Stop

- **入力**: このセッションに束縛された作業、ledger から導いた中断の有無と未受理の指示の種別、未取り込みの停止の観測、作業ごとの継続回数と照合した記録 ID。
- **判断**: [中断](guarantees.md#中断)による停止の許可と継続上限を先に照合する。上限は `devkit.toml` の `[limits] stop_continuation` で設定し、既定3回、1以上を必須とし、無効にはできない。通常は問い・外部操作の確認・完了・中止・判定待ち・単位の作業者の結果待ちなら停止を許し、それ以外はラウンドの有無ではなく作業単位で `block` する。
- **出力**: `block` 時は理由として「`devkit next` を実行する」を返す。継続上限時の問いの提示は[中断](guarantees.md#中断)に従う。
- **書く生成物**: [停止の観測](data.md#停止の観測-stopobservation)。問いの ledger への取り込みは CLI に委ねる。

Claude Code の継続回数の上限は8回である。ホストが数える継続回数はツール呼び出しでリセットされることが確認済みであり、[kit の継続上限](guarantees.md#中断)とは別である。他の hook の `continue: false` が優先される場合がある。セッションをまたぐ再開は SessionStart の注入、回答待ちからの継続は[人の回答と再開](flow.md#人の回答と再開)に従う。保証との関係は[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)に従う。

Claude Code ではサブエージェントの終了時に Stop は発火せず、SubagentStop だけが発火する。Agent は既定でバックグラウンドで動くため、親のターンが先に終わって Stop が発火し、完了通知が後から届く。Codex では Stop の `block` による継続を12回続けても停止せず、ホストの継続上限は少なくとも12回を超える。

## PreToolUse

- **入力**: ツール名、Bash のコマンド、サブエージェント起動の引数、呼び出し元の `agent_id`・`agent_type`、セッションの束縛。Codex で `tool_input.command` を取得できることは確認済みである。
- **判断**: [中断](guarantees.md#中断)の経路を確保したうえで、[共通処理](#共通処理)を経て続行する場合は、[規範の固定](guarantees.md#規範の固定)に従って `[deny]` を読み、禁止規則と起動の定型を照合する。読み出し関数の受け渡しは [hook の依存](modules.md#hook)に従う。
- **出力**: 許可、または deny と理由。hook の matcher(起動対象を絞り込む設定)にはツール名だけを指定し、禁止規則との一致は hook 内で判定する。
- **書く生成物**: [共通処理](#共通処理)で定めるセッションの観測。

規則は `Bash(<前方一致> *)`、ツール名(MCP ツールを含む)、`Agent(<属性>)` の3種とする。Claude Code の permission rule の意味論は再実装しない。Bash の `&&`・`||`・`;`・`|` による線形な連結は分割し、各部分に前方一致を当てる。Bash は `[deny]` に明示した操作との一致だけで deny する。結果ファイルの保護は[誤った場所への書き込みと危険な操作](guarantees.md#誤った場所への書き込みと危険な操作)に従う。

Claude Code の Agent では `tool_input` の `subagent_type`・`prompt`・`model` を読め、`-p` でも引数に基づく deny が効く。束縛のあるセッションで `subagent_type` が `devkit-` で始まる場合に限り、prompt が[判定結果の受理](guarantees.md#判定結果の受理)の定型でなければ deny する。履歴継承は `[deny]` の既定の `Agent(fork)` で禁止する。

Codex の `spawn_agent` は `tool_name: collaborationspawn_agent` として捕捉でき、`tool_input` には `task_name`・`agent_type`・`fork_turns`・`message` が入る。`fork_turns` は `"all"` と `"none"` で、既定の `"all"` はカスタムの `agent_type` でも親の会話をすべて引き継ぐ。束縛のあるセッションで devkit の判定者・計画者を起動するときは `fork_turns: "none"` を必須とし、それ以外や省略は deny する。`message` は暗号化されて読めないため、起動時の本文は照合しない。

Claude Code では、束縛のあるセッションで `subagent_type` が `devkit-` で始まる起動について、[規範の固定](guarantees.md#規範の固定)で定める担当定義の `model`・[`effort`](data.md#check-の結果-judgment)(推論量の設定)を実際に変える上書きを deny する。定義と同じ値の明示と、隔離指定(`isolation`)の有無だけでは拒否しない。起動条件は担当定義との照合で制限する。両ホストとも、ここで定める起動の制限をリポジトリ全体の `permissions.deny` には足さない。

Codex では、束縛のあるセッションで呼び出し元の `agent_type` が devkit の判定者・計画者である場合、`[deny] judge` に列挙した書き込みを行うツール(`apply_patch`、書き込みのできる MCP ツールなど)を deny する。ファイルの読み取りには exec や node の REPL などを使える。単位の作業者 `devkit-worker` はこの書き込み禁止の対象外であり、判定者・計画者への禁止は維持する。

## SubagentStop

- **入力**: `agent_type`、`agent_id`、`last_assistant_message`、`agent_transcript_path`、取得できる場合はホストから観測した effort または model、発行済みの[指示](data.md#指示-instruction)。
- **判断**: `agent_type` が devkit の作業者・判定者・計画者の定義名であることを確かめ、[最終メッセージの結果](guarantees.md#判定結果の受理)から指示 ID を取る。束縛があり、発行済みのサブエージェントへの指示と指示 ID・担当名が一致すれば採取する。単位の作業、計画の check と検出試験の指示も対象とし、指示 ID・担当名の不一致なら何もしない。最終メッセージを JSON として解析できない場合は失敗の観測を残す。同じ runner の未受理の指示が複数あり、指示 ID のない不正な最終メッセージをどの指示にも結び付けられない場合も観測を残す。
- **出力**: 最終メッセージに来歴を添えて結果ファイルとして公開し、[判定結果の受理](guarantees.md#判定結果の受理)に渡す。`devkit-worker` の最終メッセージは `schemas/progress.json` の終了ブロックである。JSON の解析失敗の観測も公開し、`next` が失敗の記録・結果の拒否・再指示と上限での人への問いを処理する。指示に結び付けられない不正な結果は、観測を残して人に問う。
- **書く生成物**: 結果ファイル `results/<指示 ID>-<agent_id>.json`、または JSON の解析失敗の観測である。結果ファイルには検証済みの指示 ID・担当、SubagentStop の観測時刻を添える。effort・model の来歴は [Judgment](data.md#check-の結果-judgment)に従う。

両ホストで `agent_type` は定義の `name` と一致し、`last_assistant_message` から最終メッセージの本文を取得できる。Claude Code の `agent_transcript_path` は `<session>/subagents/agent-<id>.jsonl` で、最初の user メッセージから起動時の prompt を読める。Codex は rollout の JSONL を指すが、起動時の `message` は平文で取り出せない。

来歴の採取元は Claude Code が SubagentStop 入力の `effort.level`、Codex が hook 入力の `model` である。Claude Code の入力に model はなく、Codex の入力に effort はない。

## UserPromptSubmit

- **入力**: セッション ID と `prompt`。
- **判断**: 束縛の有無にかかわらず、人の入力ごとにセッションの最新入力本文1件を更新する。束縛がある場合は[共通処理](#共通処理)に従って回答・訂正の本文も観測する。ただし Claude Code で本文が `<task-notification>` で始まる完了通知は、最新入力の保持と回答の原文の観測から除く。
- **出力**: `start` が最新入力の原文を作業に結び付け、計画者の brief と計画の承認の問いには原文と `start` の課題の両方を載せる。観測がなければ原文なしと記録する。束縛後の入力は[回答の受理](guarantees.md#回答の受理)の根拠にもする。hook 自身は承認を決めない。
- **書く生成物**: 束縛前を含むセッションごとの最新入力1件と、束縛後の `prompts/<session_id>-<時刻>.txt`。観測した本文はそのまま保存する。保持先は固定の `.devkit/run/` 内とし、セッションの観測の追記と最新入力1件の保持は区別する。

Claude Code ではサブエージェントの完了通知、他のセッションからのメッセージ、定期タスクでも発火し、人が打った入力と区別する項目がない。Codex の対話モードでは、回答後の訂正も含めて人の入力ごとに発火する。Codex ではサブエージェントの結果で発火しない。両ホストとも Stop の `block` による継続では発火しない。
