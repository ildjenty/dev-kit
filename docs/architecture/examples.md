# 最小の例

雛形・計画・設定は [PlanTemplate と Plan](data.md#ループの雛形-plantemplate)、[config](modules.md#config)、[検出試験](guarantees.md#検出試験)に従う。

## 雛形と計画

`init` が用意する既定の雛形を、利用側の `.devkit/plans/default.json` に置く例。`assignee` の `worker` は作業者(セッション自身)、`human` は人を表す。遷移の `fix`・`ask`・`back`・`done` は、それぞれ修正する・人に確認する・前のステージに戻る・完了するを表し、`done` の `null` は最後のステージを示す。

```json
{
  "name": "default",
  "start_stage": "implement",
  "meta_loop_stages": [],
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true },
        { "aspect": "wording", "runner": "review", "required": true }
      ],
      "gate": { "required": false, "reason": "公開契約を変えない文言の修正" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "implement", "ask": "implement", "back": "implement", "done": null }
    }
  ],
  "optional_stages": [
    {
      "id": "research",
      "assignee": "worker",
      "checks": [],
      "gate": { "required": false, "reason": "調査結果を次のステージに渡す" },
      "limits": { "max_rounds": 1, "max_no_progress": 1 },
      "transitions": { "fix": "research", "ask": "research", "back": "research", "done": "implement" }
    }
  ]
}
```

課題「保存ボタンの文言の修正」から作った計画の例。調査は不要なので候補の `research` は足さず、`implement` を選ぶ。`template.content_hash` は、上の JSON に末尾の改行を含めた雛形ファイルの SHA-256 の例である。

```json
{
  "version": 1,
  "template": { "name": "default", "content_hash": "ad129ad29196a75002c26666388190219abb207bc4b79b69d1aa23da211e9ee5" },
  "task": "保存ボタンの文言を『登録』から『保存』に変える",
  "completion_conditions": [
    { "id": "C1", "description": "保存ボタンに『保存』と表示され、既存のテストが通る" }
  ],
  "start_stage": "implement",
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "checks": [
        { "aspect": "command:test", "runner": "command:test", "required": true, "completion_condition_ids": ["C1"], "target": "返却後のツリー" },
        { "aspect": "wording", "runner": "review", "required": true, "completion_condition_ids": ["C1"], "target": "保存ボタンの表示とそのテスト" }
      ],
      "gate": { "required": false, "reason": "公開契約を変えない文言の修正" },
      "limits": { "max_rounds": 3, "max_no_progress": 2 },
      "transitions": { "fix": "implement", "ask": "implement", "back": "implement", "done": null }
    }
  ],
  "external_operations": []
}
```

上の計画で作業した[終了ブロック](data.md#進捗-progress)の例。`instruction_id` には作業の指示で受け取った ID を入れる。提出は[作業結果の提出](flow.md#作業結果の提出)、受理は[状態遷移と受理条件](data.md#状態遷移と受理条件)に従う。

```json
{
  "instruction_id": "<作業指示 ID>",
  "termination_kind": "完了を主張",
  "work_performed": ["保存ボタンの文言を『登録』から『保存』に変更した"],
  "confirmed": ["保存ボタンに『保存』と表示され、既存のテストが通る"],
  "unconfirmed": [],
  "obstacles": [],
  "temporary_state": [],
  "resume_conditions": []
}
```

## 設定

`devkit.toml` の例。実行ファイル・固定引数・期限を登録する。`review` は上の雛形と計画で画面を起動・確認する[実行主体(runner)](data.md#計画-plan)(check の担当)であり、`[judges]` の判定コマンドとして登録する。スクリプトと出力スキーマのパスは利用側で用意するものに合わせる。

```toml
kit_version = "v1"

[commands]
test = { executable = "./scripts/test", args = [], timeout_seconds = 120 }

[judges]
review = { executable = "./scripts/judge", args = [], timeout_seconds = 120, output_schema = ".devkit/kit/v1/schemas/judgment.json" }

[deny]
rules = ["Bash(git push *)", "Agent(fork)"]

[router]
human_confirmation = ["billing_changed"]
```

`[router]` は課金に関わる変更を人への確認の分類に追記する例であり、[分類表の既定](guarantees.md#人への確認対象)に従う。`[deny]` の構文は [PreToolUse](hooks.md#pretooluse)、結果ファイルの保護は[操作の禁止](guarantees.md#操作の禁止とサブエージェントの制限)に従う。

## 観点と検出試験

観点ファイル `.devkit/checks/wording.md` の例。[Judgment](data.md#check-の結果-judgment)(観点ごとの判定結果)を返し、`NOT ASSESSED`(未評価)の判定値は同じ定義に従う。

```markdown
# wording

- 確認事項: 指定されたボタンの文言が完了条件と一致するか。
- blocking: 指定された文言と表示が異なる。
- 報告しない事項: 対象外のボタンの文言。
- 出力形式: 依頼で渡された Judgment の JSON Schema に従う。表示を確認できなければ NOT ASSESSED とする。
```

試験仕様と例の配置は次のようにする。

```text
.devkit/checks/
  wording.md
  wording/
    spec.json
    violation.html
    conforming.html
    allowed.html
```

`checks/wording/spec.json` は例のファイル名と期待判定を持つ。

```json
{
  "aspect": "wording",
  "cases": [
    { "kind": "violation", "file": "violation.html", "expected": "FAIL" },
    { "kind": "conforming", "file": "conforming.html", "expected": "PASS" },
    { "kind": "allowed", "file": "allowed.html", "expected": "PASS" }
  ]
}
```

各 HTML の最小の内容は、違反例が `<button id="save">登録</button>`、適合例が `<button id="save">保存</button>`、許容例が `<button id="save">保存</button><button id="other">登録</button>`。試験の完了条件は「`save` の文言が『保存』」とする。起動と画面の確認方法も依頼に含め、上の計画と同じ判定コマンド `review` で実行する。合格の扱いは[検出試験](guarantees.md#検出試験)に従う。

## pre-commit

`.githooks/pre-commit` の例。実行可能にし、`scripts/test` と `scripts/lint` は利用側で用意する。

```sh
#!/bin/sh
set -eu
./scripts/test
./scripts/lint
```

既存の hook がある場合の導入、実行環境、失敗の扱いは[完了条件](guarantees.md#完了条件)に従う。
