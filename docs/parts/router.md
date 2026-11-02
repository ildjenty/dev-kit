# router — 次の行動の決定

## 責務

router は、分類表に基づいて次の行動を決める。

## 入力と出力

分類表の[固定物](../architecture/data.md#所有と生存期間)(kit が供給する定義)とリポジトリ側の追記を解析済みの値として受け取る。[check](../architecture/modules.md#check)(評価と判定)の集約結果、[ラウンド数](../architecture/data.md#ラウンド-round)(作業と判定の往復回数)、[engine](../architecture/modules.md#engine)(進行と記録)が計算した進展の有無、[計画](../architecture/data.md#計画-plan)(課題固有のループのデータ)の上限、[終了ブロック](../architecture/data.md#進捗-progress)(ラウンドの進捗)の終了の種別から、分類と次の行動を返す。

### 分類表

各分類は、分類先と適用場面を持つ。

#### 人への確認

- **分類先**: 人への確認の要否を返す。既定と利用側の追記による分類、および暫定判断(人の確認前の決定)の記録と確認は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **適用場面**: [`needs-decision`](../architecture/data.md#指摘-finding)(人の判断が必要な指摘)、[brief](brief.md)(作業の依頼文)に沿った作業中の判断、計画に設ける[ゲート](../architecture/data.md#ループの雛形-plantemplate)(人の承認を待つ関門)。

#### check の結果

- **分類先**: check の集約結果から次の行動を返す。分類値と判断後の進行は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **適用場面**: 各[ステージ](../stages.md)(作業を区切る単位)の check が終わった後。

#### 教訓の反映先

- **分類先**: 教訓の反映先を返す。候補と優先順は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **適用場面**: 指摘や失敗、同じ手順の繰り返しやコピー、[振り返り](../meta-loop.md#メタループ)(会話の記録の検討)から改善案が出たとき。

教訓の記録から分類の付与、人による候補の選択までの手順は[メタループの入口](../meta-loop.md#メタループ)に従い、router は分類だけを返す。移行の記録(教訓を反映先へ移した記録)の保存先と参照の制約は[規則と強制手段](../architecture/data.md#規則と強制手段-rules)に従い、記録は router の外で行う。

<a id="ロールとエンジン"></a>

#### ロールと実行主体

router は[ロール](../architecture/data.md#計画-plan)(役割の区別)の語彙を持ち、実行主体(runner)を選ばない。[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(再利用するループの定義)の check で定める runner とロールの境界は [router の契約](../architecture/modules.md#router)に従う。

#### 振り返りの判定

振り返りで出た案を受け取り、その採否を返す。分類値は[振り返り](../meta-loop.md#メタループ)に従う。

## 保証

分類とループの進行の責務の境界は [router の契約](../architecture/modules.md#router)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

完了条件の照合は [plan](../architecture/modules.md#plan)(計画の意味と遷移)が定める[完了条件](../architecture/guarantees.md#完了条件)に従う。回答・承認の受理は router の外で行い、[回答の受理](../architecture/guarantees.md#回答の受理)に従う。回答・承認の前提となる hook(ホストイベントとの接続)の稼働確認は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)に従う。

## 試験項目

試験の範囲は[正本の試験項目](../architecture/contract-changes.md#試験項目)に従う。

- ロールの語彙を扱う際に、check の runner を選択・変更しないこと。
