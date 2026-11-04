# 最小の例

パターンは計画を組む参考であり、計画者が課題ごとに並び・担当・check・ゲート・上限・指示を決める。機械で守らせる決まりは `devkit.toml` に置く。利用側の資産は既定 `.devkit/plans/`・`.devkit/stages/`・`.devkit/principles/`・`.devkit/checks/`・`.devkit/templates/`・`.devkit/rules.md`・`.devkit/requirements.md`・`.devkit/structure.json` に置き、配置は設定で変更できる。

<a id="雛形と計画"></a>

## パターンと計画

既定の参考の並びは default が purpose・scope・structure・contract・steps・implement・verification・retrospective、fix が implement・verification・retrospective、docs が edit・retrospective である。default の purpose の `done` は `"replan"` であり、要件と完了条件を合意してから残りを組む。計画の開始から `done` で到達できないステージは含めない。

`start --plan fix` は fix を優先する指定である。省略時は `default_plan`(既定 default)を優先し、計画者はほかのパターンも参考にできる。計画の `pattern` に名前と内容ハッシュを記録し、指示の参照コミットで存在とハッシュを確かめる。

`.devkit/plans/fix.json` に配るパターンである。check は名前と必須かだけを持ち、runner・基準・例・経緯は `[checks.<観点>]` で定める。

```json
{
  "name": "fix",
  "start_stage": "implement",
  "stages": [
    {
      "id": "implement",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:test",
          "required": true
        },
        {
          "aspect": "test",
          "required": true
        },
        {
          "aspect": "diff",
          "required": true
        }
      ],
      "gate": {
        "required": false,
        "reason": "Changes to existing code that do not affect hard-to-change decisions"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "implement",
        "ask": "implement",
        "back": "implement",
        "done": "verification"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/implement.json"
      }
    },
    {
      "id": "verification",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:e2e",
          "required": true
        },
        {
          "aspect": "behavior",
          "required": true
        }
      ],
      "gate": {
        "required": false,
        "reason": "Review the results of work required for the task"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "verification",
        "ask": "verification",
        "back": "implement",
        "done": "retrospective"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/verification.json"
      }
    },
    {
      "id": "retrospective",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:test",
          "required": true
        },
        {
          "aspect": "test",
          "required": true
        },
        {
          "aspect": "harness",
          "required": true
        }
      ],
      "gate": {
        "required": false,
        "reason": "Review lessons and preserve requirements and prevention measures"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "retrospective",
        "ask": "retrospective",
        "back": "implement",
        "done": null
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/retrospective.json"
      },
      "history": "all"
    }
  ],
  "optional_stages": [
    {
      "id": "research",
      "assignee": "worker",
      "checks": [],
      "gate": {
        "required": false,
        "reason": "Pass research findings to the next stage"
      },
      "limits": {
        "max_rounds": 1,
        "max_no_progress": 1
      },
      "transitions": {
        "fix": "research",
        "ask": "research",
        "back": "research",
        "done": "implement"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/research.json"
      }
    },
    {
      "id": "prototype",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "contract",
          "required": true
        }
      ],
      "gate": {
        "required": false,
        "reason": "Test unverified assumptions and keep prototypes separate from production"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "prototype",
        "ask": "prototype",
        "back": "prototype",
        "done": "implement"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/prototype.json"
      }
    },
    {
      "id": "handoff",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "wording",
          "required": true
        }
      ],
      "gate": {
        "required": false,
        "reason": "Write the PR description for reviewers"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "handoff",
        "ask": "handoff",
        "back": "implement",
        "done": "retrospective"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/handoff.json"
      }
    }
  ]
}
```

保存ボタンの文言を変える計画である。`pattern.content_hash` は配布する `assets/plans/fix.json` の末尾の改行を含む SHA-256 と一致する。実体 ID と `definition` を分け、遷移は実体 ID を使う。完了条件・対象・指示は課題ごとに書く。入力と報告の形は配布パターンと同じ選び方である。

```json
{
  "version": 1,
  "pattern": {
    "name": "fix",
    "content_hash": "a50b9109c1181f0d92d494c0db9de0d49021d2569e5ab38be2f51efbdabdda2c"
  },
  "task": "保存ボタンの文言を「登録」から「保存」に変える",
  "completion_conditions": [
    {
      "id": "C1",
      "description": "保存ボタンに「保存」と表示され、表示のテストと利用場面の通しの確認が登録済みコマンドで成功する"
    }
  ],
  "start_stage": "implement-1",
  "stages": [
    {
      "id": "implement-1",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:test",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        },
        {
          "aspect": "test",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        },
        {
          "aspect": "diff",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        }
      ],
      "gate": {
        "required": false,
        "reason": "Changes to existing code that do not affect hard-to-change decisions"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "implement-1",
        "ask": "implement-1",
        "back": "implement-1",
        "done": "verification-1"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/implement.json"
      },
      "definition": "implement",
      "instructions": [
        "完了条件 C1 を、この定義の成果と確認方法に照らして満たす"
      ]
    },
    {
      "id": "verification-1",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:e2e",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        },
        {
          "aspect": "behavior",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        }
      ],
      "gate": {
        "required": false,
        "reason": "Review the results of work required for the task"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "verification-1",
        "ask": "verification-1",
        "back": "implement-1",
        "done": "retrospective-1"
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/verification.json"
      },
      "definition": "verification",
      "instructions": [
        "完了条件 C1 を、この定義の成果と確認方法に照らして満たす"
      ]
    },
    {
      "id": "retrospective-1",
      "assignee": "worker",
      "checks": [
        {
          "aspect": "command:test",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        },
        {
          "aspect": "test",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        },
        {
          "aspect": "harness",
          "required": true,
          "completion_condition_ids": [
            "C1"
          ],
          "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
        }
      ],
      "gate": {
        "required": false,
        "reason": "Review lessons and preserve requirements and prevention measures"
      },
      "limits": {
        "max_rounds": 3,
        "max_no_progress": 2
      },
      "transitions": {
        "fix": "retrospective-1",
        "ask": "retrospective-1",
        "back": "implement-1",
        "done": null
      },
      "inputs": [
        {
          "source": "requirements",
          "select": ""
        },
        {
          "source": "rules",
          "select": ""
        },
        {
          "source": "structure",
          "select": ""
        },
        {
          "source": "git",
          "select": ""
        }
      ],
      "report": {
        "source": "templates",
        "select": "reports/retrospective.json"
      },
      "history": "all",
      "definition": "retrospective",
      "instructions": [
        "完了条件 C1 を、この定義の成果と確認方法に照らして満たす"
      ]
    }
  ],
  "external_operations": [],
  "open_questions": []
}
```

`definition` を省くと `id` と同じ定義を使う。同じ定義を何度も使う増分は、`implement-2`・`verification-2` のように新しい ID で組む。走った実体 ID の再利用は同じ仕事の再実行だけを意味する。戻り後の計画には戻り先 ID を含める。

最初の計画、完了条件の変更、外部操作、未決事項を既定では人が確認する。パターンとの違いは承認理由にはせず、前に承認した計画の残り、最初なら参考パターンとの構造の差分として表示する。未決事項への回答は再提案の指示として計画者へ渡す。

implement の終了ブロックである。`instruction_id` は受け取った作業指示 ID に置き換える。`report` の形への適合は証拠や完了の根拠にはしない。

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
  "resume_conditions": [],
  "report": {
    "completion_conditions": [
      { "id": "C1", "tests": ["保存ボタンの表示のテスト"], "verification_results": ["登録済みの test が成功"] }
    ],
    "changes": ["保存ボタンの文言を保存に揃える"]
  },
  "commit_message": "fix: 保存ボタンの文言を修正"
}
```

### 修正の担当の規則

追加する役割を設定し、両ホストの agent 定義を用意する。

```toml
[roles.senior_worker]
kind = "work"
agent = "devkit-senior-worker"
```

計画の implement-1 を次の形にする。判定の観点、必須 check、上限は既定の決まりに従う。

```json
{
  "id": "implement-1",
  "assignee": "worker",
  "checks": [
    {
      "aspect": "command:test",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    },
    {
      "aspect": "test",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    },
    {
      "aspect": "diff",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    }
  ],
  "gate": {
    "required": false,
    "reason": "Changes to existing code that do not affect hard-to-change decisions"
  },
  "limits": {
    "max_rounds": 3,
    "max_no_progress": 2
  },
  "transitions": {
    "fix": [
      {
        "when": {
          "check": "diff",
          "failures_at_least": 2
        },
        "assignee": "senior_worker"
      },
      {
        "when": {
          "human_rejected": true
        },
        "assignee": "senior_worker"
      }
    ],
    "ask": "implement-1",
    "back": "implement-1",
    "done": "verification-1"
  },
  "inputs": [
    {
      "source": "requirements",
      "select": ""
    },
    {
      "source": "rules",
      "select": ""
    },
    {
      "source": "structure",
      "select": ""
    },
    {
      "source": "git",
      "select": ""
    }
  ],
  "report": {
    "source": "templates",
    "select": "reports/implement.json"
  },
  "definition": "implement",
  "instructions": [
    "完了条件 C1 を、この定義の成果と確認方法に照らして満たす"
  ]
}
```

FAIL のラウンドが2回以上で diff が FAIL または NOT ASSESSED なら、次の修正を senior_worker に渡す。人の却下も同じ役割へ渡す。上限3は担当の規則より先に効く。同じ StageRun の比較元と累積差分を保ち、どの規則にも当てはまらなければ worker に戻す。

<a id="分割の雛形と計画"></a>

### 分割のパターンと計画

計画者はパターンを参考に、独立した仕事を `split: true` と `units` で分けられる。implement-1 を次の形にし、後続には verification-1 と retrospective-1 を置く。既定の配布パターンは複数単位に分割しない。

```json
{
  "id": "implement-1",
  "assignee": "worker",
  "checks": [
    {
      "aspect": "command:test",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    },
    {
      "aspect": "test",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    },
    {
      "aspect": "diff",
      "required": true,
      "completion_condition_ids": [
        "C1"
      ],
      "target": "保存ボタンの表示、テスト、利用場面、作業の振り返りのうち、この観点が担当する範囲"
    }
  ],
  "gate": {
    "required": false,
    "reason": "Changes to existing code that do not affect hard-to-change decisions"
  },
  "limits": {
    "max_rounds": 3,
    "max_no_progress": 2
  },
  "transitions": {
    "fix": "implement-1",
    "ask": "implement-1",
    "back": "implement-1",
    "done": "verification-1"
  },
  "inputs": [
    {
      "source": "requirements",
      "select": ""
    },
    {
      "source": "rules",
      "select": ""
    },
    {
      "source": "structure",
      "select": ""
    },
    {
      "source": "git",
      "select": ""
    }
  ],
  "report": {
    "source": "templates",
    "select": "reports/implement.json"
  },
  "definition": "implement",
  "instructions": [
    "完了条件 C1 を、この定義の成果と確認方法に照らして満たす"
  ],
  "split": true,
  "units": [
    {
      "id": "save",
      "description": "保存処理とテストを実装する",
      "scope": "internal/save/ と保存処理のテスト",
      "role": "worker"
    },
    {
      "id": "usage",
      "description": "利用手順を記述する",
      "scope": "docs/save.md"
    }
  ]
}
```

単位の ID は計画内で一意とし、担当範囲の重なりは計画の判定者が確かめる。単位の成果に依存する仕事は別ステージにする。`role` の省略時は worker であり、agent のある work の役割に限る。

セッションは各単位の agent を1行の `devkit-instruction <指示 ID>` で起動する。作業場所は `.devkit/worktrees/<単位の指示 ID>/` であり、単位 ID をディレクトリ名には使わない。全単位の結果を受理後、ツリーを SHA で固定して順にコミットしないマージで合流する。

競合はステージの作業担当へ返す。agent があれば競合中のマージ状態と競合マーカーを持つ修正用 worktree を作り、作業者サブエージェントが1つの修正単位として直す。その結果を合流して残りの SHA を取り込む。agent のない work の役割ならセッションのチェックアウトで直す。合流後の累積差分と単位ごとの報告をステージ全体で判定する。

## 設定

新規導入時に init が書く `devkit.toml` と同じ設定である。`kit_version` と `assets/hosts/roles.toml`・`assets/hosts/deny.toml` の内容を用いる。省略した配置・承認・git・並行数などは設定の既定値を使う。

```toml
kit_version = "v3"

[roles]
read_only_tools = ["Read", "Glob", "Grep"]

[roles.worker]
kind = "work"
agent = "devkit-worker"

[roles.judge]
kind = "judge"
agent = "devkit-judge"

[roles.planner]
kind = "plan"
agent = "devkit-planner"

[roles.human]
kind = "human"

[planning]
role = "planner"
max_rounds = 3
inputs = [{ source = "requirements", select = "" }, { source = "rules", select = "" }, { source = "structure", select = "" }]

[plan_check]
runner = "judge"
schemas = ["plan", "plan-template"]
inputs = [{ source = "requirements", select = "" }, { source = "rules", select = "" }, { source = "structure", select = "" }]

[limits]
max_rounds_ceiling = 3
max_no_progress_ceiling = 2
max_total_rounds = 40

[checks."purpose"]
runner = "judge"
criteria = [{ source = "principles", select = "review/purpose.md" }]
examples = { source = "examples", select = "purpose/spec.json" }

[checks."irreversible"]
runner = "judge"
criteria = [{ source = "principles", select = "review/irreversible.md" }, { source = "principles", select = "designing.md" }]
examples = { source = "examples", select = "irreversible/spec.json" }

[checks."structure"]
runner = "judge"
criteria = [{ source = "principles", select = "review/structure.md" }, { source = "principles", select = "designing.md" }, { source = "principles", select = "structure.md" }, { source = "principles", select = "data.md" }]
examples = { source = "examples", select = "structure/spec.json" }

[checks."contract"]
runner = "judge"
criteria = [{ source = "principles", select = "review/contract.md" }, { source = "principles", select = "data.md" }, { source = "principles", select = "engineering.md" }]
examples = { source = "examples", select = "contract/spec.json" }

[checks."steps"]
runner = "judge"
criteria = [{ source = "principles", select = "review/steps.md" }]
examples = { source = "examples", select = "steps/spec.json" }

[checks."command:test"]
runner = "command:test"

[checks."test"]
runner = "judge"
criteria = [{ source = "principles", select = "review/test.md" }, { source = "principles", select = "testing.md" }]
examples = { source = "examples", select = "test/spec.json" }

[checks."diff"]
runner = "judge"
criteria = [{ source = "principles", select = "review/diff.md" }, { source = "principles", select = "engineering.md" }, { source = "principles", select = "working.md" }, { source = "principles", select = "designing.md" }, { source = "principles", select = "writing.md" }, { source = "principles", select = "verification.md" }, { source = "principles", select = "security.md" }, { source = "templates", select = "pr.md.tmpl" }, { source = "rules", select = "" }]
examples = { source = "examples", select = "diff/spec.json" }

[checks."command:e2e"]
runner = "command:e2e"

[checks."behavior"]
runner = "judge"
criteria = [{ source = "principles", select = "review/behavior.md" }, { source = "principles", select = "verification.md" }, { source = "principles", select = "testing.md" }, { source = "principles", select = "ui.md" }, { source = "rules", select = "" }]
examples = { source = "examples", select = "behavior/spec.json" }

[checks."harness"]
runner = "judge"
criteria = [{ source = "principles", select = "review/harness.md" }, { source = "principles", select = "harness.md" }, { source = "principles", select = "working.md" }, { source = "principles", select = "review/README.md" }, { source = "stages", select = "README.md" }, { source = "stages", select = "retrospective.md" }, { source = "plans", select = "README.md" }, { source = "examples", select = "README.md" }, { source = "templates", select = "brief/judgment.md.tmpl" }]
examples = { source = "examples", select = "harness/spec.json" }
history = "all"

[checks."wording"]
runner = "judge"
criteria = [{ source = "principles", select = "review/wording.md" }, { source = "principles", select = "writing.md" }, { source = "templates", select = "pr.md.tmpl" }]
examples = { source = "examples", select = "wording/spec.json" }

[[loop.stage]]
definition = "implement"
checks = ["command:test", "test", "diff"]

[[loop.stage]]
definition = "verification"
checks = ["command:e2e", "behavior"]

[[loop.stage]]
definition = "retrospective"
checks = ["harness"]

[[loop.path]]
definition = "implement"
follows = "verification"

[[loop.path]]
last = "retrospective"

[deny]
rules = ["Bash(git push *)", "Agent(fork)"]
judge = ["apply_patch", "Edit", "Write", "NotebookEdit"]
```

check の一覧は runner と基準を固定し、計画とパターンは観点を参照する。`[[loop.stage]]` は対象の定義ごとに必須 check を課す。定義は一覧または `"*"` でも選べ、ゲート、許可する役割、上限の天井、`report`・`history`・`outputs` の固定も指定できる。

`[[loop.path]]` の `definition = "implement"` と `follows = "verification"` は、implement の各実体の後に同じ区切りの verification を要求する。`last` は終了する経路の最後を定める。`require` は必ず含める定義、`first` は先頭を定める。計画1つだけでなく、採用済みの完了した実体と、計画の開始から `done` で到達する列を結んだ経路で確かめる。区切りは次の replan または終わりまでである。require と last は null で終わる区切りだけに適用する。

計画の受理時とステージの開始時に決まりを検査する。受理時は指示の参照コミットで定義とすべての文書の選び方が1件以上に解決すること、例の観点が一致すること、検出試験を必須とする設定なら必須の判定に例があることも確かめる。

`[limits] max_rounds_ceiling = 3` と `max_no_progress_ceiling = 2` を超える計画は拒否する。計画の判定のやり直しは `[planning] max_rounds = 3`、作業全体の上限は `[limits] max_total_rounds = 40` が既定である。累計上限で続行ならその時点の累計に40を加えて次の停止点とする。戻りや作り直しで累計を減らさない。

検証コマンドの追加登録である。利用側が実行ファイルを用意し、登録ごとに期限を必ず書く。init は test・e2e を登録しない。未登録なら check が修正可能な FAIL になり、登録はループ自身の変更として人が確認して次のラウンドから使う。

```toml
[commands.test]
executable = "./scripts/test"
args = []
timeout_seconds = 120

[commands.e2e]
executable = "./scripts/e2e"
args = []
timeout_seconds = 120
```

メインのエージェントは合意した課題を `start` に渡す。長い場合は課題引数の代わりに `--task-file` で本文を渡す。会話の写しは取らない。

```sh
devkit start --plan fix --task-file task.txt
```

途中の指示は JSON を `devkit instruct guidance.json` で記録する。次の計画者・作業者の依頼文へ渡し、判定者や検出試験へは渡さない。すでに発行した依頼文を変えず、計画や完了条件の変更は再計画と承認を通す。

```json
{
  "text": "保存後に一覧へ戻る利用場面も作業で確かめる"
}
```

## 観点と検出試験

原則のソースの用途別文書 `.devkit/principles/review/wording.md` は、`principles/writing.md` の原則2・3に照らし、想定読者の前提と表現の具体性を確認する。種別や期待判定を伏せても判定できるように、各例の本文に要求、完了条件、検証方法、対象を含める。違反・適合・許容の構成は `assets/examples/wording/` と揃える。

```text
.devkit/principles/
  writing.md
  review/wording.md
.devkit/checks/wording/
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

各例は要求・完了条件・検証方法・対象を含む匿名の本文として、`criteria` で選んだ基準の文書、出力の約束とともに渡す。判定者には種別・期待判定・`spec.json`・対応表と、それらを含む履歴や参照先を渡さない。devkit が判定と期待を照合する。合格の識別には `[checks.<観点>]` の定義、担当の定義、判定規則を含む判定依頼テンプレート、実際に渡した基準の文書の集合と内容ハッシュ、例の文書の内容ハッシュを含める。必要な別の文書は選び方に明示し、文中の参照は解析しない。既存の違反例・適合例・許容例の期待判定を保つことを確かめる。設定と合格の扱いは[検出試験](guarantees.md#検出試験)に従う。

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
