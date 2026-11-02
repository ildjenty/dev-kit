# router — 次のアクションの決定

router は、分類表に基づいて次の行動を決める。分類表の固定物とリポジトリ側の追記を解析済みの値として受け取り、判断を返す責務は [router の契約](../architecture/modules.md#router)に従う。

## 人への確認

- **分類・ルーティング先**: 作業を停止して確認する / 暫定判断で進めてレビューで確認する / 人による確認は不要
- **適用場面**: [check](check.md) の `needs-decision`、[brief](brief.md) に沿った作業中の判断、計画に設ける人によるレビューゲート。
- **分類の根拠**: kit の既定と利用側の追記による[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。hook の稼働不足は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)で扱う。
- **回答と承認**: 受理の条件は[回答の受理](../architecture/guarantees.md#回答の受理)に従う。

## check の結果

- **分類・ルーティング先**: 修正する / 人に確認する / 前の[ステージ](../stages.md)に戻る / 完了する
- **適用場面**: 各ステージの check が終わった後。
- **入力**: 集約した判定、ラウンド数、engine が計算した進展の有無、計画の上限、停止分類を受け取る。`command:` の check の実行結果も [Judgment](../architecture/data.md#check-の結果-judgment)として扱う。
- **完了の判断**: router は plan の完了条件を照合し、ステージ完了の確定は pre-commit を伴う commit の成功まで含む[完了条件](../architecture/guarantees.md#完了条件)に従う。判断後の進行と commit の成否の扱いは [router の判断](../architecture/flow.md#router-の判断)に従う。

## 教訓の反映先

- **分類・ルーティング先**: ループの雛形を含め、[人への確認対象で定める分類順](../architecture/guarantees.md#人への確認対象)に従う。
- **適用場面**: 指摘や失敗が発生したとき。同じ手順の繰り返しやコピーに気づいたとき。
- **移行の記録**: 移行先の決定 ID と出力は、[規則と強制手段](../architecture/data.md#規則と強制手段-rules)の `.devkit/rules.md` の項目に結び付ける。

## ロールとエンジン

- **ロール**: 作業者・人・判定者・計画者の区別と担当の範囲は [Plan の担当](../architecture/data.md#計画-plan)に従う。
- **分類・ルーティング先**: 判定者・計画者のサブエージェント名、または `[judges]` に登録した判定コマンド名を返す。定義と起動条件は[指示ファイルとホスト設定](../architecture/layout.md#指示ファイルとホスト設定)と[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。
- **適用場面**: 判定・計画の実行主体を選ぶとき。

## 振り返りの判定

- **分類・ルーティング先**: 採用 / 却下 / 保留
- **適用場面**: 振り返りで出た案を分類するとき。

## 保証

router は入出力を行わず、分類と判断を返す。判断に従ってループを進める処理との境界は [router の契約](../architecture/modules.md#router)に従う。

## 試験項目

- 集約した判定、ラウンド数、進展の有無、計画の上限、停止分類の組み合わせによる判断。
- 人への確認対象の既定と追記による分類。
- hook の稼働不足を含む次の指示との接続。

試験の範囲は[契約の試験項目](../architecture/contract-changes.md#試験項目)に従う。
