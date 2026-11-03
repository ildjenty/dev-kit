# diff

- 確認事項: コード: 失敗と変更の影響は `principles/engineering.md` の原則1・5に従う。 承認・委任: 判断と操作の範囲は `principles/working.md` の原則6・7に従い、判定依頼に含まれる有効な承認・委任の一覧と対象・範囲を照合する（`docs/architecture/guarantees.md#判定結果の受理`）。 流儀: 差分の揃え方は `principles/designing.md` の原則7（`principles/designing.md#7-既存の流儀に揃える`）に従う。 PR 本文: form（`docs/architecture/modules.md#form`）(形式と描画を扱う部品)の契約と文章と記録の規則（`docs/rules.md#文章と記録`）に従う。 報告: 判断に必要な記載は `principles/writing.md` の原則1（`principles/writing.md#1-結論と人の判断に必要なことを先に書く`）、検証の主張は `principles/verification.md` の原則1（`principles/verification.md#1-確かめていないことを確かめたように書かない`）に従う。PR 本文にも両原則を適用する。
- blocking: 照合した原則の blocking 条件に該当する。
- 報告しない事項: この観点の確認事項に含まれない事項。失敗のシナリオを示せない指摘。
- 出力形式: 依頼で渡された Judgment の JSON Schema と `docs/architecture/data.md` の Judgment・Finding に従う。必要な判断基準や必須の確認を得られない場合は `NOT ASSESSED` とし、確認済み・未確認の範囲と方法・環境を残す。失敗のシナリオを示せない指摘は返さず、重大度は付けない。指摘は全件残し、blocking でない指摘は参考情報とし修正を求めない。適用除外の主張と根拠を照合し、外れる要求に限って違反としない。
