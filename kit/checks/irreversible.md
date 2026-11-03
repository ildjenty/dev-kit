# irreversible

- 確認事項: 元に戻せないもの(変更困難な決定)の一覧に漏れがないか。判断と確定の時期は `principles/designing.md` の原則1（`principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する`）に従う。
- blocking: 一覧に漏れがある、または同原則1の blocking 条件に該当する。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
