# 規則と強制手段

規則は対象のコードと文書に適用する。「手段: 未実装」は、その規則を強制する実装がまだないことを示す。ループの仕組みとの分担は[基本方針](design.md#基本方針)に従う。

利用側の規則の正本は[規則と強制手段](architecture/data.md#規則と強制手段-rules)、既定の配置は [dev-kit のリポジトリ](architecture/layout.md#dev-kit-のリポジトリ)、初期化は [install](architecture/modules.md#install) に従う。

## コードの構造

- **通信の型(リクエスト・レスポンス)を内部で再利用しない**。 [`principles/structure.md`](../principles/structure.md) の原則6。手段: 未実装。
- **提供側と下位レイヤーは、利用側と上位レイヤーの具体的な実装に依存しない。利用側に定義した抽象(ポート)への依存は許す**。 [`principles/structure.md`](../principles/structure.md) の原則3。手段: 未実装。
- **ドメインからフレームワーク・インフラ・アプリの設定の型を import しない**。 [`principles/structure.md`](../principles/structure.md) の原則5・6。手段: 未実装。
- **静的な依存関係に循環参照を含めない**。 [`principles/structure.md`](../principles/structure.md) の原則3。手段: 未実装。
- **ドメインの型にフレームワークの型やアノテーションを含めない**。 [`principles/structure.md`](../principles/structure.md) の原則5。手段: 未実装。
- **モジュールの公開入口を経由せずに内部のパスを直接 import しない**。 [`principles/structure.md`](../principles/structure.md) の原則6。手段: 未実装。
- **引数をそのまま渡すだけの関数を置かない**。 [`principles/structure.md`](../principles/structure.md) の原則7。手段: 未実装。

## 作業の進め方

並行作業の配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。共有状態と共通の不変条件の扱いは [`principles/data.md`](../principles/data.md) の原則6に従う。

## コメントと lint

- **lint の抑制には、空でない理由を必ず記載する**。 [`principles/engineering.md`](../principles/engineering.md) の原則4。手段: 未実装。

## テスト

テスト ID は、リポジトリ内でテストを一意に指す識別子であり、テストファイルのリポジトリ相対パスとテストフレームワーク上の識別名の組で表す。

テストの検出能力の確認は、検証対象を意図的に壊してテストが失敗することを確かめる操作である。観点ファイルを対象にする[検出試験](architecture/guarantees.md#検出試験)とは区別する。

- **テスト(回帰テストを含む)は実装前に一度失敗することを確認する。テスト ID・[対象のバージョン](architecture/data.md#対象のバージョン-targetversion)(確認対象の状態)・実行結果を記録する。実装前に確認できない場合は、次項のテストの検出能力の確認で代替する**。 [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **実装前に失敗を確認できなかったテストとセキュリティ上の処理のテストでは、検証対象を1か所だけ意図的に壊し、失敗を引き起こした変更の差分と実行結果をテスト ID に結び付けて記録する**。 [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **テストの検出能力の確認では1か所を変更して1回実行し、直ちに元に戻して変更前のソースの状態が復元されたことを確認する**。 [`principles/testing.md`](../principles/testing.md) の原則4。手段: 未実装。
- **テストに `skip` や `only` を残さない**。 [`principles/testing.md`](../principles/testing.md) の原則9。手段: 未実装。

## 文章と記録

- **確信度は「直接」「裏付けあり」「推論」「推測」「不明」の5値とする**。 [`principles/verification.md`](../principles/verification.md) の原則2。手段: 未実装。

  - 「直接」: 一次情報を直接引用できる場合に用いる。
  - 「裏付けあり」: 複数の間接的な証拠が同じ結論を支持する場合に用いる。
  - 「推論」: 観測から導いた解釈を示す場合に用いる。
  - 「推測」: ほかの説明も同じくらいありうる場合に用いる。
  - 「不明」: 何をどのように調べて見つからなかったかを示す場合に用いる。

- **文章に「明らかに」「当然」「単に」を使わない**。 [`principles/verification.md`](../principles/verification.md) の原則2。手段: 未実装。
- **結論と判断材料を先に書く文章構成の検査から、次の機械が読む固定フォーマットを除外する**。 [`principles/writing.md`](../principles/writing.md) の原則1。手段: 未実装。

  - [終了ブロック](architecture/data.md#進捗-progress)(ラウンドの結果を記す JSON)
  - [PR 本文の検証の節](parts/form.md#テンプレート)

- **人に判断を求める質問には、選択肢、推奨案、その理由を添える**。 [`principles/writing.md`](../principles/writing.md) の原則7。手段: 未実装。
- **PR 本文とレビューコメントに「私は」と記述しない**。 [`principles/writing.md`](../principles/writing.md) の原則2。手段: 未実装。

## 計測と報告

- **性能比較の両方を、本番と同じビルド・フラグ・キャッシュ・データで計測する**。 [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能比較の両方を複数回、交互に(A, B, A, B)計測し、中央値と範囲を示す**。 [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **計測区間で対象の処理が実際に実行されたことを、開始・完了の記録で確認する**。 [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能の結論は「速い / 遅い / 差がない / 判定できない」のいずれかとする**。 [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。
- **性能報告には、実行回数、ばらつき、制限要因(limiter)を記載する**。 [`principles/verification.md`](../principles/verification.md) の原則4。手段: 未実装。

## 画面

- **画像や埋め込みの読み込み後に、レイアウトを動かさない**。 [`principles/ui.md`](../principles/ui.md) の原則7。手段: 未実装。
- **文字と背景のコントラストは4.5:1 以上とする**。 [`principles/ui.md`](../principles/ui.md) の原則8。手段: 未実装。
- **タッチ端末にホバー効果を適用しない**。 [`principles/ui.md`](../principles/ui.md) の原則8。手段: 未実装。
- **モバイルの入力欄のフォントは16px 以上とする**。 [`principles/ui.md`](../principles/ui.md) の原則8。手段: 未実装。
- **メニューを開いている間は背景をスクロールさせない**。 [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
- **アニメーションの各フレームで再描画を発生させない**。 [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
- **確認対象のページ・状態・[対象のバージョン](architecture/data.md#対象のバージョン-targetversion)に対応するスクリーンショットを撮影し、取得の記録を残す**。 [`principles/ui.md`](../principles/ui.md) の原則9。手段: 未実装。
