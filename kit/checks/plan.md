# plan

- 確認事項: ゲート: ループの雛形（`docs/architecture/data.md#ループの雛形-plantemplate`）の理由の書き方に従い、理由を示しているか。人への確認の判定は人への確認対象（`docs/architecture/guarantees.md#人への確認対象`）に従っているか。 逸脱: 計画の雛形からの逸脱（`docs/architecture/data.md#計画-plan`）を確認し、該当する指摘に `needs-decision`（`docs/architecture/data.md#指摘-finding`）(人の判断が必要な指摘)を付けているか。 規模: ステージを減らせないか。 網羅性: 各完了条件に対応する check と、変更の種類(画面・データ・外部との通信・構造)に必要な検査が含まれるか。
- blocking: 対応する check がない完了条件がある。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
