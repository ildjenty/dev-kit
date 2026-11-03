計画や固定フォーマットの JSON Schema を置く。
計画の検証は internal/plan、供給されたスキーマの適用は internal/form に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、扱いは [plan](../../docs/architecture/modules.md#plan) と [form](../../docs/architecture/modules.md#form) に従う。

- `progress.json`: `form.Progress`。終了ブロックの指示 ID と終了の種別を必須とする。
- `judgment.json`: `form.JudgmentResult`。判定者の最終メッセージに用い、指示 ID を含める。
- `planner-result.json`: `form.PlannerResult`。指示 ID と計画の JSON を包む。計画の内容は plan が検証する。
- `finding.json`: `form.Finding`。指摘と適用除外の主張・根拠、解決した指摘への参照を扱う。
- `question.json`: `form.Question`。承認対象、回答の選択肢、推奨、停止理由、ステージ、根拠を扱う。
- `instruction.json`: `form.Instruction`。指示の種別に応じて brief、展開先、問い、外部操作の確認対象を扱う。
- `decision.json`: `form.Decision`。判断の内容、適用条件、置換と依存、却下案と証拠を扱う。
- `evidence.json`: `form.Evidence`。調査結果、出典と確認方法を扱う。

各スキーマは参照する定義を自身の `$defs` に含める。kit が読み込んだ JSON を `form.CompileSchema` に渡し、`Schema.Validate` または `Schema.Decode` で適用する。form はファイルを読まず、対象の一致、記録参照の存在、判定の受理、決定の状態は利用側が解決する。

終了の種別ごとの追加必須項目は正本に列挙されていないため、`progress.json` は種別による追加の必須条件を設けない。実施内容などの項目は省略できる。終了ブロックの補足項目を、未知のキーであることだけでは拒否しない。
