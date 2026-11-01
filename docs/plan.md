# 初版の計画

## 初版の範囲

初版で実装する範囲を先に決める。それ以外は、語彙として文書に記載するにとどめる。

| 対象 | 初版で実装するもの | 後回しにするもの |
|---|---|---|
| 部品 | brief、check、router、ledger、および runtime の hook の部分。 | form のテンプレート集、view、runtime の workflow と再開。 |
| 記録 | checkpoint、決定、check の結果。 | 計画、ステージの実行、構造、規則と強制手段、カーソル、成果物、記録範囲の照合。 |
| 観点 | 構造、差分、動作検証、終了時の状態。 | その他の観点。 |
| ステージ | 調査、構造、実装。 | その他のステージ。 |
| router の表 | 人への確認、check の結果。 | その他の表。 |
| 組み立て | minimal。 | issue-pr。 |

## 順序

1. `principles/` を書く。form の「原則」の形式に揃え、必要な追加と弱点の修正を含める。
2. 初版で使う3種類の記録、checkpoint・決定・check の結果の形式を決める。check の結果には対象の版を含める。
3. check と router を作る。観点ファイルは、初版の4つを用意する。
4. runtime の hook を作る。hook は SessionStart、Stop、Read、SubagentStop を対象とする。
5. `assemblies/minimal/` を組み立て、このリポジトリ自身の開発に使う。必要な部品が足りなければ、部品の設計が誤っていると捉える。この段階で、ledger にどのような情報が蓄積されるかを確認する。
6. view の初版を、1枚の HTML として作る。冒頭の固定形式と要対応から始め、残りの群は5で蓄積された情報を見て加える。Web アプリ化は、ledger の形式と view の仕様が固まってから行う。
7. `plan/` に、計画を作る skill、計画の形式、観点「計画」を用意する。runtime の workflow と再開、および残りの記録も作る。
8. `assemblies/issue-pr/`、振り返り、harness の検査、skill の試験、`tools/` を作る。

kit 全体を先に完成させようとせず、部品を1つ作るごとに試験まで用意してから、次の部品に進む。

## 後回し

- 個人の pattern library（Motionsites の examples の索引の形）
- 会話の記録から自分用の playbook を作る（pstack automate-me）
- 原則の英語版（Codex に原則を渡すときに要るかどうかで決める）
- フロントエンドの追加（emil のアニメーション基準、Motionsites の spec の精度の規則）
- view の Web アプリ化（HTML 1枚で足りなくなってから）
