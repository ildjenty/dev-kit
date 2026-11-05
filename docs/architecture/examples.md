# 最小の例

パターンは計画を組む参考であり、計画者が課題ごとに並び・担当・check・ゲート・上限・指示を決める。機械で守らせる決まりは `devkit.toml` に置く。利用側の資産は既定 `.devkit/plans/`・`.devkit/stages/`・`.devkit/principles/`・`.devkit/templates/`・`.devkit/rules.md`・`.devkit/authority.md`・`.devkit/requirements.md`・`.devkit/structure.json` に置き、配置は設定で変更できる。

<a id="雛形と計画"></a>

## パターンと計画

パターンの構成と再計画は[作業の開始](flow.md#作業の開始)と[既定の開発フロー](flow.md#既定の開発フロー)、計画の到達可能な経路は[計画](data.md#計画-plan)に従う。

`start --plan fix` は fix を優先する指定である。省略時は `default_plan`(既定 default)を優先し、計画者はほかのパターンも参考にできる。計画の `pattern` に名前と内容ハッシュを記録し、指示の参照コミットで存在とハッシュを確かめる。

`.devkit/plans/fix.json` に配るパターンである。check は名前と必須かだけを持ち、runner・基準・経緯は `[checks.<観点>]` で定める。

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
        },
        {
          "source": "authority",
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
        "required": true,
        "reason": "Review the increment demo and its assigned completion conditions"
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
        },
        {
          "source": "authority",
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
        },
        {
          "source": "authority",
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
        },
        {
          "source": "authority",
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
        },
        {
          "source": "authority",
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
        },
        {
          "source": "authority",
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
    "content_hash": "a70a45e16e3b0a27f84379c946f69632b8de31fae0cc247364fb8138e8f3f15f"
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
        },
        {
          "source": "authority",
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
        "required": true,
        "reason": "Review the increment demo and its assigned completion conditions"
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
        },
        {
          "source": "authority",
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
        },
        {
          "source": "authority",
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

`definition` を省くと `id` と同じ定義を使う。同じ定義を何度も使う増分は、`design-2`・`implement-2`・`verification-2` のように新しい ID で組む。走った実体 ID の再利用は同じ仕事の再実行だけを意味する。戻り後の計画には戻り先 ID を含める。

承認と未決事項への回答は[計画のバージョン](data.md#計画のバージョン)に従う。

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

### 増分のデモと受け入れ

備品貸し出しアプリで、C1 を「起動して備品一覧をデータストアから表示でき、テストを実行できる」、C2 を「利用者が備品を借り、返却すると一覧の状態が変わる」と合意した例である。increments の成果は、C1 の骨組みを増分1、C2 の貸し出し・返却を増分2とする。配布する [`default` パターン](../../assets/plans/default.json)を参考に、increments の完了後の再計画で次の実体を組む。

```text
design-1 → implement-1 → verification-1 → design-2 → implement-2 → verification-2 → acceptance → retrospective
```

verification-1 の check には C1、verification-2 には C2 を割り当てる。acceptance は次の実体とする。ゲート、デモでの指摘、最後のデモと受け入れを兼ねる場合の扱いは[既定の開発フロー](flow.md#既定の開発フロー)に従う。

```json
{
  "id": "acceptance",
  "assignee": "worker",
  "checks": [
    {"aspect": "command:e2e", "required": true, "completion_condition_ids": ["C1", "C2"], "target": "備品一覧の骨組みと貸し出し・返却の利用場面"},
    {"aspect": "behavior", "required": true, "completion_condition_ids": ["C1", "C2"], "target": "備品一覧の骨組みと貸し出し・返却の利用場面"}
  ],
  "gate": {"required": true, "reason": "Accept end-to-end evidence for all completion conditions"},
  "limits": {"max_rounds": 3, "max_no_progress": 2},
  "transitions": {"fix": "acceptance", "ask": "acceptance", "back": "implement-2", "done": "retrospective"},
  "inputs": [
    {"source": "requirements", "select": ""},
    {"source": "rules", "select": ""},
    {"source": "structure", "select": ""},
    {"source": "git", "select": ""},
    {"source": "authority", "select": ""}
  ],
  "report": {"source": "templates", "select": "reports/verification.json"},
  "definition": "verification",
  "instructions": ["C1 と C2 の操作と観測を条件ごとに報告する"]
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
    },
    {
      "source": "authority",
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
    },
    {
      "source": "authority",
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
inputs = [{ source = "requirements", select = "" }, { source = "rules", select = "" }, { source = "structure", select = "" }, { source = "authority", select = "" }]

[plan_check]
runner = "judge"
schemas = ["plan", "plan-template", "finding"]
inputs = [{ source = "requirements", select = "" }, { source = "rules", select = "" }, { source = "structure", select = "" }, { source = "authority", select = "" }]

[limits]
max_rounds_ceiling = 3
max_no_progress_ceiling = 2
max_total_rounds = 40

[checks."purpose"]
description = "Checks that the purpose and completion conditions preserve the human request."
runner = "judge"
criteria = [{ source = "principles", select = "review/purpose.md" }]

[checks."irreversible"]
description = "Checks difficult-to-change decisions, alternatives, and approval grounds."
runner = "judge"
criteria = [{ source = "principles", select = "review/irreversible.md" }, { source = "principles", select = "designing.md" }, { source = "principles", select = "review/authority.md" }, { source = "authority", select = "" }]
history = "all"

[checks."structure"]
description = "Checks responsibility boundaries, dependencies, and structure records."
runner = "judge"
criteria = [{ source = "principles", select = "review/structure.md" }, { source = "principles", select = "designing.md" }, { source = "principles", select = "structure.md" }, { source = "principles", select = "data.md" }, { source = "principles", select = "review/authority.md" }, { source = "authority", select = "" }]
history = "all"

[checks."contract"]
description = "Checks public contracts, input validation, data ownership, and failures."
runner = "judge"
criteria = [{ source = "principles", select = "review/contract.md" }, { source = "principles", select = "data.md" }, { source = "principles", select = "engineering.md" }, { source = "principles", select = "review/authority.md" }, { source = "authority", select = "" }]
history = "all"

[checks."increments"]
description = "Checks release increments, completion condition coverage, and demonstrable size."
runner = "judge"
criteria = [{ source = "principles", select = "review/increments.md" }]

[checks."command:test"]
description = "Runs the registered test command."
runner = "command:test"

[checks."test"]
description = "Checks test coverage and meaningful assertions through user entry points."
runner = "judge"
criteria = [{ source = "principles", select = "review/test.md" }, { source = "principles", select = "testing.md" }]

[checks."diff"]
description = "Checks the cumulative diff against scope, quality principles, and rules."
runner = "judge"
criteria = [{ source = "principles", select = "review/diff.md" }, { source = "principles", select = "engineering.md" }, { source = "principles", select = "working.md" }, { source = "principles", select = "designing.md" }, { source = "principles", select = "writing.md" }, { source = "principles", select = "verification.md" }, { source = "principles", select = "security.md" }, { source = "templates", select = "pr.md.tmpl" }, { source = "rules", select = "" }, { source = "principles", select = "review/authority.md" }, { source = "authority", select = "" }]
history = "all"

[checks."command:e2e"]
description = "Runs the registered end-to-end verification command."
runner = "command:e2e"

[checks."behavior"]
description = "Checks completion scenarios against end-to-end execution evidence."
runner = "judge"
criteria = [{ source = "principles", select = "review/behavior.md" }, { source = "principles", select = "verification.md" }, { source = "principles", select = "testing.md" }, { source = "principles", select = "ui.md" }, { source = "rules", select = "" }]

[checks."harness"]
description = "Checks lessons, recurrence prevention, requirements, and loop quality."
runner = "judge"
criteria = [{ source = "principles", select = "review/harness.md" }, { source = "principles", select = "harness.md" }, { source = "principles", select = "working.md" }, { source = "principles", select = "review/README.md" }, { source = "stages", select = "README.md" }, { source = "stages", select = "retrospective.md" }, { source = "plans", select = "README.md" }, { source = "templates", select = "brief/judgment.md.tmpl" }]
history = "all"

[checks."wording"]
description = "Checks clarity, accuracy, and consistency of wording."
runner = "judge"
criteria = [{ source = "principles", select = "review/wording.md" }, { source = "principles", select = "writing.md" }, { source = "templates", select = "pr.md.tmpl" }]

[[loop.stage]]
definition = ["scope", "architecture"]
gate = true

[[loop.stage]]
definition = "implement"
checks = ["command:test", "test", "diff"]

[[loop.stage]]
definition = "verification"
checks = ["command:e2e", "behavior"]
gate = true

[[loop.stage]]
definition = "retrospective"
checks = ["harness"]
history = "all"

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

計画の受理時とステージの開始時に決まりを検査する。受理時は指示の参照コミットで定義とすべての文書の選び方が1件以上に解決することも確かめる。

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

途中の指示は JSON を `devkit instruct guidance.json` で記録する。次の計画者・作業者の依頼文へ渡し、判定者へは渡さない。すでに発行した依頼文を変えず、計画や完了条件の変更は再計画と承認を通す。

```json
{
  "text": "保存後に一覧へ戻る利用場面も作業で確かめる"
}
```

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
