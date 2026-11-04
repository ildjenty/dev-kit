# ループ計画

利用者はループの参考となる[パターン](architecture/data.md#ループの雛形-plantemplate)、ステージ定義、役割、判定の文書と、絶対に守らせる決まりを用意する。計画者が課題に合わせて毎回[計画](architecture/data.md#計画-plan)を組み、人が承認する。パターンは計画を固定しない。devkit は設定した決まりと受理条件を機械で確かめ、承認された計画を実行する。

パターンは既定 `.devkit/plans/<name>.json` に置き、配置は設定で変更できる。計画は固定の `.devkit/run/` に保存して git 管理しない。後の作業にも効かせる決定は要件・構造・規則に残す。

## フォーマット

パターンと計画の項目は[パターン](architecture/data.md#ループの雛形-plantemplate)と[計画](architecture/data.md#計画-plan)、検証の責務は [plan](architecture/modules.md#plan) に従う。記録形式の `kit_version` は `v3` である。

計画のステージは実体の `id` と使う定義の `definition` を持つ。`definition` の省略時は `id` と同じ定義を使う。同じ定義を何度も使え、例えば `implement-1 → verification-1 → implement-2 → verification-2` と増分を組める。実体 ID は計画内で一意とし、新しい仕事には新しい ID を使う。この作業で走った実体の ID を後の計画に書くのは、その仕事を走らせ直す場合だけである。

遷移、`start_stage`、戻り先、`StoppedStage`、修正の担当の規則、単位の所属は実体 ID を使う。`outputs` は実体 ID を選び、定義 ID を書いた場合はその定義の実体をすべて選ぶ。決まりは定義 ID を使う。経緯と集計は実体ごとであり、振り返りには定義ごとの集計も渡す。コミットメッセージの `{stage}` は実体 ID である。

check の一覧は `devkit.toml` の `[checks.<観点>]` に置き、`runner`・`criteria`・`schemas`・`history`・`brief_by_planner` を定める。計画とパターンの check に書くのは、観点の名前、必須か、対象、完了条件 ID、計画者の指示だけである。パターンは課題固有の `instructions` を持たない。判定者の基準は規範のソースから選ぶ。

計画者と計画の判定者の入力の選び方は `[planning] inputs` と `[plan_check] inputs` に置く。既定は要件・規則・構造である。作業者への入力はステージの `inputs` に置く。`report`・`unit_report`・`human_prompt`・`delegations`・`history`・`outputs` は[依頼の入力](architecture/data.md#依頼の入力)に従う。本文の参照を解析して必要な文書を補わない。

ステージの担当 `assignee` は `work` または `human` の役割 ID である。check の runner は一覧に定める `judge` の役割、登録済み判定コマンド、`command:<name>` のいずれかである。`fix` には同じ実体の次のラウンドの担当を変える[規則](architecture/data.md#修正の担当の規則)を書ける。`fix` と `ask` の行き先は自分の実体だけである。

`done` は次の実体 ID、終了を表す `null`、完了後に残りを作り直す `"replan"` を取る。開始から `done` で到達できないステージと循環を拒否する。戻り先は現在の計画の実体と、この作業で走った実体に限る。戻り後の計画には戻り先の ID を含める。成果に依存する仕事は別のステージにし、単位に `after` は設けない。

`split: true` のステージには計画者が `units` を書ける。単位は計画内で一意の ID、説明、担当範囲、任意の `role`・`instructions`・`outputs` を持つ。役割の既定は agent のある `work` の役割 `worker` である。範囲の重なりは計画の判定者が判断する。

### 絶対に守らせる決まり

`[[loop.stage]]` は `definition` に1つの定義、定義の一覧、または `"*"` を指定する。対象の各実体に、必須の check の名前、ゲート、担当してよい役割、`max_rounds`・`max_no_progress` の天井、固定する `report`・`history`・`outputs` を定める。check の内容は一覧から解決する。

`[[loop.path]]` は `require`、`definition` と `follows` の組、`first`、`last` のいずれかを持つ。`require` は必ず含める定義、`follows` はある定義の実体の後に別の定義の実体を置くこと、`first` と `last` は先頭と最後の定義を表す。

決まりを確かめる対象は、採用済みの完了した実体を順に並べ、その後に計画の開始から `done` で到達する並びを足した作業の経路である。`follows` は次の `replan` または終わりまでの同じ区切りで満たす。`require` と `last` は `null` で終わる区切りだけで確かめる。`first` は採用済みまたは計画上の経路の先頭に適用する。計画の受理時と、実行可能な計画を選ぶステージ開始時に、固定した規範で確かめる。違反は理由付きで拒否して計画者へ戻す。

計画の受理時には指示の参照コミットで、すべての `definition`、基準・報告・単位の報告・人への問い・入力の選び方が1件以上の文書に解決することを確かめる。参考パターンの存在と内容ハッシュも同じ参照で照合する。

`[limits] max_rounds_ceiling = 3` と `max_no_progress_ceiling = 2` はステージ上限の天井であり、超える計画を拒否する。`[[loop.stage]]` で対象の天井も定められる。計画の判定のやり直しは `[planning] max_rounds = 3`、作業全体の累計は `[limits] max_total_rounds = 40`、Stop の継続は `[limits] stop_continuation = 3` が既定である。上限は無効にできない。累計上限で続行を承認した場合は、その時点の累計に設定値を加えた値で次に止める。

<a id="既定の雛形"></a>

## 既定のパターン

- `default`: `purpose → scope → structure → contract → steps → implement → verification → retrospective` が参考の並びである。purpose の `done` は `"replan"` とし、要件と完了条件を合意してから残りを計画する。scope にゲートを置く。
- `fix`: `implement → verification → retrospective` である。
- `docs`: 文書編集の定義 `edit` を使う `edit → retrospective` である。edit の check は wording であり、verification を置かない。

research・prototype と、PR 本文を書く handoff は候補の例である。`optional_stages` は候補の例を示す参考であり、計画者が使える定義を限定しない。handoff は `templates/pr.md.tmpl` に沿って本文を書き、wording などで判定する。git 操作は含めない。

既定の決まりは implement に `command:test`・test・diff、verification に `command:e2e`・behavior、retrospective に harness を必須とする。implement の後に同じ区切りの verification を置き、最後の定義を retrospective とする。purpose・scope のゲートはパターンと人の承認で決める。

default と fix のパターンは retrospective に `command:test`・test・harness を置く。docs は harness だけである。作業者は worker、判定者は一覧の judge が既定である。retrospective と一覧の harness は `history = "all"`、`brief_by_planner` は既定では使わない。diff の基準には `principles/security.md` を含める。

`command:test` または `command:e2e` が未登録なら check は修正可能な `FAIL` になる。作業者が検証手段を作って `[commands.<name>]` に登録し、ループ自身の変更の確認を経て次のラウンドから使う。期限は登録ごとに必須である。

## 例

[パターンと計画](architecture/examples.md#雛形と計画)、[設定](architecture/examples.md#設定)、[分割のパターンと計画](architecture/examples.md#分割の雛形と計画)に実際の形式を示す。

## 作成ルール

<a id="雛形を組む規則"></a>

### パターンを組む規則

パターンは機能追加・修正・文書編集などの参考として組む。何を変えるかに応じて、目的・スコープ・構造・契約・手順・実装の候補を選ぶ。元に戻せないものの判断は [`assets/principles/designing.md`](../assets/principles/designing.md) に従う。

どのステージにも、変えうるものを確かめる check を置く。画面・データ・外部通信を変える場合は動作の確認を置く。ゲートには人が判断する理由を記す。機械で必ず守らせる項目は、参考パターンとは別に `[[loop.stage]]`・`[[loop.path]]` で定める。

### 計画を作る規則

`devkit start --plan <name> <課題>` の `--plan` は優先するパターンであり、省略時は `default_plan`(既定 `"default"`)を優先する。計画者はどのパターンを参考にしてもよい。計画の `pattern` に参考にした名前と内容ハッシュを持つ。

計画者には優先するパターンの本文、ほかのパターンの固定したパス、`stages/README.md` の定義 ID・名前と各定義の固定した参照、役割の一覧、check の観点名と1行の説明、`plans/README.md` の組み方の方針を渡す。`[planning] patterns` は設けない。

メインのエージェントは、壁打ちで決まったことを `start` の課題にまとめる。長い課題は `devkit start --task-file <ファイル>` で本文を渡す。課題引数と `--task-file` は併用しない。途中の指示は `devkit instruct <ファイル>` で記録し、次の計画者・作業者へ渡す。判定者には渡さない。会話の写しは取らない。指示は計画や承認済みの完了条件を直接変えず、変更が必要なら再計画と承認を通す。

完了条件は ID 付きの文章で記す。条件ごとに check または人のゲートを置き、妥当性と漏れは計画の判定者と人が判断する。曖昧な要求は推測せず `open_questions` に記す。変えるものだけを扱う最小の計画に保つ。

## 計画の check と人によるレビュー

計画の check は `[plan_check]` に定め、既定の観点は plan、runner は judge である。計画者と計画の判定者の agent は分ける。

既定の `[approval] plan = "requirements"` は最初の計画、直前に人が承認した完了条件の ID または文面の変更、外部操作、未決事項を確認する。同じ完了条件で外部操作も未決事項もない再計画には再確認を求めない。`"always"` はすべてのバージョン、`"deviation"` は前に承認した計画の残りとの構造の違いも確認する。最初の計画・外部操作・未決事項はいずれの方針でも確認する。

承認の問いには課題、ID 付き完了条件、各実体の定義・担当・ゲート・必須 check・指示を順に示し、前に承認した計画の残りとの差分を表示する。最初の計画は参考パターンと比較する。パターンとの違いは表示する判断材料であり、承認の理由にしない。元に戻せないもの、外部操作、未決事項も示す。人が承認した完了条件を完了基準とする。

`open_questions` が残った計画の承認は devkit が拒否する。未決事項への回答は、人が推奨案に同意した場合も、セッションのエージェントが `devkit answer --interpretation rejected` と回答本文で記録し、計画の却下と作り直しの指示として渡す。計画者は回答を完了条件へ反映し、未決事項を解消した計画を再提案する。人による再提案には回数の上限を置かず、自動で繰り返す計画の判定には `[planning] max_rounds` を適用する。

## ステージ実行時の扱い

既定の worker は devkit-worker のサブエージェントであり、自分の worktree で作業する。セッションは1行の指示 ID で起動し、結果を待ち、人へ問いを提示し、git の規則に従って作業完了後の統合を担う。

分割したステージの競合はステージの作業担当に戻す。agent があれば、競合中のマージ状態と競合マーカーを持つ修正用の worktree で、1つの修正単位としてサブエージェントが直す。修正結果を合流して残りの SHA の取り込みを続ける。agent のない work の役割ならセッションのチェックアウトで直す。合流後の累積差分と単位ごとの報告を、ステージ全体の check で判定する。

規範・ホスト設定・skill・追跡対象の有効な pre-commit 設定の変更はループ自身の変更である。既定では効力を持つ前、最後のラウンドなら完了時のコミット前に人が確認する。要件と構造は成果としてステージの判定で確かめる。
