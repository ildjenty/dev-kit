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

- **返す値**: 教訓の反映先を返す。候補と優先順は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **対象**: 指摘や失敗、同じ手順の繰り返しやコピー、[メタループの振り返り](../meta-loop.md#メタループ)(会話の記録の検討)から改善案が出たとき。

分類と記録の分担は[作業の指示](../architecture/flow.md#作業の指示)、人による候補の選択は[メタループの入口](../meta-loop.md#メタループ)に従う。移行の記録(教訓を反映先へ移した記録)の保存先と参照の制約は[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従う。

#### 振り返りの判定

- **返す値**: 案の分類を返す。分類値は[メタループの振り返り](../meta-loop.md#メタループ)に従う。
- **対象**: 振り返りで案が出たとき。

## 保証

分類とループの進行の責務の境界は [router の契約](../architecture/modules.md#router)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

router による[完了条件](../architecture/guarantees.md#完了条件)(ステージを終える条件)の照合と、[plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)との分担は [router の契約](../architecture/modules.md#router)に従う。回答・承認の受理は[回答の受理](../architecture/guarantees.md#回答の受理)、その前提となる [hook](../architecture/hooks.md)(ホストイベントとの接続)の稼働確認は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)に従う。

### ロールと実行主体

[ロール](../architecture/data.md#計画-plan)(ループ内の担当区分)と[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する相手)の境界は [router の契約](../architecture/modules.md#router)に従う。

## 試験項目

試験の範囲は正本の [router の試験項目](../architecture/contract-changes.md#router-1)に従う。

- [router の契約](../architecture/modules.md#router)で定めるロールと実行主体の境界の試験。
