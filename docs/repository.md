# リポジトリ側の構成

個人開発だけでなく、複数人が同じリポジトリで作業するチーム開発を射程に入れる。

## 実行の仕組みとの対応

- **skill**: 開始・`next`・`submit`・問いの見せ方・判定者と計画者のサブエージェントの起動、振り返り、教訓の反映先の選択を扱う。配置と指示ファイルとの関係は[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)、手順は[skill の手順](architecture/contract-changes.md#skill-の手順)に従う。
- **[runtime](parts/runtime.md)**: Claude Code と Codex CLI のセッションから呼ぶ単一バイナリの `devkit` が、CLI による進行と hook による観測を担う。責務の分担は[全体構成](architecture.md#全体構成)、作業の提出と完了は[作業結果の提出](architecture/flow.md#作業結果の提出)と[完了条件](architecture/guarantees.md#完了条件)に従う。
- **作業者・判定者・計画者**: 作業者はセッション自身であり、判定者・計画者はサブエージェントとして起動する。別系統のエンジンによる判定は判定コマンド(`[judges]`)で扱い、担当と check の実行主体は[ループの雛形](architecture/data.md#ループの雛形-plantemplate)と[計画](architecture/data.md#計画-plan)に従う。計画の作成は[作業の開始](architecture/flow.md#作業の開始)で計画者が担う。

人への問いと回答はセッションのチャットで扱い、CLI が受理して進行を再開する([人の回答と再開](architecture/flow.md#人の回答と再開))。計画の作り直しと切り替えは[計画のバージョン](architecture/data.md#計画のバージョン)に従う。

## リポジトリ側の構成

各リポジトリには、次のものを用意する。

- **指示ファイル(AGENTS.md と CLAUDE.md)**: リポジトリ自身が管理し、kit は触れない。判定の規範としての扱いは[指示ファイルとホスト設定](architecture/layout.md#指示ファイルとホスト設定)に従う。
- **配置ガイド**: 置くもの・置かないもの・配置先を示す。配置と形式は[dev-kit のリポジトリ](architecture/layout.md#dev-kit-のリポジトリ)、brief への取り込みは[作業の指示](architecture/flow.md#作業の指示)に従う。
- **元に戻せないものと外部への書き込み**: 元に戻せないものは[規則と強制手段](architecture/data.md#規則と強制手段-rules)の `.devkit/rules.md` と計画に置く。外部への書き込みは [config](architecture/modules.md#config)に登録し、計画の承認で足りる操作は[計画の宣言](architecture/data.md#計画-plan)で扱う。
- **育てるもの**: 資産の範囲・所有者・git 管理は[所有と生存期間](architecture/data.md#所有と生存期間)に従い、更新とチームでの共有は[資産の更新と共有](architecture/data.md#資産の更新と共有)に従う。
- **規則と強制手段**: 移行記録を含め、[Rules](architecture/data.md#規則と強制手段-rules)に従って `.devkit/rules.md` に置く。
- **設定ファイル**: `devkit.toml` は kit のバージョン、判定コマンド(`[judges]`)、検証コマンド(`[commands]`)、外部操作、deny、分類表の追記を扱う。設定項目は [config](architecture/modules.md#config)、配置は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)、検証コマンドを用いる `command:` の check は[作業結果の判定](architecture/flow.md#作業結果の判定)に従う。
- **実行の記録と生成物**: 配置と git 管理からの除外は[利用側のリポジトリ](architecture/layout.md#利用側のリポジトリ)に従う。保存する実行の記録と、そこから組み立てる [checkpoint](architecture/data.md#checkpoint)は区別する。
- **kit の固定物**: [kit の供給](architecture/modules.md#kit)に従い、`.devkit/kit/<バージョン>/` のコピーから参照する。

導入時に `init` が用意するものは [install](architecture/modules.md#install)、hook の登録先と設定の追記は[権限の境界](architecture/guarantees.md#権限の境界)に従う。バイナリの配置を含む配布方法は[仮決定](architecture/decisions.md#仮決定)に未決定として残す。

導入後の照合は [doctor](architecture/modules.md#doctor-の確認項目)、別セッションや compaction 後の再開の確認は[結合試験](architecture/contract-changes.md#結合試験)に従う。
