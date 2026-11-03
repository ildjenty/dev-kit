router が使う分類表の既定を置く。
分類と判断の処理は internal/router、リポジトリ側の追記は利用側の devkit.toml に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、判断は [router](../../docs/architecture/modules.md#router) に従う。

`default.toml` の `human_confirmation` は人に確認する分類名の列である。

- `dependency_changed`: 依存のマニフェストまたはロックファイルの差分。
- `undeclared_meta_loop_edit`: メタループの編集を宣言していないステージでの、ループ自身の変更。
- `target_changed_again`: commit 前後の対象を1回取り直した後の再変化。

`check_results` は集約結果の既定の行動、`roles` はロールの語彙、`lessons.destinations` は教訓の反映先の優先順を持つ。`fix`・`ask`・`back`・`done` は雛形の遷移と同じ語彙を使う。上限・進展・前提の崩れ・終了の種別・完了条件の照合は router が担い、この対応だけでは完了を確定しない。runner は各 check で指定し、ここでは割り当てない。

正本に分類表の保存形式と分類名の指定はないため、TOML と上記の名前を用いる。集約結果の既定は、修正可能な `FAIL` を `fix`、評価できない `NOT ASSESSED` を `ask`、完了条件を満たす `PASS` を `done` と解釈する。前提が崩れた場合の `back` と計数の上限は、判定値だけでは決めない。
