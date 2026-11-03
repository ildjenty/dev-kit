form が扱う固定フォーマットのテンプレートを置く。
テンプレートの適用と Markdown の描画は internal/form、HTML の生成は internal/view に置く。
配置は [kit](../../docs/architecture/modules.md#kit)、形式は [form](../../docs/architecture/modules.md#form) に従う。

`text/template` の構文を使い、読み込んだ文面と次の入力を `form.ApplyTemplate` に渡す。

- `skill.md.tmpl`: `form.SkillTemplate`。目的・入力・手順・出力・参照から skill を構成する。
- `aspect.md.tmpl`: `form.AspectTemplate`。確認事項・blocking・報告しない事項・出力形式から観点ファイルを構成する。
- `pr.md.tmpl`: `form.PRTemplate`。変更範囲・計画の要約・決定・何をどう確認したか・未確認事項とリスク・参照を載せる。
- `design.md.tmpl`: `form.DesignTemplate`。提案・制約・元に戻せないもの・比較した案・検証方法を載せる。

PR の既存テンプレートがあるリポジトリでは、その文面を渡して見出しを維持する。設計案の「元に戻せないもの」には、計画が持つ一覧への参照と必要な更新内容を渡す。

固定フォーマットの値は `form.RenderMarkdown` で描画する。問いには自由な言葉での回答、選択肢にない回答、条件付きの回答、質問返し、回答の訂正を案内する。画面のデザインの承認を求める場合は、問いの `screen_id` と表示内容を含むツリーの承認対象を利用側で設定する。
