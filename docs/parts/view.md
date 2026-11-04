# view — 人向けの表示

## 責務

view は1つの[作業](../architecture/data.md#作業-work)(1課題のループ)を表示する。[ledger](../architecture/modules.md#ledger)(作業の記録)と[構造](../architecture/data.md#構造の記録-structure)(責務と依存関係の資産)から人向けの静的 HTML を生成し、表示の文面は記録から組み立てる。

## 入力と出力

### 入力

対象の作業の ledger と、構造のソースの固定した本文を受け取る。既定の `.devkit/structure.json` は `[sources.structure]` で接続先と配置を変更できる。元のバージョンの識別子を取得できれば来歴も受け取る。リポジトリ内のファイルは git の履歴を使う。取得と受け渡しは [view の契約](../architecture/modules.md#view)に従う。

### 出力

`devkit view` で表示する静的 HTML を返す。表示の構成は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

#### 冒頭の固定形式

作業の現在地と人の次の行動は [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。[記録の網羅性](../architecture/modules.md#view)の定義と入力はリンク先に従う。

#### 主表示

表示のまとまりと各項目の属性は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。[新着](../architecture/modules.md#view)の基準は、正本の定義に従う。

##### 要対応

対応が必要な項目と、関連する[暫定判断](../architecture/data.md#決定と証拠-decision--evidence)(人の確認前の決定)への参照は [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。

##### 現在の到達点と有効な変更

有効な変更と、決定に付いた有効条件・適用条件をそのまま添えた暫定判断の表示は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

##### 経緯

現在に影響する効果と過去の判断・確認の扱いは [view の表示構成](../architecture/modules.md#view-の表示構成)に従って示す。

#### 判断対象ごとの表示

要対応の各項目は、[判断対象](../architecture/modules.md#view-の表示構成)(人が確認・判断する対象)ごとに正本の表示構成に従ってまとめる。

確認状況の表示項目は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。判定値と有効性の条件は[判定結果](../architecture/data.md#check-の結果-judgment)(観点から対象を判定した記録)に従う。

決定の有効条件・適用条件と一時状態の条件は、出典・参照とともにそのまま表示し、有効かどうかはエージェントと人が判断する。条件の充足を機械で判定したり、条件によって表示から除外したりしない。決定の承認状態や記録内の置換・依存関係の状態は、自然文の条件の充足と区別して表示する。

未回答の問いや未解決の指摘などの表示は [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。

承認・却下・保留の状態変更の経路は[人の回答と再開](../architecture/flow.md#人の回答と再開)に従う。

`status` と view は、ステージごとのラウンド数、人に止められた回数と理由、担当の切り替え、検出試験の回数、依頼文の大きさを既存の記録から作業ごとに集計する。集計のための新しい記録は足さない。作業全体の累計ラウンド数は戻り・作り直し・担当の変更でも減らさず、`max_total_rounds` の次の停止点も示す。

#### 構造図

構造の本文は自由な形式の知識である。`schemas/structure.json` に従う JSON のときだけ構造図を描き、それ以外は本文を表示する。構造図の入力は[構造の記録](../architecture/data.md#構造の記録-structure)、生成形式は [view の契約](../architecture/modules.md#view)に従う。図の描き方は [`assets/principles/writing.md` の原則8](../../assets/principles/writing.md#8-define-a-diagrams-purpose-and-show-relationship-kinds-and-directions)に従い、入力に関係の種類がなければ依存関係の図までにとどめる。

## 保証

表示の責務と他の部品との境界は [view の契約](../architecture/modules.md#view)、本体の重複表示と未解決状態の扱いは [view の表示構成](../architecture/modules.md#view-の表示構成)に従う。
