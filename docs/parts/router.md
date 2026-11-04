# router — 次の行動の分類

## 責務

router は、分類表に基づいて次の行動を分類する。

## 入力と出力

分類表と判断に必要な値を受け取り、分類と次の行動を返す。[終了ブロック](../architecture/data.md#進捗-progress)(作業結果の記録)の終了の種別を含む入力の構成は [router の契約](../architecture/modules.md#router)に従う。

### 分類表

#### 人への確認

- **返す値**: 人への確認の要否を返す。既定と利用側の上書き・追加・削除による分類、および[暫定判断](../architecture/data.md#決定と証拠-decision--evidence)(人の確認前の決定)の記録と確認は[人への確認対象](../architecture/guarantees.md#人への確認対象)に従う。
- **対象**: [`needs-decision`](../architecture/data.md#指摘-finding)(人の判断が必要な指摘)、[brief](brief.md)(作業の依頼文)に沿った作業中の判断、[計画](../architecture/data.md#計画-plan)(課題固有のループのデータ)に設ける[ゲート](../architecture/data.md#ループの雛形-plantemplate)(人の承認を待つ関門)。

ループ自身の変更は、規範・ホスト設定・skill・リポジトリ内で git が追跡する有効な pre-commit の設定の変更であり、requirements と structure は含めない。`[approval] meta_loop_edit` の既定は `"confirm"` であり、ステージにかかわらず、変更が効く最初のラウンドの前に確認する。最後のラウンドの変更は、判定後、完了時のコミット前に確認する。`"allow"` も設定できる。

計画の承認はパターンとの違いによらず、`[approval] plan` で定める。既定の `"requirements"` は最初の計画、直前に人が承認した完了条件の ID または文面が変わる計画、外部操作を宣言した計画を人に確認する。`"always"` はすべてのバージョンを確認する。`"deviation"` は最初の計画に加え、前に承認した計画の残りと構造が違う計画、外部操作を宣言した計画を確認する。`open_questions` が空でない計画はいずれの方針でも人に確認し、未決事項を残したままの承認を拒否する。同じ完了条件で外部操作も未決事項もない再計画は、既定では再確認しない。

未決事項への回答は、人が推奨案に同意した場合も、セッションのエージェントが `devkit answer --interpretation rejected` と回答本文で記録する。計画者は回答を完了条件へ反映し、未決事項を解消した計画を再提案する。

分類表の追加・削除に未知の分類名があれば設定エラーとする。依存の宣言として扱うファイルは `[classification] dependency_files` で指定する。

判定後の依存の宣言とループ自身の変更の分類は、そのステージ実行の最初のラウンド開始時のスナップショットと返却後のツリー、および規範のソース全体の文書の集合と内容ハッシュを比較する。修正ラウンドでも比較元を保ち、不合格のラウンドで行った変更を含める。規範は各ラウンドの開始時に固定する。

`[router] human_confirmation` で変更できる分類名は `dependency_changed` と `target_changed_again` だけである。`meta_loop_edit` の確認は `[approval] meta_loop_edit` だけに従い、分類表から削除できない。check の結果から次の行動への対応は固定であり、計画の check は `[plan_check] aspect`・`runner` で指定する。

#### check の結果

- **返す値**: [check](../architecture/modules.md#check)(評価と判定)の集約結果から次の行動を返す。分類値と判断後の進行は [router の判断](../architecture/flow.md#router-の判断)に従う。
- **対象**: 各[ステージ](../stages.md)(作業を分ける段階)の check が終わった後。

blocking の指摘に `premise_broken` が付いていれば、「人に確認する」を選び、前のステージへの戻りを推奨として問いに示す。戻るかどうかは人が `back` の答えで決める。

「修正する」を選んだ後の担当は router では決めない。engine が修正ラウンドを開く処理で[修正の担当の規則](../architecture/data.md#修正の担当の規則)を当てはめる。ラウンドの上限は規則より先に照合し、達したら人に確認する。

分割した作業も、合流後のツリーに対するステージの check から次の行動を選ぶ。単位ごとの判定はせず、ゲート・ループ自身の変更の確認・上限・戻り・再計画・問いはステージのラウンドで扱う。

前もって任せる判断は、計画のステージの `delegations` で選んだ文書に範囲を書く。出所付きの原文を diff などの判定者へ渡し、人の回答による承認と区別する。機械で判断の一覧には変換しない。暫定判断で進み次のゲートか PR で確かめる経路を使う。計画・完了条件の変更、外部操作、ループ自身の変更、上限、中断は任せる対象にしない。

`[limits] max_total_rounds` を設定した作業は累計が上限に達したら人に確認する。続行の回答後の次の停止点は、その時点の累計に設定値を足した値である。戻り・作り直し・担当の変更でも累計は減らさない。

#### 教訓の反映先

教訓の記録と反映先の提案は[メタループ](../meta-loop.md#メタループ)、反映先の優先順は `[lessons] destinations` で設定する。既定は、機械の検査(hook・lint・型・検証コマンド) → パターン → 用途別の判定基準 → 原則 → skill であり、リポジトリの資産に反映する。ソースへの改善はリポジトリ内の書き込めるものに限り、リポジトリ外のソースと読み取り専用のソースへの改善は教訓として管理者へ引き継ぐ。

#### 振り返りの判定

retrospective はその作業の問題・見逃し・規範の不足や負担を扱い、どこで捕まえるべきだったか、再発を防ぐ反映先、反映しない理由を残す。規範全体の棚卸しは必要が見えた場合に行う。規範を変えた場合はループ自身の変更として確認し、変えなければ確認なしで終わる。

案に対する判断の記録と変更の承認は[メタループ](../meta-loop.md#メタループ)に従う。

## 保証

分類とループの進行の責務の境界は [router の契約](../architecture/modules.md#router)、ファイルの読み書きの制約は[モジュールごとの契約](../architecture/modules.md#モジュールごとの契約)に従う。

router による[完了条件](../architecture/guarantees.md#完了条件)(ステージを終える条件)の照合と、[plan](../architecture/modules.md#plan)(計画の意味論を扱う実装)との分担は [router の契約](../architecture/modules.md#router)に従う。回答・承認の受理は[回答の受理](../architecture/guarantees.md#回答の受理)、必要な観測の照合は[再開と hook の稼働確認](../architecture/guarantees.md#再開と-hook-の稼働確認)に従う。

### ロールと実行主体

[役割](../architecture/data.md#役割-role)(`devkit.toml` で定義するループ内の担当)と[実行主体(runner)](../architecture/data.md#計画-plan)(check を実行する相手)は、計画と check の一覧に書いた役割の ID とコマンド名で決まる。router の分類表は役割を持たない。境界は [router の契約](../architecture/modules.md#router)に従う。
