# 最小の例

雛形・計画・設定は [PlanTemplate と Plan](data.md#ループの雛形-plantemplate)、[config](modules.md#config)、[検出試験](guarantees.md#検出試験)に従う。利用側の資産は `[sources.<名前>]` の既定の場所で示し、設定で変更できる。

## 雛形と計画

判定者の check は `criteria` に規範のソース名と選び方を並べ、`examples` に検出試験の例と仕様の選び方を書く。`aspect` は判定の名前である。原則の用途別文書が必要とする原則も、選び方に明示する。選び方を雛形と変える計画は逸脱として承認の問いに差分を示す。

既定の雛形は次の構成である。

- `default`: purpose → scope → structure → contract → steps → implement → retrospective。候補は research と prototype であり、scope にゲートを置く。implement の必須 check は `command:test`・`test`・`diff` である。
- `fix`: implement → retrospective。候補は research と prototype であり、implement の必須 check は `command:test`・`test`・`diff` である。
- `docs`: implement → retrospective。implement の必須 check は `wording` である。

`default` は、要件定義を先に行い、合意した完了条件で残りの計画を作る形にする。`.devkit/plans/default.json` の `purpose` に置く遷移表は次のとおりである。

```json
{
  "fix": "purpose",
  "ask": "purpose",
  "back": "purpose",
  "done": "replan"
}
```

最初の計画は `purpose` から始め、`completion_conditions` には要件と完了条件の合意に関する条件を文章で書く。

```json
[
  { "id": "R1", "description": "人に確かめた要件が requirements に記録されている" },
  { "id": "R2", "description": "残りの作業の完了条件の案が人と合意できている" }
]
```

最初の計画を人が承認してから、`purpose` で人に確かめながら要件を詰め、既定の `.devkit/requirements.md` に書き、完了条件の案を作る。置き場所は `[sources.requirements]` で変更できる。`purpose` の完了後、`done: "replan"` により既存の再計画の経路へ進み、計画者が合意した完了条件で残りの計画を作り直す。作り直した計画は計画の check を通し、完了条件が変わるため既定では人が承認してから残りのステージを始める。

遷移 `done` は、次のステージ ID、作業の終了を表す `null`、ステージの完了後に残りを再計画する `"replan"` を取る。要件定義を先に行って再計画するかは雛形の組み方で決まり、kit は要件定義を特別扱いしない。`fix` と `docs` は `purpose` の後に再計画する形を使わない。

retrospective は作業者が担当する。必須 check は `default` と `fix` では `command:test`・`test`・`harness`、`docs` では `harness` だけである。判定者の runner は `init` が配るサブエージェント `devkit-judge` とする。`--plan` の省略時は `default_plan` の値(既定 `"default"`)を使う。不要なステージを外す場合は、逸脱として承認の問いに示す。

`init` が用意する `.devkit/plans/fix.json` の例である。`assignee` の `worker` は作業者(セッション自身)を表す。遷移の `fix`・`ask`・`back`・`done` は、修正・人への確認・前のステージへの戻り・完了を表し、`done` の `null` は最後のステージを示す。

```json
{
  "name": "fix",
  "start_stage": "implement",
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true },
        { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"} },
        { "aspect": "diff", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/diff.md"}, {"source": "principles", "select": "engineering.md"}, {"source": "principles", "select": "working.md"}, {"source": "principles", "select": "designing.md"}, {"source": "principles", "select": "writing.md"}, {"source": "principles", "select": "verification.md"}], "examples": {"source": "examples", "select": "diff/spec.json"} }
      ],
      "gate": { "required": false, "reason": "公開契約を保つ修正" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "implement", "ask": "implement", "back": "implement", "done": "retrospective" }
    },
    {
      "id": "retrospective",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true },
        { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"} },
        { "aspect": "harness", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/harness.md"}, {"source": "principles", "select": "harness.md"}, {"source": "principles", "select": "working.md"}], "examples": {"source": "examples", "select": "harness/spec.json"} }
      ],
      "gate": { "required": false, "reason": "作業の問題と再発防止を確認する" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "retrospective", "ask": "retrospective", "back": "implement", "done": null }
    }
  ],
  "optional_stages": [
    {
      "id": "research",
      "assignee": "worker",
      "checks": [],
      "gate": { "required": false, "reason": "調査結果を実装に渡す" },
      "limits": { "max_rounds": 1, "max_no_progress": 1 },
      "transitions": { "fix": "research", "ask": "research", "back": "research", "done": "implement" }
    },
    {
      "id": "prototype",
      "assignee": "worker",
      "checks": [
        { "aspect": "contract", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/contract.md"}, {"source": "principles", "select": "data.md"}, {"source": "principles", "select": "engineering.md"}], "examples": {"source": "examples", "select": "contract/spec.json"} }
      ],
      "gate": { "required": false, "reason": "未確認の前提を試作で確かめる" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "prototype", "ask": "prototype", "back": "prototype", "done": "implement" }
    }
  ],
  "plan_limits": { "max_rounds": 3 }
}
```

課題「保存ボタンの文言の修正」から作る計画の例である。候補の research と prototype は使わず、implement と retrospective を採用する。`template.content_hash` は、雛形の JSON に末尾の改行を含めた SHA-256 である。完了条件ごとの確認手段は[計画](data.md#計画-plan)に従う。

```json
{
  "version": 1,
  "template": { "name": "fix", "content_hash": "5b27c4145427857b54001b77938d5654c9775829b70f747f7abdba9f324e41d0" },
  "task": "保存ボタンの文言を「登録」から「保存」に変える",
  "completion_conditions": [
    { "id": "C1", "description": "保存ボタンに「保存」と表示され、その表示を確かめるテストが登録済みのコマンドから実行されて通る" }
  ],
  "start_stage": "implement",
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true, "completion_condition_ids": ["C1"], "target": "返却後のツリー" },
        { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"}, "completion_condition_ids": ["C1"], "target": "保存ボタンの表示と、その表示を確かめるテスト" },
        { "aspect": "diff", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/diff.md"}, {"source": "principles", "select": "engineering.md"}, {"source": "principles", "select": "working.md"}, {"source": "principles", "select": "designing.md"}, {"source": "principles", "select": "writing.md"}, {"source": "principles", "select": "verification.md"}], "examples": {"source": "examples", "select": "diff/spec.json"}, "completion_condition_ids": ["C1"], "target": "保存ボタンの表示と、その表示を確かめるテスト" }
      ],
      "gate": { "required": false, "reason": "公開契約を保つ修正" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "implement", "ask": "implement", "back": "implement", "done": "retrospective" }
    },
    {
      "id": "retrospective",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true, "completion_condition_ids": ["C1"], "target": "返却後のツリー" },
        { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"}, "completion_condition_ids": ["C1"], "target": "最終のツリーのテストと登録済みコマンド" },
        { "aspect": "harness", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/harness.md"}, {"source": "principles", "select": "harness.md"}, {"source": "principles", "select": "working.md"}], "examples": {"source": "examples", "select": "harness/spec.json"}, "completion_condition_ids": ["C1"], "target": "その作業の判定と指摘、要件への反映、規範の不足と再発防止" }
      ],
      "gate": { "required": false, "reason": "作業の問題と再発防止を確認する" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "retrospective", "ask": "retrospective", "back": "implement", "done": null }
    }
  ],
  "external_operations": [],
  "open_questions": []
}
```

`open_questions` は任意の文字列配列であり、計画者が決められなかった要求を入れる。人が承認した完了条件が作業の完了基準となる。既定の `[approval] plan = "requirements"` では最初の計画を確認する。同じ完了条件で逸脱も外部操作も未決事項もない再計画には再確認を求めない。未決事項への回答は、計画の却下と作り直しの指示として計画者に渡す。

`open_questions` が空でなければ、承認方針によらず人の承認を必須とする。

implement の終了ブロックの例である。`instruction_id` は作業の指示で受け取った ID とする。提出は[作業結果の提出](flow.md#作業結果の提出)、受理は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。

```json
{
  "instruction_id": "<作業指示 ID>",
  "termination_kind": "completion-claimed",
  "work_performed": [
    "保存ボタンの文言を「登録」から「保存」に変更し、表示を確かめるテストを追加した"
  ],
  "confirmed": [
    "保存ボタンに「保存」と表示され、登録済みのコマンドから表示のテストが実行されて通る"
  ],
  "unconfirmed": [],
  "obstacles": [],
  "temporary_state": [],
  "resume_conditions": []
}
```

retrospective は、その作業の判定と指摘、人の回答や教訓を確認して再発防止を反映し、purpose で反映していない要件を `.devkit/requirements.md` に残す。要件と構造の変更はステージの判定で確認し、規範を変更した場合はループ自身の変更として人に確認する。

### 分割の雛形と計画

ステージの `inputs` の例は `[{"source": "requirements", "select": ""}, {"source": "rules", "select": ""}]` である。雛形は計画者・作業者・判定者それぞれの入力の選び方を持ち、既定で要件と規則を渡す。ステージ定義は `stages` ソースから選ぶ。

リポジトリが分割を選ぶ雛形 `.devkit/plans/split.json` の `stages` の抜粋である。置き場所は `[sources.plans]` で変更できる。steps の完了後に残りを再計画し、implement の作業を単位に分ける。既定の雛形は分割を使わない。

```json
[
  {
    "id": "steps",
    "assignee": "worker",
    "checks": [
      { "aspect": "steps", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/steps.md"}], "examples": {"source": "examples", "select": "steps/spec.json"} }
    ],
    "gate": { "required": false, "reason": "手順ごとの目的と範囲を確かめる" },
    "limits": { "max_rounds": 3, "max_no_progress": 2 },
    "transitions": { "fix": "steps", "ask": "steps", "back": "steps", "done": "replan" }
  },
  {
    "id": "implement",
    "assignee": "worker",
    "split": true,
    "checks": [
      { "aspect": "command:test", "runner": "command:test", "required": true },
      { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"} },
      { "aspect": "diff", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/diff.md"}, {"source": "principles", "select": "engineering.md"}, {"source": "principles", "select": "working.md"}, {"source": "principles", "select": "designing.md"}, {"source": "principles", "select": "writing.md"}, {"source": "principles", "select": "verification.md"}], "examples": {"source": "examples", "select": "diff/spec.json"} }
    ],
    "gate": { "required": false, "reason": "合流した成果をステージ全体で判定する" },
    "limits": { "max_rounds": 3, "max_no_progress": 2 },
    "transitions": { "fix": "implement", "ask": "implement", "back": "steps", "done": "retrospective" }
  }
]
```

steps で「保存処理とそのテスト」「利用手順」の2つの手順を決めた場合の、再計画した計画の implement の抜粋である。完了条件 `C1` は、保存処理が契約を満たし、利用手順が記述されていることを人が承認した条件とする。`completion_conditions` はその条件を引き継ぎ、残りのステージには implement と retrospective を置く。完了済みの steps を含めないことは逸脱に数えない。分割の追加・削除は逸脱であり、単位 ID の重複は検証で拒否する。範囲の重なりは計画の観点の判定者が指摘する。

```json
{
  "start_stage": "implement",
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "split": true,
      "units": [
        {
          "id": "save",
          "description": "保存処理と完了条件を確かめるテストを実装する",
          "scope": "internal/save/ と、その保存処理のテスト"
        },
        {
          "id": "usage",
          "description": "保存の利用手順を記述する",
          "scope": "docs/save.md"
        }
      ],
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true, "completion_condition_ids": ["C1"], "target": "合流後のツリー" },
        { "aspect": "test", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/test.md"}, {"source": "principles", "select": "testing.md"}], "examples": {"source": "examples", "select": "test/spec.json"}, "completion_condition_ids": ["C1"], "target": "保存処理のテストと登録済みコマンド" },
        { "aspect": "diff", "runner": "devkit-judge", "required": true, "criteria": [{"source": "principles", "select": "review/diff.md"}, {"source": "principles", "select": "engineering.md"}, {"source": "principles", "select": "working.md"}, {"source": "principles", "select": "designing.md"}, {"source": "principles", "select": "writing.md"}, {"source": "principles", "select": "verification.md"}], "examples": {"source": "examples", "select": "diff/spec.json"}, "completion_condition_ids": ["C1"], "target": "単位の変更とマージ結果を含む累積差分" }
      ],
      "gate": { "required": false, "reason": "合流した成果をステージ全体で判定する" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "implement", "ask": "implement", "back": "steps", "done": "retrospective" }
    }
  ]
}
```

`next` が返す各単位の指示 ID で、セッションが `devkit-worker` を1行の `devkit-instruction <指示 ID>` で起動する。作業場所は `.devkit/worktrees/<単位の指示 ID>/` とし、計画内の `save`・`usage` はディレクトリ名に使わない。brief に絶対パスを載せ、そこでだけ編集させる。単位がなければ通常のステージとして進める。

すべての単位の終了ブロックを受理したら、devkit が各ツリーをコミットオブジェクトにして SHA で固定し、作業ブランチへ順に、コミットしないマージで合流する。ぶつかった場合はセッションの作業者への修正ラウンドにする。合流後のツリーと、単位の終了ブロックをまとめた作業者の終了ブロックを通常のステージのラウンドとして判定し、単位ごとの判定はしない。合流の記録と再開、中断・中止・作業完了時の worktree の扱いは[分割した作業と合流](flow.md#分割した作業と合流)に従う。

## 設定

`devkit.toml` の例である。`devkit-judge` はサブエージェント定義を使うため、判定コマンドの登録は不要である。検証コマンド `test` の実行ファイルは利用側で用意する。

```toml
kit_version = "v2"
default_plan = "default"

[sources.principles]
adapter = "files"
path = ".devkit/principles"
norm = true
writable = true

[sources.stages]
adapter = "files"
path = ".devkit/stages"
norm = true
writable = true

[sources.plans]
adapter = "files"
path = ".devkit/plans"
norm = true
writable = true

[sources.rules]
adapter = "files"
path = ".devkit/rules.md"
norm = true
writable = true

[sources.examples]
adapter = "files"
path = ".devkit/examples"
norm = true
writable = true

[sources.templates]
adapter = "files"
path = ".devkit/templates"
norm = true
writable = true

[sources.requirements]
adapter = "files"
path = ".devkit/requirements.md"
norm = false
writable = true

[sources.structure]
adapter = "files"
path = ".devkit/structure.json"
norm = false
writable = true

[state]
detect = ".devkit/detect.json"

[approval]
plan = "requirements"
meta_loop_edit = "confirm"

[plan_check]
aspect = "plan"
runner = "devkit-judge"
criteria = [{ source = "principles", select = "review/plan.md" }, { source = "rules", select = "" }]
examples = { source = "examples", select = "plan/spec.json" }

[git]
branch = "devkit/{work_id}"
commit_message = "devkit: {stage} ({work_id})"
allow_empty_commit = true

[classification]
dependency_files = ["go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Cargo.toml", "Cargo.lock", "Gemfile", "Gemfile.lock", "requirements.txt", "pyproject.toml", "poetry.lock", "uv.lock", "composer.json", "composer.lock"]

[limits]
stop_continuation = 3

[parallel]
max_workers = 3

[detection]
required = true

[lessons]
destinations = ["hook_lint_type", "plan_template", "aspect", "principle", "skill"]

[commands.test]
executable = "./scripts/test"
args = []
timeout_seconds = 120

[deny]
rules = ["Bash(git push *)", "Agent(fork)"]
```

`kit_version` は記録形式とバイナリの互換性を識別する値であり、作業にも記録する。ソースごとの接続先と配置は変更できる。規範のソースは全体を列挙して固定し、成果は選んだ文書を固定する。`writable = true` は書き込み能力を表し、読み取り専用のソースへの改善は教訓として管理者へ引き継ぐ。`[state] detect` はソースとは別の合格の索引である。実行の記録と生成物の `.devkit/run/` は固定である。`[detection] required` は既定 `true` であり、`false` なら検出試験を指示せず、合格記録なしで判定を受理する。`[lessons] destinations` は、機械の検査 → 雛形 → 用途別の判定基準 → 原則 → skill の優先順を表す。

`[parallel] max_workers` は同時に動かす単位の作業者数の上限であり、既定は3、1以上を必須とする。1でも単位を1つずつ、同じ worktree と合流の手順で進める。

`[router] human_confirmation` は `dependency_changed` と `target_changed_again` だけを変更でき、未知の分類名は設定エラーになる。check の結果から次の行動への対応とロールは固定である。ループ自身の変更の確認は `[approval] meta_loop_edit` だけに従う。`[plan_check]` は計画の check の名前、runner、`criteria`・`examples` の選び方を指定し、登録済みの判定コマンドなら devkit が実行する。`[classification] dependency_files` は依存の宣言として扱うファイルの glob である。`[deny]` の構文は [PreToolUse](hooks.md#pretooluse)、結果ファイルの保護は[操作の禁止](guarantees.md#操作の禁止とサブエージェントの制限)に従う。

検証コマンドの期限は登録ごとに必須である。ラウンド開始時に `test` が未登録なら、`command:test` は `FAIL` となり、検証の手段を作って `[commands.test]` に登録するよう `auto-fixable` の指摘を返す。登録はループ自身の変更として `[approval] meta_loop_edit` に従い確認し、次のラウンドのスナップショットから使う。

`devkit init` は人が導入内容を確認したあと、導入したファイルをコミットする。`devkit init --no-commit` で省略した場合は手動でコミットする。HEAD がない場合、または作業ツリーに存在する設定済みの規範・`devkit.toml`・サブエージェント定義が未コミットまたは変更されている場合は `start` を拒否し、理由と直し方を返す。

## 観点と検出試験

原則のソースの用途別文書 `.devkit/principles/review/wording.md` は、`principles/writing.md` の原則2・3に照らし、想定読者の前提と表現の具体性を確認する。種別や期待判定を伏せても判定できるように、各例の本文に要求、完了条件、検証方法、対象を含める。違反・適合・許容の構成は `assets/examples/wording/` と揃える。

```text
.devkit/principles/
  writing.md
  review/wording.md
.devkit/examples/wording/
  spec.json
  case-a.md
  case-b.md
  case-c.md
```

`examples` ソースの `wording/spec.json` は例の文書の参照と、devkit だけが読む期待判定の対応を持つ。check の `examples` でこの仕様と例を選ぶ。

```json
{
  "aspect": "wording",
  "cases": [
    { "kind": "violation", "file": "case-a.md", "expected": "FAIL" },
    { "kind": "conforming", "file": "case-b.md", "expected": "PASS" },
    { "kind": "allowed", "file": "case-c.md", "expected": "PASS" }
  ]
}
```

`case-a.md` の本文である。

```markdown
## 要求

会話を読んでいない担当者が、保存済みの日報を削除する手順を読んで実行する。

## 完了条件

担当者が、この手順だけで削除対象、操作する画面、操作の順序、削除の確認方法を特定できる。

## 検証方法

対象の手順を読み、完了条件と principles/writing.md の原則2・3に照合する。

## 対象

# 日報の削除手順

1. あの画面であれを削除する。
2. いつもの場所を確認する。
```

`case-b.md` の本文である。

```markdown
## 要求

会話を読んでいない担当者が、保存済みの日報を削除する手順を読んで実行する。

## 完了条件

担当者が、この手順だけで削除対象、操作する画面、操作の順序、削除の確認方法を特定できる。

## 検証方法

対象の手順を読み、完了条件と principles/writing.md の原則2・3に照合する。

## 対象

# 日報の削除手順

1. 日報一覧で、削除する日報のタイトルを選ぶ。
2. 日報詳細で「削除」を押す。
3. 確認画面のタイトルが削除する日報と一致したら「削除を確定」を押す。
4. 日報一覧に対象のタイトルがないことを確認する。
```

`case-c.md` の本文である。

```markdown
## 要求

会話を読んでいない担当者が、保存済みの日報を削除する手順を読んで実行する。

## 完了条件

担当者が、この手順だけで削除対象、操作する画面、操作の順序、削除の確認方法を特定できる。

## 検証方法

対象の手順を読み、完了条件と principles/writing.md の原則2・3に照合する。

## 対象

# 日報の削除手順

1. 日報一覧で、削除する日報のタイトルを選ぶ。
2. 日報詳細で「削除」を押す。
3. 確認画面のタイトルが削除する日報と一致したら「削除を確定」を押す。
4. 日報一覧に対象のタイトルがないことを確認する。

元の問い合わせ記録: 「あれを削除したい」。出典は削除要求 R1 の発言であり、「あれ」は手順の対象の日報を指す。発言と照合するため引用内の原語を保持する。principles/writing.md の原則2の適用除外は、この引用だけに適用する。
```

各例は要求・完了条件・検証方法・対象を含む匿名の本文として、`criteria` で選んだ基準の文書、出力の約束とともに渡す。判定者には種別・期待判定・`spec.json`・対応表と、それらを含む履歴や参照先を渡さない。devkit が判定と期待を照合する。合格の識別には担当の定義、判定規則を含む判定依頼テンプレート、実際に渡した基準の文書の集合と内容ハッシュ、例の文書の内容ハッシュを含める。必要な別の文書は選び方に明示し、文中の参照は解析しない。既存の違反例・適合例・許容例の期待判定を保つことを確かめる。設定と合格の扱いは[検出試験](guarantees.md#検出試験)に従う。

## pre-commit

素の git hook を使うリポジトリの `.git/hooks/pre-commit` の例である。実行可能にし、`scripts/test` と `scripts/lint` は利用側で用意する。

```sh
#!/bin/sh
set -eu
./scripts/test
./scripts/lint
```

既存の hook の扱い、実行環境、失敗の扱いは[完了条件](guarantees.md#完了条件)に従う。

ループ自身の変更として検出する pre-commit の設定は、git が追跡するものだけである。`core.hooksPath` で指定された場所もリポジトリ内の追跡対象に限る。追跡されない `.git/hooks/pre-commit` の変更は検出対象外である。
