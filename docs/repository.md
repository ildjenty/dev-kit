# 利用側リポジトリの構成

対象は、個人開発と、複数人が同じリポジトリで作業するチーム開発の両方である。

## リポジトリ側の構成

各リポジトリには、次のものを用意する。

- **指示ファイル(AGENTS.md と CLAUDE.md)**: 管理・更新方針と判定の規範としての扱いは[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)に従う。
- **配置ガイド**: 置くもの・置かないもの・配置先の案内である。配置先は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)、[brief](parts/brief.md)(作業依頼を組み立てる部品)への取り込みは[作業の指示](architecture/flow.md#作業の指示)に従う。
- **元に戻せないものと外部操作**: 元に戻せないものの共有項目は下記の「規則と強制手段」、課題固有の項目は[計画](architecture/data.md#計画-plan)(雛形から作る課題ごとの実行内容)で扱う。brief に渡す範囲は [Rules](architecture/data.md#規則と強制手段-rules)、外部操作(外部への書き込みの行為)の登録は [config](architecture/modules.md#config)、宣言と承認は[外部操作の承認](architecture/guarantees.md#外部操作の承認)に従う。
- **育てるもの**: リポジトリが所有する資産であり、範囲と管理は[所有と生存期間](architecture/data.md#所有と生存期間)、更新とチームでの共有は[資産の更新と共有](architecture/data.md#資産の更新と共有)に従う。
- **規則と強制手段**: 移行の記録(教訓を反映先へ移した記録)を含め、内容・配置・作業への参照は [Rules](architecture/data.md#規則と強制手段-rules)に従う。
- **設定ファイル**: `devkit.toml` の項目は [config](architecture/modules.md#config)、配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。[判定コマンド](architecture/modules.md#config)(`[judges]`)は別エンジンによる判定に使う。[検証コマンド](architecture/modules.md#config)(`[commands]`)はテスト・型検査などに使い、`command:<name>` の [check](parts/check.md)(評価と判定の部品)から呼ぶ。実行の扱いは[作業結果の判定](architecture/flow.md#作業結果の判定)に従う。
- **実行の記録と生成物**: 保存する記録と再生成できる出力の区分は[所有と生存期間](architecture/data.md#所有と生存期間)、配置と git 管理は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。記録から組み立てる [checkpoint](architecture/data.md#checkpoint)(再開用の派生状態)は保存する記録と区別する。
- **kit の固定物**: kit が供給するバージョン付きの資産であり、供給元と照合は [kit の供給](architecture/modules.md#kit)に従う。

導入時に `init` が用意するものは [install](architecture/modules.md#install)、hook の登録先と設定の追記は[権限の境界](architecture/guarantees.md#権限の境界)に従う。バイナリの配置を含む配布方法は[バイナリの配布方法](architecture/decisions.md#バイナリの配布方法)に未決定として残す。

導入後の照合は [doctor](architecture/modules.md#doctor-の確認項目)、別セッションや compaction(ホストによる会話の要約)後の再開の確認は[結合試験](architecture/contract-changes.md#結合試験)に従う。

## 実行の仕組みとの対応

- **skill**: 作業の進め方をセッションに伝える手順である。開始、`next`、`submit`、問いの見せ方、判定者と計画者のサブエージェントの起動、振り返り、教訓の反映先の候補を人と選ぶ手順を扱う。配置と指示ファイルとの関係は[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)、手順は[処理の流れ](architecture/flow.md)、教訓の記録と分類は[メタループの入口](meta-loop.md#メタループ)を参照する。
- **[runtime](parts/runtime.md)(実行を担う部品)**: セッションからの CLI 呼び出しとホストのイベントを受け取り、進行の指示と観測の記録を返す。責務の分担は[全体構成](architecture.md#全体構成)、作業の提出と完了は[作業結果の提出](architecture/flow.md#作業結果の提出)と[完了条件](architecture/guarantees.md#完了条件)に従う。
- **ロールと実行主体**: ロール(作業や判断を担う役割)と実行主体(runner。check を実行する相手)を区別する。語彙と割り当ては[計画](architecture/data.md#計画-plan)、check の実行主体の指定は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)に従う。計画の作成は[作業の開始](architecture/flow.md#作業の開始)で計画者が担う。

人への問いと回答はセッションのチャットで扱い、CLI が受理して進行を再開する([人の回答と再開](architecture/flow.md#人の回答と再開))。計画の作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

人による中断、`devkit pause` と再開の条件、Stop の継続上限は[中断](architecture/guarantees.md#中断)に従う。
