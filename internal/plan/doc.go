// plan は入出力を伴わない計画の意味論を扱い、検証・逸脱の計算・遷移・上限判定・作り直しを担う。
// 実行可能な計画と承認で足りる外部操作の照合も扱い、保存は ledger、進行は engine に置く。
// 他のモジュールに依存しない。
// 正本: docs/architecture/modules.md#plan
package plan
