ホスト設定、判定者・計画者のサブエージェント定義、rules の断片を置く。
devkit の進め方は kit/skills/、イベント JSON の変換処理は internal/host/event に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、導入は [install](../../docs/architecture/modules.md#install) に従う。

- `claude/settings.json` は `.claude/settings.json` に追記する hook・sandbox・permissions の断片。
- `claude/agents/` は `.claude/agents/` に置く読み取り用の判定者・計画者の定義。
- `codex/hooks.json` と `codex/config.toml` は `.codex/` の同名ファイルに追記する断片。
- `codex/agents/` と `codex/rules/` は `.codex/` の同名ディレクトリに置く定義。

hook の `devkit hook <event>` は、導入時に kit のバイナリを登録するためのコマンドである。既存の項目を上書きせず、導入側が差分を示して追記する。リポジトリ全体の deny は記録・結果への Edit の2項目だけとし、作業に束縛された操作とサブエージェントの制限は PreToolUse に任せる。
