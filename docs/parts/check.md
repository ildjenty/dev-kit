# check — 評価と判定

## 責務

check は固定した[規範](../architecture/modules.md#check)(評価に用いる規則)と check の `criteria` で選んだ基準の文書を基に判定依頼の内容を決め、結果の解析・受理・集約と[適用除外](../architecture/guarantees.md#判定結果の受理)(特定の要求を外す条件)の主張と根拠の照合を担う。外部入出力の扱いは[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。公開する型と操作、規範の束ね方、必須 check と[検出試験](../architecture/guarantees.md#検出試験)(check の判定能力の試験)の照合、他の部品との境界は [check のモジュール契約](../architecture/modules.md#check)に従い、依頼文の組み立ては [brief](../architecture/modules.md#brief)(依頼文を組み立てる部品)に委ねる。

## 入力と出力

### 入力

評価対象と[対象のバージョン](../architecture/data.md#対象のバージョン-targetversion)(評価対象の識別値)・場所、固定した規範と選んだ基準の文書、依頼と結果の照合に必要な資料を受け取る。[ラウンド](../architecture/data.md#ラウンド-round)(作業と判定の往復)の[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)や修正時の[指摘の参照](../architecture/data.md#指摘-finding)を含む入力の構成は[判定依頼の公開契約](../architecture/modules.md#check)、観点「差分」の入力は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

規範の入力は[規範の固定](../architecture/guarantees.md#規範の固定)に従う。

ラウンドの判定依頼は、そのステージ実行の最初のラウンド開始時の比較元の展開先、変更ファイル一覧、devkit が git で生成して `brief/` 内に保存した差分ファイルのパス、指示発行時点の作業履歴ファイルを含む。修正ラウンドでも同じ比較元から返却後への累積差分を渡し、不合格のラウンドで行った変更を含める。規範は各ラウンドの開始時に固定する。短いパッチだけ本文にも載せる。対象の受け渡し、コマンドの実行場所と照合は[判定対象の展開と完了時の永続化](../architecture/guarantees.md#判定対象の展開と完了時の永続化)に従う。

分割した作業も、合流後のツリーを返却後の対象としてステージのラウンドで判定する。判定依頼には単位 ID を付けて欠けなく保持した各単位の終了ブロックを含め、累積差分には単位の変更とマージの結果を含める。単位ごとの判定はしない。

計画の判定依頼には、作業に記録した要求の原文と `start` の課題を固定入力として含める。観点 `plan` は、完了条件と `open_questions` が原文の要求を漏らさず扱い、要求を追加していないかを確かめる。

[判定者](../architecture/data.md#役割-role)(評価を担う `judge` の種類の役割)の結果ファイルまたはコマンド実行結果を受け取り、[受理境界](../architecture/guarantees.md#判定結果の受理)(結果を取り込む際の照合条件)で照合する。判定者サブエージェントの返却形式は [form](../architecture/modules.md#form)(スキーマとテンプレートを持つ部品)と[作業結果の判定](../architecture/flow.md#作業結果の判定)、判定コマンド(`[judges]`)(判定を返すコマンド)の出力スキーマは [config](../architecture/modules.md#config)(設定を解析する実装)に従う。

採用された前のステージの成果と直前のラウンドの報告を固定した参照で受け取る。`history` で経緯を絞っても、対象に対応する検証コマンドの結果、有効な承認、引用された証拠と参照先は必ず受け取る。`outputs` の既定は採用された成果のすべてであり、選ばない成果も固定した索引から参照できる。

check の `schemas` で指定した入力契約と出力スキーマの参照先の和集合を渡す。`brief_by_planner` の計画者指示は補助の情報とし、固定した基準と確認範囲を狭めない。差分・報告・計画・要件は原資料として別に渡す。判定の指示、直前の指示、経緯の範囲、出し直しはラウンド・観点・runner の組で識別する。

### 出力

判定依頼の内容と、解析・受理した [Judgment](../architecture/data.md#check-の結果-judgment)(観点ごとの判定結果)、集約結果、必須 check と検出試験の照合結果を返す。結果に含める [Finding](../architecture/data.md#指摘-finding)(失敗や判断を要する点)の項目は、その定義に従う。

## 保証

### 観点と判定

check の `aspect` は判定の名前、`criteria` は基準の選び方、`examples` は検出試験の例の選び方である。基準は規範のソースから選ぶ。必須の基準の選び方が0件なら判定を始めず止める。基準の本文の構造は解析せず、参照する別の文書が必要なら選び方に明示する。選び方は check の約束であり、雛形と違う場合は逸脱として承認の問いに差分を示す。

判定の値と結果・指摘の構成は [Judgment](../architecture/data.md#check-の結果-judgment)と [Finding](../architecture/data.md#指摘-finding)、適用除外と集約は[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。判定者は、前のステージの前提が崩れたと判断した指摘に任意の `premise_broken` を付けられる。blocking の指摘にこれがあれば、router は戻りを推奨する人への問いにし、判定者の指摘だけでは戻らない。

判定の共通規則は判定依頼テンプレートに集約する。`NOT ASSESSED` の使い方、失敗のシナリオを示せない指摘を返さないこと、重大度を付けないこと、blocking でない指摘を参考情報として残すこと、指摘数に上限を設けないことを定める。判定の範囲・確認事項・blocking・報告しない事項・適用除外は原則のソースの用途別文書で定める。

評価対象や外部の文書の扱いは [assets/principles/context.md](../../assets/principles/context.md) の原則3に従う。

省略した手順の報告は [assets/principles/working.md](../../assets/principles/working.md) の原則1、判定後の反証の検討は [assets/principles/verification.md](../../assets/principles/verification.md) の原則1に従う。

観点 `test` は `principles/testing.md` に照らし、完了条件ごとに具体的なテストがあるか、登録済みコマンドから実行されるか、利用者から見える入口で契約を確かめるか、実装をなぞっていないかを判定する。CLI では引数・標準出力・終了コードを確認する。観点 `harness` は、振り返りでの問題の捕捉先、再発防止の反映先と反映しない理由、どの原則がどの観点・雛形で使われているか、使われない仕組みや負担に見合わない仕組みがないか、前提を確認する。規範全体の棚卸しは必要が見えたときに行う。

### 実行主体と受理

[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する担当)の指定は[ループの雛形](../architecture/data.md#ループの雛形-plantemplate)(ループの定義)の check に従い、サブエージェントの runner は `judge` の種類の[役割](../architecture/data.md#役割-role)の ID に限る。runner の種類と必要な確認能力は[計画](../architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)と[操作の禁止とサブエージェントの制限](../architecture/guarantees.md#操作の禁止とサブエージェントの制限)に従う。検証コマンド(`[commands]`)(テストなどを実行するコマンド)を使う `command:` の check と判定コマンドの登録は [config](../architecture/modules.md#config)で定める。

サブエージェントの設定は[リポジトリ内の定義](../architecture/layout.md#指示ファイルとホスト設定)、起動時の制限は [PreToolUse](../architecture/hooks.md#pretooluse)(ツール実行前の hook)に従う。起動と結果の来歴の照合、重複結果の扱いは[判定結果の受理](../architecture/guarantees.md#判定結果の受理)に従う。

サブエージェントの結果がスキーマまたは内容の検証に失敗した場合は、engine が失敗と結果の拒否を記録して再読せず、同じ対象と依頼内容で出し直し回数を増やして再指示する。2回の再指示でも検証に失敗する場合は人へ問いを出す。最終メッセージを JSON として解析できない場合も hook の失敗の観測を経て同じ扱いとする。

同じ runner の未受理の指示が複数あり、指示 ID のない不正な最終メッセージをどの指示にも結び付けられない場合は、観測を残して人に問う。

コマンド実行結果の Judgment への変換は[作業結果の判定](../architecture/flow.md#作業結果の判定)と[受理境界](../architecture/guarantees.md#判定結果の受理)に従う。結果の保存の責務は [check のモジュール契約](../architecture/modules.md#check)に従い、判定の有効性は[対象のバージョンと規範の識別](../architecture/data.md#check-の結果-judgment)で照合する。

ラウンド開始時の `devkit.toml` に `command:<name>` が登録されていない場合、その check を `FAIL` とし、`auto-fixable` の指摘「検証コマンド `<name>` が登録されていない。検証の手段を作り `devkit.toml` の `[commands.<name>]` に登録する」を返す。登録はループ自身の変更として `[approval] meta_loop_edit` に従い確認し、次のラウンドのスナップショットから使う。検証コマンドがツリーを変更した場合の指摘には、出力先を ignore する案内を含める。

### 検出試験

検出試験の対象・実行時期・合格記録の再利用・原則の適用除外は[検出試験の保証](../architecture/guarantees.md#検出試験)、合格記録の同一性は [DetectionTest](../architecture/data.md#検出試験の記録-detectiontest)(check の試験記録)に従う。

判定者への依頼には、基準の文書、匿名の例の本文、出力の約束だけを渡す。試験仕様・期待判定・対応表と、それらを含む作業履歴や参照先を渡さず、devkit が結果と期待を照合する。

結果は例 ID ごとの判定値・blocking の指摘・根拠だけの専用の小さい形式とし、共通の識別は1回だけ扱う。agent の定義にあるモデル・推論量の指定を合格の識別に含める。親から引き継ぐ場合は観測できた値を来歴にだけ残し、推測せず、その定義での合格として扱う。

合格の識別には、担当の定義、判定規則を含む判定依頼テンプレート、実際に渡した基準の文書の集合と内容ハッシュ、例の文書の内容ハッシュを含める。基準を変えた check は次に必須として使うラウンドの前に再試験する。基準を更新するときは検出試験の例も更新し、既存の期待判定を保つことを確かめる。例と仕様は `examples` ソース(既定 `.devkit/checks/`)、基準は `principles` ソース(既定 `.devkit/principles/`)に置き、ソース設定で変更できる。合格の索引は `[state] detect`(既定 `.devkit/detect.json`)に置く。

### 集約と遷移の境界

集約後の次の行動は [router](../architecture/modules.md#router)(次の行動を選ぶ部品)の[完了条件](../architecture/guarantees.md#完了条件)と [plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)の遷移で扱う。
