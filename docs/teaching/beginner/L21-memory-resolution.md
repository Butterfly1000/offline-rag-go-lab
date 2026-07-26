# L21：用确定性规则决定记忆生命周期

对应工程课：[Memory 小节 21](../memory-item-resolution-sop.md)。本课只根据 validated candidate 和当前 item 决定动作，不把数据库动作继续交给模型。

## 1. 本课一句话

resolver 的结果只有 INSERT、UPDATE、NOOP、FORGET：值等价不改版本，forget 不存在项是 NOOP，新的 upsert 可恢复 forgotten item。

## 2. 先用人话理解

模型只递来“我认为该记什么”的便签；resolver 像档案管理员，根据现有档案决定新建、替换、不动或标记不再使用。相同输入总要得到相同决定，不能让模型每次重新猜数据库动作。

## 3. 系统里有什么

- `Resolve(current, candidate)`：单条确定性决策。
- action：`insert`、`update`、`noop`、`forget`。
- `Item.Status`：`active` 或 `forgotten`；版本号随状态/值的真实变化递增。
- `ResolveBatch`：按最小 source message ID 稳定排序，避免 map 顺序影响结果。

## 4. 一条完整链路

L19/L20 的 validated candidate + 当前相同 `(kind,key)` item → 再规范化 operation/kind/key/value → 比较 identity/status/value → 输出 Decision。不存在 + upsert 是 INSERT version 1；active 且等价是 NOOP；值变更是 UPDATE；active + forget 是 FORGET；forgotten + upsert 是 UPDATE/恢复；不存在或已 forgotten + forget 是 NOOP。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-resolve-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/memoryitem -run TestResolve -count=1
```

- 两条都是纯本地确定性计算：依赖 Go 与项目依赖，不调用 Ollama/MySQL/Qdrant 或配置，不写业务数据；只可能写 build cache，可安全重复。
- demo 依次展示 Go → ` go ` → Rust → forget → Go 的五个决策；它只在内存中模拟 MySQL 分配 ID，不保存任何 item 或 evidence。

## 6. 结果怎么看

demo 应依次得到：INSERT version 1、NOOP version 1、UPDATE version 2、FORGET version 3（status forgotten）、UPDATE version 4（恢复 active）。没有 candidate 不意味着 forget；真正的 forget 还必须已在 L19 得到明确用户证据。

## 7. 算一遍或走一遍

当前 active `project_fact/implementation_language=Go, version=2`。upsert value `  go  ` 仅经 trim、连续空白折叠与大小写忽略后等价，因此 NOOP、仍为 version 2；upsert `Rust` 则 UPDATE 到 version 3。随后 forget 变为 forgotten version 4；再 upsert `Go` 是恢复 UPDATE version 5。不存在项上的 forget 不创建 tombstone，返回 NOOP。

## 8. 常见误解

“Go”和“Golang”必然等价”错误：当前等价规则只有 trim、空白归一和大小写忽略，不做语义同义判断。confidence 目前不参与 resolver 的 action/版本分支；它已由前置校验保证范围，但不是“高于某值才更新”的阈值。FORGET 也不等于物理擦除，它表示停止召回，审计/evidence/备份清理是另一套流程。

## 9. 当前实现与生产边界

已保证：kind/key 不匹配、非法当前 status/version、空 upsert value和无 source ID 会失败；missing forget 与重复 forget 都安全 NOOP，forgotten upsert 可恢复。未保证：MySQL 事务、`SELECT ... FOR UPDATE`、version conflict 和 item/evidence 原子提交；这些由后续 store 层处理，resolver 本身不写 SQL。

## 10. 面试怎么说

模型不决定 INSERT/UPDATE/FORGET。我将 validated candidate 与当前 item 输入确定性 resolver：只对文本空白/大小写做等价规范，不假装理解同义词；NOOP 不增版本，显式 forget 改状态，后续 upsert 可恢复。

## 11. 自检题与答案

问：active `Go` 和候选 ` go ` 会更新吗？答：不会，是 NOOP。问：不存在 item 的 forget 会创建 tombstone 吗？答：不会，NOOP。问：confidence 0.99 会让 resolver 更倾向 UPDATE 吗？答：不会，当前 confidence 不参与该决策。

## 12. 事实锚点

- 原课：[memory-item-resolution-sop.md](../memory-item-resolution-sop.md)
- Go 组件：`internal/memoryitem/resolve.go`
- 演示：`cmd/memory-resolve-demo/main.go`
- 测试：`internal/memoryitem/resolve_test.go`
- 前置：[L19](L19-memory-candidate-validation.md)、[L20](L20-memory-extraction.md)
