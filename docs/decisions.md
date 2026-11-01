# 決定の記録

参照先の略称: sofi（手持ちの運用）、p4cloud（手持ちのレビュー原則）、pstack（cursor/plugins の pstack）、harness（walkinglabs/learn-harness-engineering）、CCGS（Claude-Code-Game-Studios）、emil（emilkowalski/skills）、Motionsites（Motionsites-AI/claude-skill）。

### 却下した案: 49の agent と3階層の director、team-* skill

- **出典**: CCGS
- **理由**: 個人開発では、agent が並列でもレビューは人が1つずつ行う（harness L14 の Orchestration Tax）。ゲートを観点ファイルにする方式だけを採用する
- **再検討の条件**: なし

### 却下した案: registry の YAML、sprint・epic・story の流れ、63のテンプレート

- **出典**: CCGS
- **理由**: 維持の手間に見合わない。状態は ledger で足りる
- **再検討の条件**: なし

### 却下した案: 設定の注入（yaml-helper の2100行）

- **出典**: CCGS
- **理由**: 負担が大きい
- **再検討の条件**: なし

### 却下した案: arena・swarm・orchestrate・autopilot・shipping

- **出典**: pstack
- **理由**: チームとクラウドの VM が前提。2案の比較はステージ2の check で代わりになる
- **再検討の条件**: なし

### 却下した案: モデル名の指定と、各 skill に重複して記載されたフォールバックの説明

- **出典**: pstack
- **理由**: router の表1つで足りる
- **再検討の条件**: なし

### 却下した案: statechart と形式検証

- **出典**: pstack
- **理由**: 日常の開発では使わない
- **再検討の条件**: なし

### 却下した案: automate-me・recall

- **出典**: pstack
- **理由**: 会話の記録に依存する。kit が動いてから考える
- **再検討の条件**: kit が動いてから

### 却下した案: feature list の詳細版、OpenTelemetry、graph engineering の用語

- **出典**: harness
- **理由**: 個人開発では負担が大きい。ステージの考え方だけ、ループ計画の作成ルール（段階的に具体化し、前提が崩れたら前のステージに戻る）に入れた
- **再検討の条件**: なし

### 却下した案: 講義の中の数値

- **出典**: harness
- **理由**: 作者自身が仮想の数値と書いている
- **再検討の条件**: なし

### 却下した案: media shelf、examples の本文、spec のテンプレート本体

- **出典**: Motionsites
- **理由**: LP 専用で、examples と media は再配布できない
- **再検討の条件**: なし

### 却下した案: アニメーション系 skill の中身

- **出典**: emil
- **理由**: フロントエンド専用。必要になったら ui の追加として扱う
- **再検討の条件**: 必要になったとき

### 却下した案: gh stack、ブランチ名、drizzle、mise、Biome、ラベルの運用

- **出典**: sofi
- **理由**: リポジトリ固有。issue-pr の例の中で示すだけにする
- **再検討の条件**: なし

### 却下した案: worklog（Slack への作業報告）

- **出典**: sofi
- **理由**: 個人の運用で、ループの役割に当たらない
- **再検討の条件**: なし
