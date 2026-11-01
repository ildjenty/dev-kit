# 決定の記録

参照先の略称: sofi（手持ちの運用）、p4cloud（手持ちのレビュー原則）、pstack（cursor/plugins の pstack）、harness（walkinglabs/learn-harness-engineering）、CCGS（Claude-Code-Game-Studios）、emil（emilkowalski/skills）、Motionsites（Motionsites-AI/claude-skill）。

| 決定 | 却下した案と出典 | 理由 | 再検討の条件 |
|---|---|---|---|
| 49の agent と3階層の director、team-* skill を採らない | 49の agent と3階層の director、team-* skill（CCGS） | 個人開発では、agent が並列でもレビューは人が1つずつ行う（harness L14 の Orchestration Tax）。ゲートを観点ファイルにする方式だけを採用する | なし |
| registry の YAML、sprint・epic・story の流れ、63のテンプレートを採らない | registry の YAML、sprint・epic・story の流れ、63のテンプレート（CCGS） | 維持の手間に見合わない。状態は ledger で足りる | なし |
| 設定の注入（yaml-helper の2100行）を採らない | 設定の注入（yaml-helper の2100行）（CCGS） | 負担が大きい | なし |
| arena・swarm・orchestrate・autopilot・shipping を採らない | arena・swarm・orchestrate・autopilot・shipping（pstack） | チームとクラウドの VM が前提。2案の比較はステージ2の check で代わりになる | なし |
| モデル名の指定と、各 skill に重複して記載されたフォールバックの説明を採らない | モデル名の指定と、各 skill に重複して記載されたフォールバックの説明（pstack） | router の表1つで足りる | なし |
| statechart と形式検証を採らない | statechart と形式検証（pstack） | 日常の開発では使わない | なし |
| automate-me・recall を採らない | automate-me・recall（pstack） | 会話の記録に依存する。kit が動いてから考える | kit が動いてから |
| feature list の詳細版、OpenTelemetry、graph engineering の用語を採らない | feature list の詳細版、OpenTelemetry、graph engineering の用語（harness） | 個人開発では負担が大きい。ステージの考え方だけ、ループ計画の作成ルール（段階的に具体化し、前提が崩れたら前のステージに戻る）に入れた | なし |
| 講義の中の数値を採らない | 講義の中の数値（harness） | 作者自身が仮想の数値と書いている | なし |
| media shelf、examples の本文、spec のテンプレート本体を採らない | media shelf、examples の本文、spec のテンプレート本体（Motionsites） | LP 専用で、examples と media は再配布できない | なし |
| アニメーション系 skill の中身を採らない | アニメーション系 skill の中身（emil） | フロントエンド専用。必要になったら ui の追加として扱う | 必要になったとき |
| gh stack、ブランチ名、drizzle、mise、Biome、ラベルの運用を採らない | gh stack、ブランチ名、drizzle、mise、Biome、ラベルの運用（sofi） | リポジトリ固有。issue-pr の例の中で示すだけにする | なし |
| worklog（Slack への作業報告）を採らない | worklog（Slack への作業報告）（sofi） | 個人の運用で、ループの役割に当たらない | なし |
