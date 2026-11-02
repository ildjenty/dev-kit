# hook

hook はホストの5つのイベントを受け、セッションの現在地の注入、停止の補助、操作の拒否、判定結果と入力の観測を行う。イベントの処理は決定的にし、モデルに判断させない。観測を生成物として公開し、記録への受理は CLI に委ねる。

## 共通処理

Claude Code と Codex CLI を対象とし、どちらも既定のホストとはしない。ホストは `devkit hook <event>` を呼び、標準入力の JSON を host/event が共通イベントへ変換する。出力も host/event がホストの形式と上限に合わせる。

束縛は[セッションの観測](data.md#セッションの観測-sessionobservation)から取得する。束縛がなければ早期終了するが、SessionStart の観測と PreToolUse・UserPromptSubmit の最終観測時刻の更新は行う。入力の本文は束縛があるときだけ観測する。これにより、他の skill や plugin と同じリポジトリで使える。

サブエージェント内(入力に `agent_id` がある場合)では、Stop と SessionStart は何もしない。PreToolUse の deny は親と同じく適用する。ブランチの不一致は[セッションとブランチ](data.md#セッションとブランチ)、ledger と生成物の書き分けは[記録の書き込みと排他](data.md#記録の書き込みと排他)に従う。

## SessionStart

- **入力**: 起動・再開・compaction 後のセッション ID、ホスト名、cwd などのイベント。
- **判断**: ブランチに関係なく観測を作り、束縛があれば [checkpoint](data.md#checkpoint)を組み立てる。
- **出力**: checkpoint を注入する。失敗時は警告を注入する(SessionStart は block できない)。本文と要約の切り替えは[性能と出力](guarantees.md#性能と出力の上限)に従う。
- **書く生成物**: `.devkit/run/sessions/<session_id>.json`。Claude Code では `CLAUDE_ENV_FILE` に `DEVKIT_SESSION_ID` も書き、後続コマンドに渡す。

## Stop

- **入力**: このセッションに束縛された作業、`next` が返す指示、観測内の継続回数。
- **判断**: 問い・完了・中止・判定待ち・検証中なら停止を許す。それ以外はラウンドの有無ではなく作業単位で `block` する。継続上限に達したら、状態を変えず停止を許す。
- **出力**: `block` 時は理由として「`devkit next` を実行する」を返す。検証中の扱いは[作業の排他制御](data.md#作業の排他制御)に従う。
- **書く生成物**: セッションの観測に継続回数を書く。

Claude Code の継続上限は8回で、ツール呼び出しでリセットされることは確認済みである。他の hook の `continue: false` が優先される場合がある。停止後の再開は SessionStart の注入で行い、保証との関係は[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)に従う。

## PreToolUse

- **入力**: ツール名、Bash のコマンド、サブエージェント起動の引数、セッションの束縛。Codex で `tool_input.command` を取得できることは確認済みである。
- **判断**: 先に最終観測時刻を更新する。束縛があれば、[規範の固定](guarantees.md#規範の固定)に従って `devkit.toml` の `[deny]` を読み、以下の構文と起動の定型を照合する。
- **出力**: 許可、または deny と理由。登録はツール名の matcher だけにし、一致の判定は hook が行う。
- **書く生成物**: セッションの観測の PreToolUse 最終観測時刻。

規則は `Bash(<前方一致> *)` とツール名(MCP ツールを含む)の2種だけとし、Claude Code の permission rule の意味論は再実装しない。Bash の `&&`・`||`・`;`・`|` による線形な連結は分割し、各部分に前方一致を当てる。リダイレクト・置換・制御構文は分割せず、規則に触れる可能性があれば deny して「分けて実行する」と返す。

束縛のあるセッションでサブエージェントを起動するとき(Claude Code の Agent ツール)、`subagent_type` が `devkit-` で始まる場合は、prompt が[判定結果の受理](guarantees.md#判定結果の受理)の定型でなければ deny する。fork(`isolation` や fork の指定)、`model`・`effort` の明示も deny する。これらは `[deny]` の既定の `Agent(fork)`・`Agent(model:*)`・`Agent(effort:*)` と定型の照合として適用し、リポジトリ全体の `permissions.deny` には足さない。

## SubagentStop

- **入力**: `agent_type`、`agent_id`、`last_assistant_message`、`agent_transcript_path`、発行済みの判定または計画の指示。
- **判断**: 束縛があり、`agent_type` が指示の担当名と一致すれば採取する。不一致なら何もしない。Claude Code の `agent_type` が定義の `name` であることは確認済みである。
- **出力**: 最終メッセージに来歴を添えて結果ファイルとして公開し、[判定結果の受理](guarantees.md#判定結果の受理)に渡す。
- **書く生成物**: `results/<指示 ID>-<agent_id>.json`。transcript の最初の利用者メッセージ、Claude Code の `effort.level` または Codex の `model`、その時点の定義ファイルの内容ハッシュ、SubagentStop の観測時刻を添える。

## UserPromptSubmit

- **入力**: セッション ID と `prompt`。
- **判断**: 束縛の有無に関係なく最終観測時刻を更新する。束縛があれば、未回答の問いの有無にかかわらず、回答後の訂正も含めて常に本文を観測する。
- **出力**: 観測した入力を[回答の受理](guarantees.md#回答の受理)の根拠にする。hook 自身は承認を決めない。
- **書く生成物**: セッションの観測の UserPromptSubmit 最終観測時刻と、束縛がある場合の `prompts/<session_id>-<時刻>.txt`。本文はそのまま保存する。
