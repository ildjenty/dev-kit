# check の観点ファイル

## 目的

- **[ステージ](stages.md)**: 0
- **確認事項**: 目的を1文で説明できるか。完了を判定できるか。
- **blocking**: 完了を判定できない。

## 不可逆な決定

- **ステージ**: 1
- **確認事項**: [`principles/designing.md`](../principles/designing.md) の原則1に従い、変更困難な決定を影響と再生成・復旧の可否から判断しているか。一覧に漏れがないか。選択を左右する未確認事項を解消してから確定しているか。
- **blocking**: 変更困難な決定が一覧から漏れているか、「後で直せる」扱いになっている。または、選択を左右する未確認事項を残したまま確定している。

## 構造

- **ステージ**: 2
- **確認事項**: 各配置を責務に基づいて正当化できるか。同じ目的と境界の保証を保つ、より単純な構造を1つ作り、元の案の余分な部分を示す。[`principles/designing.md`](../principles/designing.md)、[`principles/structure.md`](../principles/structure.md)、[`principles/data.md`](../principles/data.md) の各原則の違反・兆候（浅いモジュール、常に一緒に変更・理解する知識の分散、共通の不変条件に関わる書き手の調停不足、同じ規則の変更を同時に反映する保証のない処理経路）を確認する。外部入力を利用先で命令や許可範囲外の対象として解釈させない境界と、更新時に必要な整合性を保証する境界が定まっているか（structure の原則2、data の原則5）。既存の同種の処理を探し、揃えているか。揃えない理由が示されているか（designing の原則7）。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 整合

- **ステージ**: 2〜4、文書
- **確認事項**: 文書や設計案の中の矛盾、未定義の事項、用語の揺れ。
- **blocking**: 矛盾がある。未定義の事項と用語の揺れは記録のみとする。

## 校閲

- **ステージ**: 文書
- **確認事項**: [`principles/writing.md`](../principles/writing.md) の原則2・3の違反・兆候と blocking に従う。
- **blocking**: 読み手が意味を取り違える箇所。

## 契約

- **ステージ**: 3
- **確認事項**: 不正な状態や値を、型・境界の検証・DB 制約・更新時の検査で拒否できるか（[`principles/data.md`](../principles/data.md) の原則3）。復旧・調査・制御に必要な失敗の種類を内部で区別でき、公開応答が秘匿すべき情報を漏らさないか（[`principles/engineering.md`](../principles/engineering.md) の原則1）。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 手順

- **ステージ**: 4
- **確認事項**: 各ステップを単独で検証できるか。人が一度で読める大きさか。
- **blocking**: 検証できないステップがある。

## 差分

- **ステージ**: 5
- **確認事項**: [`principles/engineering.md`](../principles/engineering.md) の原則1・5 と、[`principles/working.md`](../principles/working.md) の原則6・7、および各原則の blocking に従う。人の承認が必要な判断・操作に、有効な承認や委任があるか。差分が周囲の同種の処理の流儀と違っていないか。違うなら理由が示されているか（[`principles/designing.md`](../principles/designing.md) の原則7）。PR 本文と報告は、[form](parts/form.md) の PR 本文の契約と [規則と強制手段](rules.md#文章と記録)、および [`principles/writing.md`](../principles/writing.md) の原則1と [`principles/verification.md`](../principles/verification.md) の原則1の blocking に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 動作検証

- **ステージ**: 5
- **確認事項**: 起動、操作、証拠の取得、後片付けを確認し、[規則と強制手段](rules.md#画面) の画面の規則に従う。画面はスクリーンショットを画像として見て、反応・待機・フォーカス移動・アニメーションは実際に操作して確かめる。コンソールエラー、ページ全体の意図しない横スクロール、読めない文字、プレースホルダなどを確認する（[principles/ui.md](../principles/ui.md) の原則9）。比較に必要な情報や対応関係を保つための要素内の横スクロールは許す。長い名前、0件、大量のデータ、多言語など、問題が起きやすいデータも使う。
- **blocking**: ui の原則9の blocking 条件に該当するか、画面の規則に違反する。確認できなかった箇所や性質は `NOT ASSESSED` とする。

## 終了時の状態

- **ステージ**: 5（Stop）
- **確認事項**: 完了時は [runtime](parts/runtime.md) の終了条件と [`principles/context.md`](../principles/context.md) の原則5に従う。中断時は未完了・起動不能・未確認の状態と再開条件、意図して保存した再現手順・証拠の用途・保存先・扱いを確認する。依頼外の復旧が必要なときは、復旧を行わず、状態と再開条件を明示して安全に止める（原則5の中断時の扱い）。
- **blocking**: 完了として扱う作業が合意した完了条件を満たしていない、または安全な停止・再開を妨げる状態がある。必須の確認を実施できなかった場合は `NOT ASSESSED` とする。

## 計画

- **ステージ**: 計画
- **確認事項**: 人によるレビューゲートの理由が [router](parts/router.md) の分類に含まれるか。ステージを減らせないか。**各完了条件に対応する [check](parts/check.md) があるか。変更の種類（画面・データ・外部との通信・構造）に必要な検査が含まれるか。**
- **blocking**: 対応する check がない完了条件がある。

## 前提の批評

- **ステージ**: 設計、harness
- **確認事項**: 部品・原則・記録・観点の数と形式を疑い、対案を作って比較する。
- **blocking**: 採用すると設計の形が変わる。

## harness

- **ステージ**: メタ
- **確認事項**: [`principles/harness.md`](../principles/harness.md) の各原則の違反・兆候と blocking、および [check](parts/check.md)・[ledger](parts/ledger.md)・[router](parts/router.md)・[runtime](parts/runtime.md) の契約に従う。check の必須化では違反例と適合例・許容例の判定理由を確かめ、防ぐ失敗と運用負担から仕組みと適用範囲を選んでいるか（harness の原則2・5）。skill に description があるか。参照先が存在するか。check が `NOT ASSESSED` を返せるか。原則が所定の形式を満たすか。skill・playbook・checkpoint に同じ処理の実装が重複していないか（`principles/working.md` の原則5。同じ既存コマンドの呼び出しは数えない）。Fresh Session Test として、新しいセッションがリポジトリだけを読み、「何のシステムか」「どう動かすか」「どう確かめるか」「今どこか」に答えられるかを確かめる。部品間の接続は runtime の組み立ての試験に従い、保存した checkpoint を別セッションで読み戻し、保存先・読み込み先と版・内容を照合する。
- **blocking**: 答えられない問いがある。照合した原則の blocking 条件に該当するか、部品の契約に違反する。

## skill の試験

- **ステージ**: メタ
- **確認事項**: 依頼と入力だけを渡し、結果を期待結果と照合する。テストケースには「判定できない」場合も含める。評価中だと分かる表現は、試験対象に見せない。
- **blocking**: 期待結果と異なる。

各リポジトリでは、そのリポジトリで繰り返し発生する失敗を、新たな観点として追加できる。
