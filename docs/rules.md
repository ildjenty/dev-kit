# 規則と強制手段

## コードの構造

- **通信の型（リクエスト・レスポンス）を内部で再利用しない。**: [`principles/design.md`](../principles/design.md) の原則3。手段: 未実装。
- **提供側と下位レイヤーは、利用側と上位レイヤーの具体的な実装に依存しない。利用側に定義した抽象（ポート）への依存は許す。**: [`principles/design.md`](../principles/design.md) の原則4。手段: 未実装。
- **ドメインからフレームワーク・インフラ・アプリの設定の型を import しない。**: [`principles/design.md`](../principles/design.md) の原則4。手段: 未実装。
- **静的な依存関係に循環参照を含めない。**: [`principles/design.md`](../principles/design.md) の原則4。手段: 未実装。
- **ドメインの型にフレームワークの型やアノテーションを含めない。**: [`principles/design.md`](../principles/design.md) の原則8。手段: 未実装。
- **モジュールの公開入口を経由せずに内部のパスを直接 import しない。**: [`principles/design.md`](../principles/design.md) の原則12。手段: 未実装。
- **引数をそのまま渡すだけの関数を置かない。**: [`principles/design.md`](../principles/design.md) の原則15。手段: 未実装。
- **実行期間が重なる並列の作業は、異なる worktree で行う。**: [`principles/design.md`](../principles/design.md) の原則16。手段: 未実装。

## コメントと lint

- **lint の抑制には、空でない理由を必ず記載する。**: [`principles/engineering.md`](../principles/engineering.md) の原則4。手段: 未実装。

## テスト

- **テストは実装前に一度失敗することを確認してテスト ID・対象の版・実行結果を記録し、実装前に確認できない場合は検出試験で代替する。**: [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **実装前に失敗を確認できなかったテストとセキュリティ上の処理のテストでは、検証対象を1か所だけ意図的に壊し、失敗を確認した変異差分と実行結果をテスト ID に結び付けて記録する。**: [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **検出試験では1か所を変更して1回実行し、直ちに元に戻して対象の版が復元されたことを確認する。**: [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **回帰テストは、修正前のコードで失敗することを確認してから信頼する。**: [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **テストに `skip` や `only` を残さない。**: [`principles/testing.md`](../principles/testing.md) の原則9。手段: 未実装。

## 文章と記録

- **確信度は「直接」「裏付けあり」「推論」「推測」「不明」の5値とし、「直接」は一次情報を直接引用できる場合、「裏付けあり」は複数の間接的な証拠が同じ結論を支持する場合、「推論」は解釈、「推測」はほかの説明も同じくらいありうる場合、「不明」は何をどのように調べて見つからなかったかを示す場合に用いる。**: [`principles/verification.md`](../principles/verification.md) の原則2。手段: 未実装。
- **文章に「明らかに」「当然」「単に」を使わない。**: [`principles/verification.md`](../principles/verification.md) の原則2。手段: 未実装。
- **性能比較の両方を、本番と同じビルド・フラグ・キャッシュ・データで計測する。**: [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能比較の両方を複数回、交互に（A, B, A, B）計測し、中央値と範囲を示す。**: [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **計測区間で対象の処理が実際に実行されたことを、開始・完了の記録で確認する。**: [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能の結論は「速い / 遅い / 差がない / 判定できない」のいずれかとする。**: [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能報告には、実行回数、ばらつき、制限要因（limiter）を記載する。**: [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **機械が読む固定フォーマット（終了ブロック、PR 本文の Verification）は、結論と判断材料を先に書く文章構成の検査から除外する。**: [`principles/writing.md`](../principles/writing.md) の原則1。手段: 未実装。
- **人の回答を受け取ったら、番号で指定された場合も選択肢の名前で言い直してから実行する。**: [`principles/writing.md`](../principles/writing.md) の原則8。手段: 未実装。
- **人に判断を求める質問には、選択肢、推奨案、その理由を添える。**: [`principles/writing.md`](../principles/writing.md) の原則8。手段: 未実装。
- **PR 本文とレビューコメントに「私は」と記述しない。**: [`principles/writing.md`](../principles/writing.md) の原則9。手段: 未実装。

## 画面

- **画像や埋め込みの読み込み後に、レイアウトを動かさない。**: [`principles/ui.md`](../principles/ui.md) の原則8。手段: 未実装。
- **文字と背景のコントラストは4.5:1 以上とする。**: [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
- **タッチ端末にホバー効果を適用しない。**: [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
- **モバイルの入力欄のフォントは16px 以上とする。**: [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
- **メニューを開いている間は背景をスクロールさせない。**: [`principles/ui.md`](../principles/ui.md) の原則10。手段: 未実装。
- **アニメーションの各フレームで再描画を発生させない。**: [`principles/ui.md`](../principles/ui.md) の原則10。手段: 未実装。
- **確認対象のページ・状態・版に対応するスクリーンショットを撮影し、取得の記録を残す。**: [`principles/ui.md`](../principles/ui.md) の原則10。手段: 未実装。
