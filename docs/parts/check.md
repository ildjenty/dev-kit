# check — 評価と判定

## 責務

check は固定した[規範](../architecture/modules.md#check)(評価に用いる規則)と[観点ファイル](../architecture/modules.md#check)(判定の基準)を基に判定依頼の内容を決め、結果の解析・受理・集約と[適用除外](../architecture/guarantees.md#判定結果の受理)(特定の要求を外す条件)の主張と根拠の照合を担う。外部入出力の扱いは[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。公開する型と操作、規範の束ね方、必須 check と[検出試験](../architecture/guarantees.md#検出試験)(観点ファイルの試験)の照合、他の部品との境界は [check のモジュール契約](../architecture/modules.md#check)に従い、依頼文の組み立ては [brief](../architecture/modules.md#brief)(依頼文を組み立てる部品)に委ねる。

## 入力と出力

### 入力

評価対象と[対象のバージョン](../architecture/data.md#対象のバージョン-targetversion)(評価対象の識別値)・場所、固定した規範と観点、依頼と結果の照合に必要な資料を受け取る。[ラウンド](../architecture/data.md#ラウンド-round)(作業と判定の往復)の[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)や修正時の[指摘の参照](../architecture/data.md#指摘-finding)を含む入力の構成は[判定依頼の公開契約](../architecture/modules.md#check)、観点「差分」の入力は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

規範の入力は[規範の固定](../architecture/guarantees.md#規範の固定)に従う。対象の受け渡し、コマンドの実行場所と照合は[判定対象の展開と完了時の永続化](../architecture/guarantees.md#判定対象の展開と完了時の永続化)に従う。

[判定者](../architecture/data.md#計画-plan)(評価を担うロール)の結果ファイルまたはコマンド実行結果を受け取り、[受理境界](../architecture/guarantees.md#判定結果の受理)(結果を取り込む際の照合条件)で照合する。判定者サブエージェントの返却形式は [form](../architecture/modules.md#form)(スキーマとテンプレートを持つ部品)と[作業結果の判定](../architecture/flow.md#作業結果の判定)、判定コマンド(`[judges]`)(判定を返すコマンド)の出力スキーマは [config](../architecture/modules.md#config)(設定を解析する実装)に従う。

### 出力

判定依頼の内容と、解析・受理した [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、集約結果、必須 check と検出試験の照合結果を返す。結果に含める [Finding](../architecture/data.md#指摘-finding)(失敗や判断を要する点)の項目は、その定義に従う。

## 保証

### 観点と判定

観点ファイルの形式は [form](../architecture/modules.md#form) のテンプレート、解析の責務は [check のモジュール契約](../architecture/modules.md#check)、配置と最小の例は[観点と検出試験の例](../architecture/examples.md#観点と検出試験)に従う。

判定の値と結果・指摘の構成は [Judgment](../architecture/data.md#check-の結果-judgment)と [Finding](../architecture/data.md#指摘-finding)、適用除外と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

失敗のシナリオを示せない指摘は返さず、重大度は付けない。[blocking](../architecture/modules.md#check)(修正を求める条件)ではない指摘は修正を求めず、参考情報として残す。指摘数に上限は設けず、報告する範囲は観点ファイルの blocking の定義と「報告しない事項」で調整する。

評価対象や外部の文書の扱いは [assets/principles/context.md](../../assets/principles/context.md) の原則3に従う。

省略した手順の報告は [assets/principles/working.md](../../assets/principles/working.md) の原則1、判定後の反証の検討は [assets/principles/verification.md](../../assets/principles/verification.md) の原則1に従う。

### 実行主体と受理

[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する担当)の指定は[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(ループの定義)の check、種類と必要な確認能力は[計画](../architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)と[操作の禁止とサブエージェントの制限](../architecture/guarantees.md#操作の禁止とサブエージェントの制限)に従う。検証コマンド(`[commands]`)(テストなどを実行するコマンド)を使う `command:` の check と判定コマンドの登録は [config](../architecture/modules.md#config)で定める。

サブエージェントの設定は[リポジトリ内の定義](../architecture/layout.md#指示ファイルとホスト設定)、起動時の制限は [PreToolUse](../architecture/hooks.md#pretooluse)(ツール実行前の hook)に従う。起動と結果の来歴の照合、重複結果の扱いは[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

コマンド実行結果の Judgment への変換は[作業結果の判定](../architecture/flow.md#作業結果の判定)と[受理境界](../architecture/guarantees.md#判定結果の受理)に従う。結果の保存の責務は [check のモジュール契約](../architecture/modules.md#check)に従い、判定の有効性は[対象のバージョンと規範の識別](../architecture/data.md#check-の結果-judgment)で照合する。

### 検出試験

検出試験の対象・実行時期・合格記録の再利用・原則の適用除外は[検出試験の保証](../architecture/guarantees.md#検出試験)、合格記録の同一性は [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)(観点ファイルの試験記録)に従う。

### 集約と遷移の境界

集約後の次の行動は [router](../architecture/modules.md#router)(次の行動を選ぶ部品)の[完了条件](../architecture/guarantees.md#完了条件)と [plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)の遷移で扱う。
