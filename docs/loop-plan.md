# ループ計画

ループは、利用者が組む[ループの雛形](architecture/data.md#ループの雛形-plantemplate)(再利用するループの定義)と、[計画者](architecture/flow.md#作業の開始)(雛形から計画を作るサブエージェント)が課題ごとに作る[計画](architecture/data.md#計画-plan)(課題に合わせたループのデータ)の2層で表す。計画のデータ型は [plan](architecture/modules.md#plan) モジュールが定め、計画者への依頼文は [brief](architecture/modules.md#brief)(入力から組み立てる依頼文)として渡す。

kit は、[雛形から計画を作る手順](architecture/flow.md#作業の開始)と、1[ラウンド](architecture/data.md#ラウンド-round)(作業の指示から結果に対する判断までの往復)の進め方を固定し、利用者は雛形の構成を定める。

雛形を[育てるもの](architecture/data.md#所有と生存期間)(リポジトリが所有する資産)、計画を[実行の記録](architecture/data.md#所有と生存期間)(作業ごとに残す記録)として扱い、保存先は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)、決定の共有は[資産の更新と共有](architecture/data.md#資産の更新と共有)に従う。

## フォーマット

雛形と計画は英語キーの JSON とし、項目は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)と[計画](architecture/data.md#計画-plan)、検証の方式と責務は [plan](architecture/modules.md#plan)に従う。

ステージ(作業を区切る単位)の担当と [check](architecture/modules.md#check)(観点やコマンドによる確認)の実行主体(runner)は[計画](architecture/data.md#計画-plan)で定める。

個別の承認なしに計画の承認だけで実行してよい[外部操作](architecture/flow.md#外部操作)(リポジトリ外への書き込み行為)は、[計画の宣言](architecture/data.md#計画-plan)と[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従って扱う。

## 例

JSON の例は[雛形と計画](architecture/examples.md#雛形と計画)を参照する。check の実行主体として登録する[判定コマンド(`[judges]`)](architecture/modules.md#config)(判定結果を返すコマンド)と[検証コマンド(`[commands]`)](architecture/modules.md#config)(終了コードで検証結果を返すコマンド)の例は[設定](architecture/examples.md#設定)を参照する。

## 作成ルール

### 雛形を組む規則

利用者は変更の種類(機能追加・修正・文書など)ごとに雛形を組み、担当を含む構成項目は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。

各雛形の開始ステージは、修正方法を知っているかではなく、何を変えるかで決める。元に戻せないもの(変更困難な決定)の判断は、公開契約か内部契約かにかかわらず、[`principles/designing.md` の原則1](../principles/designing.md#1-変更困難な決定は先に特定し判断材料を得てから確定する)に従う。

- 目的やスコープの根拠を具体化するなら、[ステージ0(目的)](stages.md#0-目的)。
- 元に戻せないものに触れるなら、[ステージ1(スコープ)](stages.md#1-スコープ)。
- 責務・配置・依存を変えるなら、[ステージ2(構造)](stages.md#2-構造)。
- 元に戻せないものに触れない内部の契約だけなら、[ステージ3(契約)](stages.md#3-契約)。
- コードだけなら、[ステージ4(手順)](stages.md#4-手順)または[ステージ5(実装)](stages.md#5-実装)。

check とゲート(人の承認を待つ関門)は、次の規則で組む。

- 画面・データ・外部との通信を変えるステージには、観点[「動作検証」](checks.md#動作検証)の check を置く。
- 人によるレビューゲートの理由の書き方は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。人に確認する条件は[人への確認対象](architecture/guarantees.md#人への確認対象)で定める。
- 画面のデザインを始める前のレビューゲートでは、[問いの定義](architecture/data.md#問い-question)に従ってツリーのスナップショットを承認対象とする。
- check の追加・必須化の判断は [`principles/harness.md` の原則5](../principles/harness.md#5-防ぐ失敗と運用負担から仕組みを選ぶ)に従う。[判定者](architecture/data.md#計画-plan)(check を担当するロール)が担当する観点を必須にする条件は、[検出試験](architecture/guarantees.md#検出試験)(観点ファイルが期待する判定を出せるかの試験)に従う。

ステージ間の前提と戻り先の設計は [`principles/harness.md` の原則3](../principles/harness.md#3-前提が崩れたときに戻るステージを決める)に従う。戻り先の表現は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)、入力の組み立ては[作業の指示](architecture/flow.md#作業の指示)、再計画は[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

### 計画を作る規則

計画への入力は、雛形、依頼内容、リポジトリの事実である。リポジトリの事実と brief への取り込みは[作業の指示](architecture/flow.md#作業の指示)に従う。brief の「元に戻せないもの」に渡す範囲は[規則と強制手段](architecture/data.md#規則と強制手段-rules)に従う。

課題ごとの雛形の選択と計画者による取捨の範囲は[作業の開始](architecture/flow.md#作業の開始)に従い、開始ステージの変更を含む逸脱の判定は[計画](architecture/data.md#計画-plan)で定める。

変えるものだけを扱う最小の計画に保ち、戻り先を定めるためだけに不要なステージを足さない。

## 計画の check と人によるレビュー

計画自体に観点[「計画」](checks.md#計画)の check を適用し、雛形からの逸脱と承認条件は[計画](architecture/data.md#計画-plan)に従って確認する。

check と必要なゲートの進行は[計画の check と承認ゲート](architecture/flow.md#計画の-check-と承認ゲート)、作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

## ステージ実行時の扱い

次の行動を選ぶ入力は [router](architecture/modules.md#router)(check の結果から次の行動を分類する部品)、必須 check とコミットの関係は[完了条件](architecture/guarantees.md#完了条件)に従う。起動・操作を要する check の実行主体は[操作の禁止とサブエージェントの制限](architecture/guarantees.md#操作の禁止とサブエージェントの制限)で定める。
