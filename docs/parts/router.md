# router — 次の行動の分類

## 責務

router は、分類表に基づいて次の行動を分類する。

## 入力と出力

分類表と判断に必要な値を受け取り、分類と次の行動を返す。[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の終了の種別を含む入力の構成は [router の契約](../architecture/modules.md#router)に従う。

### 分類表

#### 人への確認

- **返す値**: 人への確認の要否を返す。既定と利用側の上書き・追加・削除による分類、および[暫定判断](../architecture/data.md#決定と証拠-decision--evidence)(人の確認前の決定)の記録と確認は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **対象**: [`needs-decision`](../architecture/data.md#指摘-finding)(人の判断が必要な指摘)、[brief](brief.md)(作業の依頼文)に沿った作業中の判断、[計画](../architecture/data.md#計画-plan)(課題固有のループのデータ)に設ける[ゲート](../architecture/data.md#ループの雛形-plantemplate)(人の承認を待つ関門)。

ループ自身の変更は、規範・ホスト設定・skill・有効な pre-commit の設定の変更であり、requirements と structure は含めない。`[approval] meta_loop_edit` の既定は `"confirm"` であり、ステージにかかわらず、変更が効く最初のラウンドの前に確認する。最後のラウンドの変更は、判定後、完了時のコミット前に確認する。`"allow"` も設定できる。

計画承認は `[approval] plan` に従い、既定の `"requirements"` では、最初の計画、完了条件の ID または文面が直前に人が承認した計画から変わる計画、逸脱のある計画、外部操作を宣言した計画を確認する。同じ完了条件で逸脱も外部操作も未決事項もない再計画には再確認を求めない。`"always"` は毎バージョン、`"deviation"` は逸脱または外部操作の宣言がある場合に確認する。

`open_questions` が空でない計画は、承認方針によらず人の承認を必須とする。

分類表の追加・削除に未知の分類名があれば設定エラーとする。依存の宣言として扱うファイルは `[classification] dependency_files` で指定する。

`[router] human_confirmation` で変更できる分類名は `dependency_changed` と `target_changed_again` だけである。`meta_loop_edit` の確認は `[approval] meta_loop_edit` だけに従い、分類表から削除できない。check の結果から次の行動への対応とロールは固定であり、計画の check は `[plan_check] aspect`・`runner` で指定する。

#### check の結果

- **返す値**: [check](../architecture/modules.md#check)(評価と判定)の集約結果から次の行動を返す。分類値と判断後の進行は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **対象**: 各[ステージ](../stages.md)(作業を分ける段階)の check が終わった後。

#### 教訓の反映先

教訓の記録と反映先の提案は[メタループ](../meta-loop.md#メタループ)、反映先の優先順は `[lessons] destinations` で設定する。既定は、機械の検査(hook・lint・型・検証コマンド) → 雛形 → 観点 → 原則 → skill であり、リポジトリの資産に反映する。

#### 振り返りの判定

retrospective はその作業の問題・見逃し・規範の不足や負担を扱い、どこで捕まえるべきだったか、再発を防ぐ反映先、反映しない理由を残す。規範全体の棚卸しは必要が見えた場合に行う。規範を変えた場合はループ自身の変更として確認し、変えなければ確認なしで終わる。

案に対する判断の記録と変更の承認は[メタループ](../meta-loop.md#メタループ)に従う。

## 保証

分類とループの進行の責務の境界は [router の契約](../architecture/modules.md#router)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

router による[完了条件](../architecture/guarantees.md#完了条件)(ステージを終える条件)の照合と、[plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)との分担は [router の契約](../architecture/modules.md#router)に従う。回答・承認の受理は[回答の受理](../architecture/guarantees.md#回答の受理)、必要な観測の照合は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)に従う。

### ロールと実行主体

[ロール](../architecture/data.md#計画-plan)(ループ内の担当区分)と[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する相手)の境界は [router の契約](../architecture/modules.md#router)に従う。
