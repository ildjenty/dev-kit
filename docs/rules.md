# 規則と強制手段

規則は対象のコードと文書に適用する。「手段: 未実装」は、その規則を強制する実装がまだないことを示す。毎回の振り返りで、これらの項目と繰り返し見つかる違反を、テスト・lint・型・pre-commit・検証コマンドに移す候補にする。ループの仕組みとの分担は[基本方針](design.md#基本方針)に従う。

利用側の規則の正本は `.devkit/rules.md` であり、`devkit.toml` の `[paths] rules` で配置を変更できる。`init` がどのリポジトリにも当てはまる規則の既定をコピーし、以後はリポジトリが所有して育てる。複数の作業で守る変更困難な決定と強制手段を残し、ローカルの作業記録だけに依存させない。規則はラウンド開始時に固定する判定の規範に含め、更新はループ自身の変更として、既定では効力を持つ前に人が確認する。

## コードの構造

- **通信の型(リクエスト・レスポンス)を内部で再利用しない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則6。手段: 未実装。
- **提供側と下位レイヤーは、利用側と上位レイヤーの具体的な実装に依存しない。利用側に定義した抽象(ポート)への依存は許す**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則3。手段: 未実装。
- **ドメインからフレームワーク・インフラ・アプリの設定の型を import しない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則5・6。手段: 未実装。
- **静的な依存関係に循環参照を含めない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則3。手段: 未実装。
- **ドメインの型にフレームワークの型やアノテーションを含めない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則5。手段: 未実装。
- **モジュールの公開入口を経由せずに内部のパスを直接 import しない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則6。手段: 未実装。
- **引数をそのまま渡すだけの関数を置かない**。 [`assets/principles/structure.md`](../assets/principles/structure.md) の原則7。手段: 未実装。

## 作業の進め方

並行作業の配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。共有状態と共通の不変条件の扱いは [`assets/principles/data.md`](../assets/principles/data.md) の原則6に従う。

人による中断と再開、Stop の継続上限は[中断](architecture/guarantees.md#中断)に従う。

## コメントと lint

- **lint の抑制には、空でない理由を必ず記載する**。 [`assets/principles/engineering.md`](../assets/principles/engineering.md) の原則4。手段: 未実装。

## テスト

テスト ID は、リポジトリ内でテストを一意に指す識別子であり、テストファイルのリポジトリ相対パスとテストフレームワーク上の識別名の組で表す。

テストの検出能力の確認は、検証対象を意図的に壊してテストが失敗することを確かめる操作である。観点ファイルを対象にする[検出試験](architecture/guarantees.md#検出試験)とは区別する。

- **テスト(回帰テストを含む)は実装前に一度失敗することを確認する。テスト ID・[対象のバージョン](architecture/data.md#対象のバージョン-targetversion)(確認対象の状態)・実行結果を記録する。実装前に確認できない場合は、次項のテストの検出能力の確認で代替する**。 [`assets/principles/testing.md`](../assets/principles/testing.md) の原則4。手段: 未実装。
- **実装前に失敗を確認できなかったテストとセキュリティ上の処理のテストでは、検証対象を必要最小限の差分で意図的に壊し、失敗を引き起こした変更の差分と実行結果をテスト ID に結び付けて記録する**。 [`assets/principles/testing.md`](../assets/principles/testing.md) の原則4。手段: 未実装。
- **テストの検出能力の確認では、狙う性質を1つに絞り、必要最小限の差分で壊して1回実行し、直ちに元に戻して変更前のソースの状態が復元されたことを確認する**。 [`assets/principles/testing.md`](../assets/principles/testing.md) の原則4。手段: 未実装。
- **テストに `skip` や `only` を残さない**。 [`assets/principles/testing.md`](../assets/principles/testing.md) の原則9。手段: 未実装。

## 文章と記録

- **確信度は「直接」「裏付けあり」「推論」「推測」「不明」の5値とする**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則2。手段: 未実装。

  - 「直接」: 一次情報を直接引用できる場合に用いる。
  - 「裏付けあり」: 複数の間接的な証拠が同じ結論を支持する場合に用いる。
  - 「推論」: 観測から導いた解釈を示す場合に用いる。
  - 「推測」: ほかの説明も同じくらいありうる場合に用いる。
  - 「不明」: 何をどのように調べて見つからなかったかを示す場合に用いる。

- **文章に「明らかに」「当然」「単に」を使わない**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則2。手段: 未実装。
- **結論と判断材料を先に書く文章構成の検査から、次の機械が読む固定フォーマットを除外する**。 [`assets/principles/writing.md`](../assets/principles/writing.md) の原則1。手段: 未実装。

  - [終了ブロック](architecture/data.md#進捗-progress)(ラウンドの結果を記す JSON)

- **人に判断を求める質問には、選択肢、推奨案、その理由を添える**。 [`assets/principles/writing.md`](../assets/principles/writing.md) の原則7。手段: 未実装。
- **PR 本文とレビューコメントに「私は」と記述しない**。 [`assets/principles/writing.md`](../assets/principles/writing.md) の原則2。手段: 未実装。

## 計測と報告

- **性能比較の両方を、結論の範囲に必要なビルド・フラグ・キャッシュ・データの条件を揃えて計測する**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則4。手段: 未実装。
- **性能比較の両方を複数回、交互に(A, B, A, B)計測し、中央値と範囲を示す**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則4。手段: 未実装。
- **計測区間で対象の処理が実際に実行されたことを、出力・処理件数などの根拠で確認する**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則4。手段: 未実装。
- **性能の結論は「速い / 遅い / 差がない / 判定できない」のいずれかとする**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則4。手段: 未実装。
- **性能報告には、実行回数とばらつきを記載する。原因や改善上限を説明する場合は、その説明に必要な観測を示す**。 [`assets/principles/verification.md`](../assets/principles/verification.md) の原則4。手段: 未実装。

## 画面

- **画像や埋め込みの読み込み後に、レイアウトを動かさない**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則7。手段: 未実装。
- **文字と背景のコントラストは4.5:1 以上とする**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則8。手段: 未実装。
- **必要な情報・操作をホバーだけに依存させない**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則8。手段: 未実装。
- **モバイルの入力欄のフォントは16px 以上とする**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則8。手段: 未実装。
- **メニューを開いている間に背景を操作対象から外す場合に限り、背景のスクロールを抑止する**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則9。手段: 未実装。
- **アニメーションでは不要な描画処理がないか、実際の反応と描画品質を確認する**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則9。手段: 未実装。
- **確認対象のページ・状態・[対象のバージョン](architecture/data.md#対象のバージョン-targetversion)に対応するスクリーンショットを撮影し、取得の記録を残す**。 [`assets/principles/ui.md`](../assets/principles/ui.md) の原則9。手段: 未実装。
