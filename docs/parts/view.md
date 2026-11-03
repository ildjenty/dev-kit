# view — 人向けの表示

## 責務

view は1つの[作業](../architecture/data.md#作業-work)(1課題のループ)を表示する。[ledger](../architecture/modules.md#ledger)(作業の記録)と[構造](../architecture/data.md#構造の記録-structure)(責務と依存関係の資産)から人向けの静的 HTML を生成し、表示の文面は記録から組み立てる。

## 入力と出力

### 入力

対象の作業の ledger を受け取り、[構造ステージ](../stages.md#2-構造)(責務・配置・依存を決める段階)を使う場合は `.devkit/structure.json` の解析済みの値と、その構造の git の履歴も受け取る。履歴の取得と受け渡しは [view の契約](../architecture/modules.md#view)に従う。

### 出力

`devkit view` で表示する静的 HTML を返す。表示の構成は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

#### 冒頭の固定形式

作業の現在地と人の次の行動は [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。[記録の網羅性](../architecture/modules.md#view)の定義と入力はリンク先に従う。

#### 主表示

表示のまとまりと各項目の属性は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。[新着](../architecture/modules.md#view)の基準は、正本の定義に従う。

##### 要対応

対応が必要な項目と、関連する[暫定判断](../architecture/data.md#決定と証拠-decision--evidence)(人の確認前の決定)への参照は [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。

##### 現在の到達点と有効な変更

有効な変更と、撤回する場合に満たす必要がある条件を含む暫定判断の表示は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

##### 経緯

現在に影響する効果と過去の判断・確認の扱いは [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。

#### 判断対象ごとの表示

要対応の各項目は、[判断対象](../architecture/modules.md#view-の表示構成)(人が確認・判断する対象)ごとに正本の表示構成に従ってまとめる。

確認状況の表示項目は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。判定値と有効性の条件は[判定結果](../architecture/data.md#check-の結果-judgment)(観点から対象を判定した記録)に従う。

未回答の問いや未解決の指摘などの表示は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

承認・却下・保留の状態変更の経路は[人の回答と再開](../architecture/flow.md#人の回答と再開)に従う。

#### 構造図

構造図の入力は[構造の記録](../architecture/data.md#構造の記録-structure)、生成形式は [view の契約](../architecture/modules.md#view)に従う。図の描き方は [`principles/writing.md` の原則8](../../principles/writing.md#8-図の目的を定め関係の種類と向きを示す)に従い、入力に関係の種類がなければ依存関係の図までにとどめる。

## 保証

表示の責務と他の部品との境界は [view の契約](../architecture/modules.md#view)、本体の重複表示と未解決状態の扱いは [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。
