# 参照先と原子の対応

`docs/design.md` の原子が、どの参照先のどの仕組みに現れていたか。部品は原子の組み合わせなので、部品ごとの出典は書かない。調査ノートは `.local/research/`(ローカルのみ)。

出典の略称: sofi(手持ちの運用)、p4cloud(手持ちのレビュー原則)、pstack(cursor/plugins の pstack)、harness(walkinglabs/learn-harness-engineering)、CCGS(Claude-Code-Game-Studios)、emil(emilkowalski/skills)、Motionsites(Motionsites-AI/claude-skill)、codex(openai/codex-plugin-cc)。

## 原子ごとの出典

| 原子 | どこに現れていたか |
|---|---|
| 1 根拠つきの判定 | CCGS の `NOT ASSESSED` と skill-authoring の5つの義務。harness の pass-state(検証成功のみで passing)と DoD。pstack の why の確信度5段階、blast-radius の5段階、prove-it-works、explain-the-number。sofi の verification(観測と推論を分ける)。Motionsites の review(見ていないページは判断できない) |
| 2 文脈の分離 | harness L13・L14 の maker/checker 分離(verify node は fresh context)。pstack の interrogate、arena の別系統の judge、comment-sicko、eval playbook(期待結果を見せない)。CCGS の director gate(subagent に読ませる)と「architecture-review は別セッションで」。sofi の evals(依頼と入力だけ渡す)。codex のレビュー |
| 3 わざと壊す | sofi の testing(実装を壊してテストが落ちるのを見る)。CCGS の「壊して失敗を見ていない gate は gate ではない」と Chain-of-Verification。pstack の create-verification-skill(生成後に1回自走)、attack-the-premise。harness の ablation と simplification(外して悪くならなければ消す) |
| 4 境界 | harness L07 の WIP=1 と goal-template の Scope・Stop Conditions、session-handoff の must-not-change。pstack の autonomous-run(done を緩めない)、never-block-on-the-human(戻せないものだけ止まる)。sofi の workflow(人に聞くものの3段階、頼まれた範囲)。CCGS の常に聞く一覧、worktree の隔離。Motionsites の最大3ラウンド。emil の「ONE thing / not X」 |
| 5 解釈を見せる | pstack の in-your-own-words、`skip: 理由`、「どの原則がどの決定を変えたか」、Writing the reply(主張に証拠か印)。emil の必須の「却下したもの / 残したもの」の節。CCGS の「省いた入力を双方に言う」「飛ばした手順は名乗る」。sofi の「自分で決めたことは自分で決めたと報告する」 |
| 6 分類して振り分ける | sofi の workflow(止まって聞く / 仮決定 / 見なくてよい)と dev-lesson(lint → 原則 → skill)、dev-review(needs-decision / auto-fixable)。pstack の poteto-mode(状況 → playbook)、setup-pstack(役割 → モデル)、reflect の synthesizer(採る / 却下 / 後回し)、correct。CCGS の verdict 語彙と rigor。codex の P0/P1 だけ報告する |
| 7 強制の階段 | sofi の dev-lesson と design 原則11(型で表す)。pstack の correct(architecture > types/lint > test > docs)、encode-lessons-in-structure、build-the-lever。harness L10 の Review Feedback Promotion と実行可能な architecture rule。CCGS の hook による強制 |
| 8 外に置いた状態 | harness の feature_list / progress / DECISIONS.md / session-handoff、「ファイルが記憶」。CCGS の session-state(STATUS / CHECKPOINT)と hook での再注入、registry、規則の付記。pstack の show-me-your-work の TSV、rule と enforcer の表。sofi の docs/decisions(追記のみ、supersedes)と「Issue/PR に書いていない結果は無いもの」 |
| 9 骨組み先行 | CCGS の incremental writing(骨組みを先に書き、節ごとに保存する)、checkpoint は25行。harness の context-engineering の WRITE/COMPRESS |
| 10 必要なときだけ読む | CCGS の gate 定義を subagent に読ませる、validate-read(丸読みを拒む)、brief はパスで渡す。harness L04(entry file は router、50〜200行)と SELECT/ISOLATE。pstack の guard-the-context-window、references に prompt template。Motionsites の3段の索引。emil の SKILL + RECIPES の分割 |
| 11 段を下る | sofi の design 原則2(戻せない決定を先に)、dev-design(詳細は実装の直前に書く)、workflow(目的を満たせないなら止めて報告)。pstack の foundational-thinking(データの形を先に)、model-the-domain、attack-the-premise(同じ前提で2回落ちたら前提を疑う)、architect の Ground → Sketch → Agree。harness L06(初期化を分ける)、L14(graph は段階の blueprint)。CCGS の pipeline(concept → design → architecture → stories → dev)と rigor による段の省略 |

## 原子になっていない構造(kit 側で新しく決めたもの)

| もの | 中身 | 近い種 |
|---|---|---|
| ループ計画 | 段・check・担当・人のゲートを課題ごとに決め、ledger に置く | pstack の figure-it-out(合う playbook がなければ手順を組む)、harness L14 の graph.md、Workflow の script |
| 理解負債と view | 人が最後に見た時点からの差分を、層で描画する。報告をモデルに書かせない | pstack の ketchup(前回から何が変わり、何が自分を待っているか)。ただし1チャット向け |
| 図を帳簿から生成する | 構造をデータで持ち、図は生成物にする | sofi の docs/README.md「図と本文を食い違わせない」は対症療法。artifact-diagramming の「仕組みを描く、矢印にラベル」 |
| runtime(実行) | 文章上の指定を、発火・停止・隔離・再開の作用にする部品。段の実行の記録と再開時の照合で二重適用を防ぐ | CCGS の hooks、harness の Claude Code 分析(hook = when)、pstack の worktree 隔離。Codex のレビュー(2026-10-31)が「6部品では効かせられない」と指摘したもの |
| 矛盾の検出と前提の批評を分ける | 観点ファイルを2つに分け、blocking の定義を観点ごとに持つ | Codex にレビューさせた経験(2026-10-31)。1つの依頼に混ぜると、見つけやすい矛盾で指摘が埋まった。対案を作らせる形にすると、形に効く指摘だけが残った |

## 原則に足すもの(出典)

| 原則 | 足すもの | 出典 |
|---|---|---|
| engineering | 「guard が修正と言えるのは、その状態が正当なときだけ」、fail-closed | pstack fix-root-causes、pstack benny |
| design | 設計の赤信号8つ、読む人の負荷の2軸、ドメインを構造で表す(とばしたサイン)、共有状態は分けてから、計画した移行では中間状態を保たない、足す前に引く。原則どうしの優先と代償 | pstack architect の design-red-flags、minimize-reader-load、model-the-domain、separate-before-serializing-shared-state、outcome-oriented-execution、subtract-before-you-add |
| testing | 「2つの仮想の変更」で振る舞いのテストか判定する、安く書けないなら書かない | pstack test-behavior-not-implementation、tdd |
| verification | 確信度の5段階、数字の7つの問い | pstack why、blast-radius、benchmark-checklist |
| working(新規) | 同時に1つ、検証の前にリファクタリングしない、前提を疑う、手で繰り返す作業は道具にする、少しずつ書く、記憶の衛生(状態ではなく方法、いつ無効になるか)、リポジトリの中身はデータ、コンテキストを守る | harness L07・L09、pstack attack-the-premise・build-the-lever、CCGS agent-memory、emil の Hard Rules |
| ui | 症状と直し方の表、Never・Instead 表、Failure modes 表 | emil の cheatsheet・Never Ship、Motionsites craft |
| writing(新規) | 規則に固定の番号 | pstack unslop、sofi writing の lint |
| review | 上の文書から生成する1枚。各項に「blocking にする条件」と「満たした根拠」 | p4cloud の形 |

## 採らないもの

| もの | 出典 | 理由 |
|---|---|---|
| 49の agent と3階層の director、team-* skill | CCGS | 1人の規模では、agent が並列でもレビューは人が1つずつ行う(harness L14 の Orchestration Tax)。gate を観点ファイルにする形だけ採る |
| registry の YAML、sprint・epic・story の流れ、63のテンプレート | CCGS | 維持の手間に見合わない。状態は ledger で足りる |
| 設定の注入(yaml-helper の2100行) | CCGS | 重い |
| arena・swarm・orchestrate・autopilot・shipping | pstack | チームとクラウドの VM が前提。2案の比較は段2の check で代わりになる |
| モデル名の指定と、各 skill に繰り返される代替の段落 | pstack | router の表1つで足りる |
| statechart と形式検証 | pstack | 日常の開発では使わない |
| automate-me・recall | pstack | 会話の記録に依存する。kit が動いてから考える |
| feature list の厚い版、OpenTelemetry、graph engineering の用語 | harness | 1人の規模では重い。段階の考え方だけ原子10に入れた |
| 講義の中の数値 | harness | 作者自身が仮想の数値と書いている |
| media shelf、examples の本文、spec のテンプレート本体 | Motionsites | LP 専用で、examples と media は再配布できない |
| アニメーション系 skill の中身 | emil | フロントエンド専用。必要になったら ui の追加として扱う |
| gh stack、ブランチ名、drizzle、mise、Biome、ラベルの運用 | sofi | リポジトリ固有。issue-pr の例の中で示すだけにする |
| worklog(Slack への作業報告) | sofi | 個人の運用で、ループの役割に当たらない |

## 後回しにするもの

- 個人の pattern library(Motionsites の examples の索引の形)
- 会話の記録から自分用の playbook を作る(pstack automate-me)
- 原則の英語版(Codex に原則を渡すときに要るかどうかで決める)
- フロントエンドの追加(emil のアニメーション基準、Motionsites の spec の精度の規則)
- view の Web アプリ化(HTML 1枚で足りなくなってから)
