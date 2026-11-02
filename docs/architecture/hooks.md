# hook

hook はホストの5つのイベントを受け、セッションの現在地の注入、停止の補助、操作の拒否、判定結果と入力の観測を行う。イベントの処理は決定的にし、モデルに判断させない。hook は観測ファイルを[生成物](data.md#所有と生存期間)(再生成または削除できるファイル)として公開し、CLI がその内容を[実行の記録](data.md#所有と生存期間)(作業ごとに残す記録)として受理する。

## 共通処理

Claude Code と Codex CLI を対象とし、どちらも既定のホストとはしない。ホストは `devkit hook <event>` を呼び、標準入力の JSON を host/event が共通イベントへ変換する。出力も host/event がホストの形式と上限に合わせる。

束縛は[セッションの観測](data.md#セッションの観測-sessionobservation)から取得する。束縛がなければ早期終了するが、SessionStart の観測と PreToolUse・UserPromptSubmit の最終観測時刻の更新は先に行う。UserPromptSubmit の本文は束縛があるときだけ、未回答の問いの有無にかかわらず、回答後の訂正も含めて常に観測する。これにより、他の skill や plugin と同じリポジトリで使える。

セッションの観測は `.devkit/run/sessions/<session_id>/<時刻>-<イベント>.json` に追記型で公開し、共有ファイルを上書きしない。`<イベント>` には hook のイベント名、または[束縛を書く CLI](data.md#セッションとブランチ)の操作名を使う。読み手は最新の観測を取る。

サブエージェント内(入力に `agent_id` がある場合)では、Stop と SessionStart は何もしない。PreToolUse の [deny](#pretooluse)(操作の禁止規則)は親と同じく適用する。ブランチの不一致は[セッションとブランチ](data.md#セッションとブランチ)、[ledger](modules.md#ledger)(記録を扱う部品)と生成物の書き分けは[記録の書き込みと排他](data.md#記録の書き込みと排他)に従う。

## SessionStart

- **入力**: 起動・再開・コンテキスト圧縮(compaction)後のセッション ID、ホスト名、cwd などのイベント。
- **判断**: ブランチに関係なく[共通処理](#共通処理)に従い、続行する場合は [checkpoint](data.md#checkpoint)を組み立てる。
- **出力**: checkpoint を注入する。中断の通知は[中断](guarantees.md#中断)に従う。失敗時は警告を注入する(SessionStart は block できない)。本文と要約の切り替えは[性能と出力](guarantees.md#性能と出力の上限)に従う。
- **書く生成物**: [共通処理](#共通処理)で定めるセッションの観測。Claude Code では `CLAUDE_ENV_FILE` に `DEVKIT_SESSION_ID` も書き、後続コマンドに渡す。

## Stop

- **入力**: このセッションに束縛された作業、ledger から導いた未受理の指示の種別、未取り込みの中断と停止の観測、作業ごとの継続回数と照合した記録 ID。
- **判断**: [中断](guarantees.md#中断)による停止の許可と継続上限を先に照合する。通常は問い・外部操作の確認・完了・中止・判定待ちなら停止を許し、それ以外はラウンドの有無ではなく作業単位で `block` する。
- **出力**: `block` 時は理由として「`devkit next` を実行する」を返す。継続上限時の問いの提示は[中断](guarantees.md#中断)に従う。
- **書く生成物**: [停止の観測](data.md#停止の観測-stopobservation)。問いの ledger への取り込みは CLI に委ねる。

Claude Code の継続回数の上限は8回である。ホストが数える継続回数はツール呼び出しでリセットされることが確認済みであり、[kit の継続上限](guarantees.md#中断)とは別である。他の hook の `continue: false` が優先される場合がある。セッションをまたぐ再開は SessionStart の注入、回答待ちからの継続は[人の回答と再開](flow.md#人の回答と再開)に従う。保証との関係は[再開と hook の稼働確認](guarantees.md#再開と-hook-の稼働確認)に従う。

## PreToolUse

- **入力**: ツール名、Bash のコマンド、サブエージェント起動の引数、セッションの束縛。Codex で `tool_input.command` を取得できることは確認済みである。
- **判断**: [中断](guarantees.md#中断)の経路を確保したうえで、[共通処理](#共通処理)を経て続行する場合は、[規範の固定](guarantees.md#規範の固定)に従って固定したスナップショットから `[deny]` を読み、以下の構文と起動の定型を照合する。読み出し関数の受け渡しは [hook の依存](modules.md#hook)に従う。
- **出力**: 許可、または deny と理由。hook の matcher(起動対象を絞り込む設定)にはツール名だけを指定し、禁止規則との一致は hook 内で判定する。
- **書く生成物**: セッションの観測の PreToolUse 最終観測時刻。

規則は `Bash(<前方一致> *)`、ツール名(MCP ツールを含む)、`Agent(<属性>)` の3種とする。Claude Code の permission rule の意味論は再実装しない。Bash の `&&`・`||`・`;`・`|` による線形な連結は分割し、各部分に前方一致を当てる。リダイレクト・置換を含むコマンドと制御構文は分割せず deny し、「分けて実行する」と返す。結果ファイルの保護は[操作の禁止](guarantees.md#操作の禁止とサブエージェントの制限)の `permissions.deny` とこの規則を組み合わせる。

他の skill の手順への影響は[Bash の一律拒否と他の skill の共存](decisions.md#bash-の一律拒否と他の-skill-の共存)に残す。

束縛のあるセッションでサブエージェントを起動するとき(Claude Code の Agent ツール)、`subagent_type` が `devkit-` で始まる場合に限り、prompt が[判定結果の受理](guarantees.md#判定結果の受理)の定型でなければ deny する。同じ適用範囲で fork(`isolation` や fork の指定)、`model`・[`effort`](data.md#check-の結果-judgment)(Claude Code の推論量の設定)の明示も deny する。これらは `[deny]` の既定の `Agent(fork)`・`Agent(model:*)`・`Agent(effort:*)` と定型の照合として適用し、リポジトリ全体の `permissions.deny` には足さない。

## SubagentStop

- **入力**: `agent_type`、`agent_id`、`last_assistant_message`、`agent_transcript_path` とその参照先の transcript、ホストから観測した effort または model、発行済みの[指示](data.md#指示-instruction)。
- **判断**: transcript の最初の利用者メッセージから[起動の定型](guarantees.md#判定結果の受理)に従って指示 ID を取る。束縛があり、発行済みのサブエージェントへの指示と指示 ID・担当名が一致すれば採取する。計画の check と検出試験の指示も対象とし、不一致なら何もしない。Claude Code の `agent_type` が定義の `name` であることは確認済みである。
- **出力**: 最終メッセージに来歴を添えて結果ファイルとして公開し、[判定結果の受理](guarantees.md#判定結果の受理)に渡す。
- **書く生成物**: `results/<指示 ID>-<agent_id>.json`。transcript の最初の利用者メッセージ、Claude Code の `effort.level` または Codex の `model`、その時点の定義ファイルの内容ハッシュ、SubagentStop の観測時刻を添える。

実効条件の採取元は[effort と model の取得元](decisions.md#effort-と-model-の取得元)、定義を編集したラウンドの完了手順は[サブエージェント定義の変更と完了](decisions.md#サブエージェント定義の変更と完了)に残す。

## UserPromptSubmit

- **入力**: セッション ID と `prompt`。
- **判断**: [共通処理](#共通処理)の条件で最終観測時刻を更新し、本文を観測する。中断の定型との照合は[中断](guarantees.md#中断)に従う。
- **出力**: 観測した入力を[回答の受理](guarantees.md#回答の受理)と[中断・再開](guarantees.md#中断)の根拠にする。hook 自身は承認を決めない。
- **書く生成物**: セッションの観測の UserPromptSubmit 最終観測時刻、`prompts/<session_id>-<時刻>.txt`、該当時の[中断の観測](data.md#中断の観測-pauseobservation)。[共通処理](#共通処理)の条件で観測した本文はそのまま保存する。
