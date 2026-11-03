# router — 次の行動の分類

## 責務

router は、分類表に基づいて次の行動を分類する。

## 入力と出力

分類表と判断に必要な値を受け取り、分類と次の行動を返す。[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の終了の種別を含む入力の構成は [router の契約](../architecture/modules.md#router)に従う。

### 分類表

#### 人への確認

- **返す値**: 人への確認の要否を返す。既定と利用側の追記による分類、および[暫定判断](../architecture/data.md#決定と証拠-decision--evidence)(人の確認前の決定)の記録と確認は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **対象**: [`needs-decision`](../architecture/data.md#指摘-finding)(人の判断が必要な指摘)、[brief](brief.md)(作業の依頼文)に沿った作業中の判断、[計画](../architecture/data.md#計画-plan)(課題固有のループのデータ)に設ける[ゲート](../architecture/data.md#ループの雛形-plantemplate)(人の承認を待つ関門)。

#### check の結果

- **返す値**: [check](../architecture/modules.md#check)(評価と判定)の集約結果から次の行動を返す。分類値と判断後の進行は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **対象**: 各[ステージ](../stages.md)(作業を分ける段階)の check が終わった後。

#### 教訓の反映先

教訓の記録と反映先の提案は[メタループ](../meta-loop.md#メタループ)、反映先の優先順は[教訓の反映先](../architecture/guarantees.md#教訓の反映先)に従う。

#### 振り返りの判定

案に対する判断の記録と変更の承認は[メタループ](../meta-loop.md#メタループ)に従う。

## 保証

分類とループの進行の責務の境界は [router の契約](../architecture/modules.md#router)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

router による[完了条件](../architecture/guarantees.md#完了条件)(ステージを終える条件)の照合と、[plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)との分担は [router の契約](../architecture/modules.md#router)に従う。回答・承認の受理は[回答の受理](../architecture/guarantees.md#回答の受理)、必要な観測の照合は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)に従う。

### ロールと実行主体

[ロール](../architecture/data.md#計画-plan)(ループ内の担当区分)と[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する相手)の境界は [router の契約](../architecture/modules.md#router)に従う。
