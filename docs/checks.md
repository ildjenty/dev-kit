# check の観点

[check](architecture/modules.md#check)(観点やコマンドによる確認)は、観点(確認の切り口)ごとの確認事項と [blocking](architecture/data.md#指摘-finding)(進行を止める指摘の条件)で対象を照合する。観点ファイルの配置は [kit と利用側の配置](architecture/layout.md)、形式は [check](architecture/modules.md#check)と[観点と検出試験の例](architecture/examples.md#観点と検出試験)に従う。

各観点を使うステージ(作業を区切る単位)と実行主体(runner)、必須かどうかは[ループの雛形](architecture/data.md#ループの雛形-plantemplate)で定め、起動・操作を要する check は[操作の禁止とサブエージェントの制限](architecture/guarantees.md#操作の禁止とサブエージェントの制限)に従う。各リポジトリでは、繰り返し発生する失敗を新たな観点として追加できる。

観点はリポジトリが所有する `.devkit/checks/<観点ID>.md` に、検出試験の違反例・適合例・許容例は `.devkit/checks/<観点ID>/` に置く。原則は `.devkit/principles/`、規則は `.devkit/rules.md`、検出試験の合格の索引は `.devkit/detect.json` に置く。配置は `devkit.toml` の `[paths]` で変更できる。

判定に適用する原則はリポジトリが所有するものであり、`assets/principles/` は既定の供給元である。

検出試験の判定者には例の本文だけを渡し、種別と期待判定は渡さない。期待との照合は `devkit` が行う。合格の識別には、観点ファイルが参照する `principles/<名前>.md` と規則ファイルの内容ハッシュを含める。原則や規則が変われば、それを参照する観点の合格記録は再利用できない。観点を変えた場合は例も更新する。試験を必須とする設定と実行時期は[検出試験](architecture/guarantees.md#検出試験)に従う。

観点ファイル内の `principles/…`・`checks/…`・`stages/…`・`templates/…`・`rules.md` はリポジトリ側の資産の論理名であり、brief が論理名と実際のパスの対応を示す。`schemas/…` の本文はバイナリが出力スキーマとして渡す。判定依頼には、該当するステージ定義と観点、その観点が参照する原則、規則、要件、関係する構造の記録を本文で入れる。他の規範はスナップショットの `snap/` に展開したパスの一覧で渡す。

## 目的

- **確認事項**: 目的と完了条件の対応を短く説明でき、完了を判定できるか。
- **blocking**: 完了を判定できない。

## 元に戻せないもの

- **確認事項**: 元に戻せないもの(変更困難な決定)の一覧に漏れがないか。判断と確定の時期は [`assets/principles/designing.md` の原則1](../assets/principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。
- **blocking**: 一覧に漏れがある、または同原則1の blocking 条件に該当する。

## 構造

- **確認事項**:
  - 配置: 各配置を責務に基づいて正当化できるか。
  - 原則との照合: [`assets/principles/designing.md`](../assets/principles/designing.md)、[`assets/principles/structure.md`](../assets/principles/structure.md)、[`assets/principles/data.md`](../assets/principles/data.md) の各原則の違反・兆候に該当するか。
  - 単純さ: 対案との比較は [`assets/principles/structure.md` の原則7](../assets/principles/structure.md#7-たどるレイヤーと覚える状態を減らす)に従う。
  - 境界: 外部入力の検証は [`assets/principles/structure.md` の原則2](../assets/principles/structure.md#2-成立条件が保たれる検証は繰り返さない)、更新時の整合性は [`assets/principles/data.md` の原則5](../assets/principles/data.md#5-データの所有と生存期間を定め不変条件を守る境界を決める)に従う。
  - 流儀: 構造の揃え方は [`assets/principles/designing.md` の原則7](../assets/principles/designing.md#7-既存の流儀に揃える)に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 整合

観点 ID は `consistency` である。

- **確認事項**: 文書や設計案の中に矛盾、未定義の事項、意味・実装・判断が分かれる用語の違いがないか。
- **blocking**: 矛盾がある。未定義の事項と、意味・実装・判断が分かれる用語の違いは記録のみとする。

## 校閲

観点 ID は `wording` である。

- **確認事項**: 想定読者の前提と表現の具体性を、[`assets/principles/writing.md`](../assets/principles/writing.md) の原則2・3の違反・兆候に照らして確認する。
- **blocking**: 読み手が意味を取り違える箇所。

## 契約

- **確認事項**: 状態と値の受け入れは [`assets/principles/data.md` の原則3](../assets/principles/data.md#3-不正な状態や値の受け入れを型と検証で防ぐ)、失敗の分類と公開応答は [`assets/principles/engineering.md` の原則1](../assets/principles/engineering.md#1-失敗を隠さない)に従う。
- **blocking**: 照合した原則の blocking 条件に該当する。

## 手順

- **確認事項**: 各ステップを単独で検証できるか。人が一度で読める大きさか。
- **blocking**: 検証できないステップがある。

## 差分

- **確認事項**:
  - コード: 失敗と変更の影響は [`assets/principles/engineering.md`](../assets/principles/engineering.md) の原則1・5に従う。
  - 承認・委任: 判断と操作の範囲は [`assets/principles/working.md`](../assets/principles/working.md) の原則6・7に従い、判定依頼に含まれる有効な承認・委任の一覧と[対象・範囲を照合する](architecture/guarantees.md#判定結果の受理)。
  - 流儀: 差分の揃え方は [`assets/principles/designing.md` の原則7](../assets/principles/designing.md#7-既存の流儀に揃える)に従う。
  - PR 本文: [form](architecture/modules.md#form)(形式と描画を扱う部品)の契約と[文章と記録の規則](rules.md#文章と記録)に従う。
  - 報告: 判断に必要な記載は [`assets/principles/writing.md` の原則1](../assets/principles/writing.md#1-結論と人の判断に必要なことを先に書く)、検証の主張は [`assets/principles/verification.md` の原則1](../assets/principles/verification.md#1-確かめていないことを確かめたように書かない)に従う。PR 本文にも両原則を適用する。
- **blocking**: 照合した原則の blocking 条件に該当する。

## テスト

観点 ID は `test` であり、`default` と `fix` の実装で、必須の `command:test` と併せて使う。

- **確認事項**: 原則 `principles/testing.md`(既定の内容は [`assets/principles/testing.md`](../assets/principles/testing.md))に照らし、完了条件ごとに、具体的なテストがあるか、登録済みの検証コマンドから実行されるかを確かめる。利用者から見える入口(CLI なら引数・標準出力・終了コード)で契約を確かめているか、テストが実装をなぞっていないかも判定する。
- **blocking**: 照合した原則の blocking 条件に該当する。
- **検出試験**: 違反例・適合例・許容例を用意する。

`command:<name>` がラウンド開始時の設定に未登録なら、check は `FAIL`、指摘は `auto-fixable` とする。「検証コマンド `<name>` が登録されていない。検証の手段を作り `devkit.toml` の `[commands.<name>]` に登録する」と伝える。登録はループ自身の変更として人が確認し、次のラウンド開始時のスナップショットから使う。コマンドの期限は登録ごとに必須である。`command:` の check がツリーを変えたときの指摘には、出力先を ignore する案内を含める。

## 動作検証

- **確認事項**:
  - 共通: 起動、操作、コンソールエラー、証拠の取得、後片付けを確認する。
  - データの変更: 保存した内容と読み出した結果が仕様どおりか。
  - 外部との通信の変更: 送受信の内容と失敗時の扱いが仕様どおりか。
  - 画面を変える場合: [画面の規則](rules.md#画面)に従う。描画結果と操作の確認は [`assets/principles/ui.md` の原則9](../assets/principles/ui.md#9-描画結果と操作を確認する)に従い、長い名前、0件、大量のデータ、多言語など、問題が起きやすいデータも使う。
- **blocking**: 保存した内容や読み出した結果、送受信の内容、通信の失敗時の扱いが仕様と異なる。画面を変える場合は、同原則9の blocking 条件に該当する、または画面の規則に違反する場合も blocking とする。未確認の扱いは同原則9に従う。

## 終了時の状態

- **確認事項**: 判定依頼に含まれるその[ラウンド](architecture/data.md#ラウンド-round)(作業の指示から結果に対する判断までの往復)の[終了ブロック](architecture/data.md#進捗-progress)(作業の結果を固定形式で返す記録)を、[計画に記載された完了条件](architecture/data.md#計画-plan)と [`assets/principles/context.md` の原則5](../assets/principles/context.md#5-完了条件を満たして終え中断時は安全に再開できる状態を残す)の中断時の引き継ぎ要件に照らして確認する。必須 check の集約と設定に応じた完了時のコミットの確認は、[完了条件](architecture/guarantees.md#完了条件)に従う `devkit` の完了の確定処理に委ねる。
- **blocking**: 同原則5の blocking 条件に従う。必須の確認が残る場合の判定も同原則5に従う。

## 計画

- **確認事項**:
  - ゲート: [ループの雛形](architecture/data.md#ループの雛形-plantemplate)の理由の書き方に従い、理由を示しているか。人への確認の判定は[人への確認対象](architecture/guarantees.md#人への確認対象)に従っているか。
  - 逸脱: [計画の雛形からの逸脱](architecture/data.md#計画-plan)を確認し、該当する指摘に [`needs-decision`](architecture/data.md#指摘-finding)(人の判断が必要な指摘)を付けているか。
  - 規模: ステージを減らせないか。
  - 完了条件: [計画](architecture/data.md#計画-plan)に従い、ID 付きの文章と、条件ごとの確認手段の妥当性・網羅性を確かめているか。
  - 網羅性: 変更の種類(画面・データ・外部との通信・構造)に必要な検査が含まれるか。
- **blocking**: 完了条件に、check または人のゲートによる確認手段がない。

## harness

毎回の振り返りで、必須の `command:test` と併せてループ自身を評価するメタ観点である。

- **確認事項**:
  - 原則と契約: [`assets/principles/harness.md`](../assets/principles/harness.md) の各原則と[モジュールごとの契約](architecture/modules.md#モジュールごとの契約)に従う。
  - check の必須化: 仕組みの選定は同原則5、[判定者](architecture/data.md#計画-plan)(check を担当するロール)が担当する観点を必須にする条件は[検出試験](architecture/guarantees.md#検出試験)(観点ファイルが期待する判定を出せるかの試験)に従う。
  - skill: [`description`](https://agentskills.io/specification#description-field)(skill の説明項目)の有無、[配置](architecture/layout.md#指示ファイルとホスト設定)、参照先を確認する。実装との処理の分担は [`assets/principles/working.md` の原則5](../assets/principles/working.md#5-定型処理はコマンドにし手順はそれを呼ぶ)に従う。
  - 判定: [check の結果](architecture/data.md#check-の結果-judgment)に従い、`NOT ASSESSED`(未評価)を返せるか。
  - 原則の内容: 適用条件・要求・例外・blocking の意味を判断できるか。
  - 問題の捕捉: 作業全体の判定・指摘(blocking でないものも含む)、`FAIL`・`NOT ASSESSED` の経緯、ラウンド数、人の却下・訂正・作り直しの指示、`note` で伝えられた見逃しを確かめ、各問題をどこで捕まえるべきだったかを判断しているか。
  - 再発防止: 機械の検査、雛形、観点、原則、skill の順(設定 `[lessons] destinations` で変更可能)で反映先を選び、リポジトリの規範へ反映しているか。反映しない理由があるか。観点を変えたら検出試験の例も更新しているか。規則の「手段: 未実装」と繰り返す違反を、テスト・lint・型・pre-commit・検証コマンドへ移す候補にしているか。
  - 規範の働き: どの原則がどの観点・雛形で使われるか、使われない仕組みや負担に見合わない仕組みがないか。その作業で見えた問題・不足・負担を扱い、規範全体の棚卸しは必要が見えたときだけ行っているか。
  - 単純さ: 部品・原則・記録・観点の数と形式に、より単純な対案があるか。比較と blocking の条件は [`assets/principles/harness.md` の原則5](../assets/principles/harness.md#5-防ぐ失敗と運用負担から仕組みを選ぶ)に従う。
  - 新しいセッションによる確認: リポジトリと実行の記録を読み、「何のシステムか」「どう動かすか」「どう確かめるか」「今どこか」に答えられるか。
- **blocking**: 新しいセッションによる確認で答えられない問いがある、照合した原則の blocking 条件に該当する、または照合した契約に違反する。

## skill の試験

- **確認事項**: 依頼と入力だけを渡し、結果を期待結果と照合する。テストケースには「判定できない」場合も含める。評価中だと分かる表現は、試験対象に見せない。
- **blocking**: 期待結果と異なる。
