# structure

- 確認事項: 配置: 各配置を責務に基づいて正当化できるか。 原則との照合: `principles/designing.md`、`principles/structure.md`、`principles/data.md` の各原則の違反・兆候に該当するか。 単純さ: 対案との比較は `principles/structure.md` の原則7（`principles/structure.md#7-たどるレイヤーと覚える状態を減らす`）に従う。 境界: 外部入力の検証は `principles/structure.md` の原則2（`principles/structure.md#2-成立条件が保たれる検証は繰り返さない`）、更新時の整合性は `principles/data.md` の原則5（`principles/data.md#5-データの所有と生存期間を定め不変条件を守る境界を決める`）に従う。 流儀: 構造の揃え方は `principles/designing.md` の原則7（`principles/designing.md#7-既存の流儀に揃える`）に従う。
- blocking: 照合した原則の blocking 条件に該当する。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
