検出試験の違反例・適合例・許容例を置く。
観点ファイルは kit/checks/、検出試験の仕様と照合処理は internal/check に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、検出試験の扱いは [check](../../docs/architecture/modules.md#check) に従う。

`<観点ID>/spec.json` は `docs/architecture/examples.md` の形式に従い、`aspect` と `cases` を持つ JSON とする。各ケースは `kind`（`violation`・`conforming`・`allowed`）、仕様ファイルからの相対パス `file`、期待判定 `expected`（順に `FAIL`・`PASS`・`PASS`）を持つ。各例は `## 依頼` と `## 対象` からなる Markdown とし、架空の入力・成果物・観測記録を使う。期待判定と種別は仕様側に置き、判定者には例の本文と固定した規範・出力スキーマを渡す。計画で指定した担当で実行し、違反例では該当する blocking 条件を根拠に指摘したか、適合例・許容例では判断上の違いや適用除外を無視して止めていないかも照合する。内容ハッシュには例の本文と期待判定を含め、合格の同一性と保存は `docs/architecture/data.md` の DetectionTest に従う。期待判定の掲載だけでは合格記録にせず、実際の判定と根拠を得てから記録する。
