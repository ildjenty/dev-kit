# completion

- 確認事項: 判定依頼に含まれるそのラウンド（`docs/architecture/data.md#ラウンド-round`）(作業の指示から結果に対する判断までの往復)の終了ブロック（`docs/architecture/data.md#進捗-progress`）(作業の結果を固定形式で返す記録)を、計画に記載された完了条件（`docs/architecture/data.md#計画-plan`）と `principles/context.md` の原則5（`principles/context.md#5-完了条件を満たして終え中断時は安全に再開できる状態を残す`）の中断時の引き継ぎ要件に照らして確認する。必須 check の集約と commit の成功の確認は、完了条件（`docs/architecture/guarantees.md#完了条件`）に従う `devkit` の完了の確定処理に委ねる。
- blocking: 同原則5の blocking 条件に従う。必須の確認が残る場合の判定も同原則5に従う。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
