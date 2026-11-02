# check の観点

[check](architecture/modules.md#check)(観点やコマンドによる確認)は、観点(確認の切り口)ごとの確認事項と [blocking](architecture/data.md#指摘-finding)(進行を止める指摘の条件)で対象を照合する。観点ファイルの配置は [kit と利用側の配置](architecture/layout.md)、形式は [check](architecture/modules.md#check)と[観点と検出試験の例](architecture/examples.md#観点と検出試験)に従う。

各観点を使うステージ(作業を区切る単位)と実行主体(runner)、必須かどうかは[ループの雛形](architecture/data.md#ループの雛形-plantemplate)で定め、起動・操作を要する check は[操作の禁止とサブエージェントの制限](architecture/guarantees.md#操作の禁止とサブエージェントの制限)に従う。各リポジトリでは、繰り返し発生する失敗を新たな観点として追加できる。

## 目的

- **確認事項**: 目的を1文で説明できるか。完了を判定できるか。
- **blocking**: 完了を判定できない。

## 元に戻せないもの

- **確認事項**: 元に戻せないもの(変更困難な決定)の一覧に漏れがないか。判断と確定の時期は [`principles/designing.md` の原則1](../principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。
- **blocking**: 一覧に漏れがある、または同原則1の blocking 条件に該当する。

## 構造

- **確認事項**:
  - 配置: 各配置を責務に基づいて正当化できるか。
  - 原則との照合: [`principles/designing.md`](../principles/designing.md)、[`principles/structure.md`](../principles/structure.md)、[`principles/data.md`](../principles/data.md) の各原則の違反・兆候に該当するか。
  - 単純さ: 対案との比較は [`principles/structure.md` の原則7](../principles/structure.md#7-たどるレイヤーと覚える状態を減らす)に従う。
  - 境界: 外部入力の検証は [`principles/structure.md` の原則2](../principles/structure.md#2-成立条件が保たれる検証は繰り返さない)、更新時の整合性は [`principles/data.md` の原則5](../principles/data.md#5-データの所有と生存期間を定め不変条件を守る境界を決める)に従う。
  - 流儀: 構造の揃え方は [`principles/designing.md` の原則7](../principles/designing.md#7-既存の流儀に揃える)に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 整合

- **確認事項**: 文書や設計案の中に矛盾、未定義の事項、用語の揺れがないか。
- **blocking**: 矛盾がある。未定義の事項と用語の揺れは記録のみとする。

## 校閲

- **確認事項**: 想定読者の前提と表現の具体性を、[`principles/writing.md`](../principles/writing.md) の原則2・3の違反・兆候に照らして確認する。
- **blocking**: 読み手が意味を取り違える箇所。

## 契約

- **確認事項**: 状態と値の受け入れは [`principles/data.md` の原則3](../principles/data.md#3-不正な状態や値の受け入れを型と検証で防ぐ)、失敗の分類と公開応答は [`principles/engineering.md` の原則1](../principles/engineering.md#1-失敗を隠さない)に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 手順

- **確認事項**: 各ステップを単独で検証できるか。人が一度で読める大きさか。
- **blocking**: 検証できないステップがある。

## 差分

- **確認事項**:
  - コード: 失敗と変更の影響は [`principles/engineering.md`](../principles/engineering.md) の原則1・5に従う。
  - 承認・委任: 判断と操作の範囲は [`principles/working.md`](../principles/working.md) の原則6・7に従い、判定依頼に含まれる有効な承認・委任の一覧と[対象・範囲を照合する](architecture/guarantees.md#判定結果の受理)。
  - 流儀: 差分の揃え方は [`principles/designing.md` の原則7](../principles/designing.md#7-既存の流儀に揃える)に従う。
  - PR 本文: [form](architecture/modules.md#form)(形式と描画を扱う部品)の契約と[文章と記録の規則](rules.md#文章と記録)に従う。
  - 報告: 判断に必要な記載は [`principles/writing.md` の原則1](../principles/writing.md#1-結論と人の判断に必要なことを先に書く)、検証の主張は [`principles/verification.md` の原則1](../principles/verification.md#1-確かめていないことを確かめたように書かない)に従う。PR 本文にも両原則を適用する。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 動作検証

- **確認事項**:
  - 共通: 起動、操作、コンソールエラー、証拠の取得、後片付けを確認する。
  - データの変更: 保存した内容と読み出した結果が仕様どおりか。
  - 外部との通信の変更: 送受信の内容と失敗時の扱いが仕様どおりか。
  - 画面を変える場合: [画面の規則](rules.md#画面)に従う。描画結果と操作の確認は [`principles/ui.md` の原則9](../principles/ui.md#9-描画結果と操作を確認する)に従い、長い名前、0件、大量のデータ、多言語など、問題が起きやすいデータも使う。
- **blocking**: 保存した内容や読み出した結果、送受信の内容、通信の失敗時の扱いが仕様と異なる。画面を変える場合は、同原則9の blocking 条件に該当する、または画面の規則に違反する場合も blocking とする。未確認の扱いは同原則9に従う。

## 終了時の状態

- **確認事項**: 判定依頼に含まれるその[ラウンド](architecture/data.md#ラウンド-round)(作業の指示から結果に対する判断までの往復)の[終了ブロック](architecture/data.md#進捗-progress)(作業の結果を固定形式で返す記録)を、[計画に記載された完了条件](architecture/data.md#計画-plan)と [`principles/context.md` の原則5](../principles/context.md#5-完了条件を満たして終え中断時は安全に再開できる状態を残す)の中断時の引き継ぎ要件に照らして確認する。必須 check の集約と commit の成功の確認は、[完了条件](architecture/guarantees.md#完了条件)に従う `devkit` の完了の確定処理に委ねる。
- **blocking**: 同原則5の blocking 条件に従う。必須の確認が残る場合の判定も同原則5に従う。

## 計画

- **確認事項**:
  - ゲート: [ループの雛形](architecture/data.md#ループの雛形-plantemplate)の理由の書き方に従い、理由を示しているか。人への確認の判定は[人への確認対象](architecture/guarantees.md#人への確認対象)に従っているか。
  - 逸脱: [計画の雛形からの逸脱](architecture/data.md#計画-plan)を確認し、該当する指摘に [`needs-decision`](architecture/data.md#指摘-finding)(人の判断が必要な指摘)を付けているか。
  - 規模: ステージを減らせないか。
  - 網羅性: 各完了条件に対応する check と、変更の種類(画面・データ・外部との通信・構造)に必要な検査が含まれるか。
- **blocking**: 対応する check がない完了条件がある。

## 前提の批評

- **確認事項**: 部品・原則・記録・観点の数と形式に、より単純な対案があるか。
- **blocking**: 対案を採用すると構成要素の数や役割が変わる。

## harness

- **確認事項**:
  - 原則と契約: [`principles/harness.md`](../principles/harness.md) の各原則と[モジュールごとの契約](architecture/modules.md#モジュールごとの契約)に従う。
  - check の必須化: 仕組みの選定は同原則5、[判定者](architecture/data.md#計画-plan)(check を担当するロール)が担当する観点を必須にする条件は[検出試験](architecture/guarantees.md#検出試験)(観点ファイルが期待する判定を出せるかの試験)に従う。
  - skill: [`description`](https://agentskills.io/specification#description-field)(skill の説明項目)の有無、[配置](architecture/layout.md#指示ファイルとホスト設定)、参照先を確認する。実装との処理の分担は [`principles/working.md` の原則5](../principles/working.md#5-定型処理はコマンドにし手順はそれを呼ぶ)に従う。
  - 判定: [check の結果](architecture/data.md#check-の結果-judgment)に従い、`NOT ASSESSED`(未評価)を返せるか。
  - 原則の形式: [原則のフォーマット](../principles/README.md#原則のフォーマット)を満たすか。
  - 新しいセッションによる確認: リポジトリと実行の記録を読み、「何のシステムか」「どう動かすか」「どう確かめるか」「今どこか」に答えられるか。
  - 接続と再開: 正本の[試験項目](architecture/contract-changes.md#試験項目)に従い、部品間の[結合試験](architecture/contract-changes.md#結合試験)を確認する。
- **blocking**: 新しいセッションによる確認で答えられない問いがある、照合した原則の blocking 条件に該当する、または照合した契約に違反する。

## skill の試験

- **確認事項**: 依頼と入力だけを渡し、結果を期待結果と照合する。テストケースには「判定できない」場合も含める。評価中だと分かる表現は、試験対象に見せない。
- **blocking**: 期待結果と異なる。
