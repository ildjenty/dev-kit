# contract

- 確認事項: 状態と値の受け入れは `principles/data.md` の原則3（`principles/data.md#3-不正な状態や値の受け入れを型と検証で防ぐ`）、失敗の分類と公開応答は `principles/engineering.md` の原則1（`principles/engineering.md#1-失敗を隠さない`）に従う。
- blocking: 照合した原則の blocking 条件に該当する。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
