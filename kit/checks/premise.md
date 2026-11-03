# premise

- 確認事項: 部品・原則・記録・観点の数と形式に、より単純な対案があるか。比較は `principles/harness.md` の原則5（`principles/harness.md#5-防ぐ失敗と運用負担から仕組みを選ぶ`）に従う。
- blocking: 同原則5の blocking 条件に該当する場合だけとする。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
