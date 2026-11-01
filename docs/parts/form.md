# form — 固定フォーマット

form は、後続の[ステージ](../stages.md)、hook、人が読み取れる固定フォーマットである。実行前に定義する規格であり、実行に伴って内容が変化する [ledger](ledger.md) とは区別する。skill などの文書を作るときは、[`principles/working.md`](../../principles/working.md) の原則6に従い、構成を先に作る。

- **skill の構成**: frontmatter には `name` / `description` / `argument-hint` / `disable-model-invocation` / `allowed-tools` を置く。description は「何をするか → スコープ → Use when "言い回し" → For X use Y」の順にし、起動トリガーとなる語句を前に置く。本文は、スコープの明示、最悪の失敗、Hard Rules、手順、Never・Instead 表、必須出力、終了ブロックで構成する。責務は1つに絞り、対象外の作業と、それを担当する skill を示す。手順では原則を参照し、書き写さない。必須出力には、却下したものと省略したものを含める。150行以内に収め、参照資料は `references/` に分けて索引から開く。リポジトリ固有の事実は含めない。
- **playbook**: スコープを示す「〜を担当する」という記述、番号付きの手順、末尾の報告項目で構成する。使用時には手順を todo に転記し、実施しない手順には `skip: 理由` を残す。
- **[観点ファイル](../checks.md)**: blocking の定義、確認事項、報告しない事項、出力形式を記載する。
- **終了ブロック**: 判定、次のアクション、変更したファイルを記載する。次のステージと hook が読む。
- **人への確認事項**: 質問、短い名前を付けた選択肢、推奨案と理由、作業を停止して確認する理由（[router](router.md) の分類）、回答で進むステージ、未回答のとき止まる範囲と独立して進められる範囲、根拠へのリンクを示す。コンテキストを再構成しなければ答えられない質問は、設計上の失敗として扱う。
- **PR 本文**: [`principles/writing.md`](../../principles/writing.md) の原則9に従う。形式はリポジトリに合わせて書き換える。
- **原則**: [`principles/README.md`](../../principles/README.md) の「原則のフォーマット」に従う。
