# ループ計画

ループは、利用者が組む[ループの雛形](architecture/data.md#ループの雛形-plantemplate)と、計画者が雛形から課題ごとに作る[計画](architecture/data.md#計画-plan)の2層で表す。計画は [plan](architecture/modules.md#plan) の型であり、計画者への依頼文は [brief](architecture/modules.md#brief) の一種である。

kit は、[雛形から計画を作る手順](architecture/flow.md#作業の開始)と、[作業の指示から router の判断まで](architecture/flow.md#作業の指示)の1ラウンドの進め方を固定する。ステージ、担当、check、遷移、ゲート、上限は利用者が雛形として組む。

雛形は git 管理で育てるもの、計画は `.devkit/run/` に置き ignore する実行の記録である。[所有と生存期間](architecture/data.md#所有と生存期間)と[固定配置](architecture/layout.md#利用側のリポジトリ)に従い、残す価値がある決定は PR 本文の「計画の要約と決定」に写して[共有](architecture/data.md#資産の更新と共有)する。

ラウンドの範囲は [Round](architecture/data.md#ラウンド-round)、次の行動の判断に使う入力は [router](architecture/modules.md#router)に従う。必須 check、`command:` の check、pre-commit を伴うコミットの関係は[完了条件](architecture/guarantees.md#完了条件)で定める。

## フォーマット

雛形と計画は英語キーの JSON とし、[plan](architecture/modules.md#plan) が JSON Schema で検証する。項目は [PlanTemplate](architecture/data.md#ループの雛形-plantemplate) と [Plan](architecture/data.md#計画-plan)に従う。

ステージの担当と check の実行主体は [Plan の担当と runner](architecture/data.md#計画-plan)で定める。作業者は担当名を持たないセッション自身であり、計画でモデルを割り当てない。

計画の承認で足りる外部操作は、[Plan の宣言](architecture/data.md#計画-plan)に種別・対象・入力のハッシュと、宣言時の登録定義・実行ファイルの内容ハッシュを持たせる。承認との照合は[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従う。

## 例

JSON の例は [雛形と計画](architecture/examples.md#雛形と計画)、判定コマンド(`[judges]`)と検証コマンド(`[commands]`)の登録例は[設定](architecture/examples.md#設定)を参照する。

## 作成ルール

### 雛形を組む規則

利用者が開始ステージ、ステージの列、check、ゲート、上限、遷移表、候補ステージ、メタループの編集を宣言したステージを組む。各項目の契約は [PlanTemplate](architecture/data.md#ループの雛形-plantemplate)に従う。

- **何を変えるか**に基づいて、開始する[ステージ](stages.md)を決める。目的やスコープが未確定ならステージ0、変更困難な決定（スキーマ、契約、外部への公開など）に触れるならステージ1、責務・配置・依存を変えるならステージ2、変更困難な決定に触れない内部の契約だけならステージ3、コードだけならステージ4または5から始める。公開契約・内部契約のいずれも、保存済みデータ、実行中の処理、利用者と呼び出し元への影響、再生成と復旧の可否から判断する（[`principles/designing.md`](../principles/designing.md) の原則1）。「修正方法を知っているか」では決めない。
- 調査・試作は、計画者が必要に応じて足せる候補ステージとして用意する。
- 画面・データ・外部との通信に対応する「動作検証」を組み込む。起動・操作を要する check の実行主体は[サブエージェントの制限](architecture/guarantees.md#操作の禁止とサブエージェントの制限)に従う。
- 人によるレビューゲートには理由を付ける。確認の分類は[人への確認対象](architecture/guarantees.md#人への確認対象)に従う。
- 画面のデザインを始める前に、画面 ID と表示内容のバージョンに対する人の承認をレビューゲートの通過条件とする。
- check の追加・必須化は `principles/harness.md` の原則5「防ぐ失敗と運用負担から仕組みを選ぶ」に従う。
- 段階的に具体化し、前のステージの出力を次のステージの入力にする。行き詰まったときの戻り先と再計画は `principles/harness.md` の原則3に従う。

### 計画を作る規則

計画への入力は、雛形、依頼内容、リポジトリの事実である。リポジトリの事実には、配置ガイド、元に戻せないものの一覧、利用可能な check を含める。

計画者は完了条件を記し、関係のないステージを外し、check の対象を指定し、必要な調査・試作を候補から足す。作成の範囲は[作業の開始](architecture/flow.md#作業の開始)に従う。

ステージの取捨は [`principles/harness.md`](../principles/harness.md) の原則3の適用除外に従い、変えるものだけを扱う最小の計画に保つ。戻り先を定めるためだけに不要なステージを足さない。

## 計画の check と人によるレビュー

計画自体に[観点](checks.md#計画)「計画」の check を適用し、雛形からの逸脱も確認する。候補にないステージの追加・ゲートの削除・遷移の変更・必須 check の削除は、[Plan の承認条件](architecture/data.md#計画-plan)に従って人の承認を要する。

check と必要なゲートの進行は[計画の check と承認ゲート](architecture/flow.md#計画の-check-と承認ゲート)、作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。
