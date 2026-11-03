# harness

- 確認事項: 原則と契約: `principles/harness.md` の各原則とモジュールごとの契約（`docs/architecture/modules.md#モジュールごとの契約`）に従う。 check の必須化: 仕組みの選定は同原則5、判定者（`docs/architecture/data.md#計画-plan`）(check を担当するロール)が担当する観点を必須にする条件は検出試験（`docs/architecture/guarantees.md#検出試験`）(観点ファイルが期待する判定を出せるかの試験)に従う。 skill: `description`（`https://agentskills.io/specification#description-field`）(skill の説明項目)の有無、配置（`docs/architecture/layout.md#指示ファイルとホスト設定`）、参照先を確認する。実装との処理の分担は `principles/working.md` の原則5（`principles/working.md#5-定型処理はコマンドにし手順はそれを呼ぶ`）に従う。 判定: check の結果（`docs/architecture/data.md#check-の結果-judgment`）に従い、`NOT ASSESSED`(未評価)を返せるか。 原則の内容: 適用条件・要求・例外・blocking の意味を判断できるか。 新しいセッションによる確認: リポジトリと実行の記録を読み、「何のシステムか」「どう動かすか」「どう確かめるか」「今どこか」に答えられるか。
- blocking: 新しいセッションによる確認で答えられない問いがある、照合した原則の blocking 条件に該当する、または照合した契約に違反する。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
