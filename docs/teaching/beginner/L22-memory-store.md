# L22：在一笔 MySQL 事务中保存事实和证据

对应工程课：[Memory 小节 22](../memory-item-store-sop.md)。本课把 L21 的决策真正落到 MySQL，同时保留每条事实的原始用户证据。

## 1. 本课一句话

`memory_items` 保存当前事实，`memory_item_evidence` 保存来源；commit 之前的 item/evidence 操作失败可由同一 MySQL 事务 rollback，commit 成功后两者才一起成为已提交状态。

## 2. 先用人话理解

事实表像档案卡的当前内容，证据表像卡片背后的引文。只改档案卡却没留下引文，或只留下引文却没改卡片，都会让审计链断裂；它们必须一起盖章。

## 3. 系统里有什么

- `memory_items`：以 `(user_id,kind,memory_key)` 唯一定位当前 value、status、version。
- `memory_item_evidence`：保存 item ID、用户、来源 session/message、operation 与原文；其唯一键避免同一证据重复写入。
- `MemoryStore.Apply`：校验候选、锁定 item、resolve、持久化 item、写 evidence、commit。
- MySQL 是事实源；Qdrant 是可重建的派生索引，见 [L23](L23-memory-qdrant.md)。

## 4. 一条完整链路

先在事务外运行 L19 校验 → `BEGIN` → `SELECT ... FOR UPDATE` 按 user/kind/key 锁当前 item → L21 Resolve → INSERT/UPDATE/FORGET/NOOP → 为每条来源写 evidence → `COMMIT`。commit 前 item/evidence 操作失败时，defer 可尝试 rollback；已有 item 的 NOOP 仍可写新的有效 evidence，但“缺失 item 的 forget”是 NOOP 且没有 item ID，因此不会写 evidence。若 `Commit` 本身返回错误，提交是否已被服务器接受可能未知：函数只能返回失败，后续需核查实际状态并用幂等恢复，而不能承诺 defer rollback 已撤回。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/memoryitem ./cmd/memory-store-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-store-demo --config /absolute/path/to/your/recent-chat.env --apply-schema
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-store-demo --config /absolute/path/to/your/recent-chat.env
```

- test 使用 fake transaction，纯本地、不需 MySQL/Ollama/Qdrant，不写业务数据；只写可重建 build cache，可安全重复。
- demo 需要 MySQL、已存在的 `recent_chat_messages` 表，以及由 `config/recent-chat.env.example` 派生的私有配置。`--apply-schema` 只创建 memory 的 `memory_items`/`memory_item_evidence` 两表，不创建 recent-chat 消息表。首次带 flag 会执行幂等 DDL，并在 source messages 为 0 时先 seed 6 条，再检查 item/evidence fixture。完整终态后的真正只读验收应省略该 flag，使用第三条命令；若仍带 `--apply-schema`，业务 fixture 不新增，但仍会向 MySQL 发送 DDL，不能称为只读。

## 6. 结果怎么看

一条 `implementation_language=Go` 首次为 INSERT version 1；再次以新用户证据确认 Go 是 NOOP，version 仍为 1，但 evidence 数增加。值改 Rust 再改 Go 是 UPDATE；`temporary_tool` 的明确 forget 改 status 为 forgotten，`ListActive` 不再返回它。

## 7. 算一遍或走一遍

当前 `(u-1,project_fact,implementation_language)` 为 `Go,version=2`。新 candidate “仍使用 Go” resolve 为 NOOP，item 保持 version 2，但 source ID 301 可新增一行 evidence；重复同一 source ID/operation 命中 evidence 唯一键，`affected=0` 而非错误。若 candidate 改为 Rust，UPDATE 条件要求 `version=2`，成功后才是 version 3；受影响行不是 1 则冲突失败。

## 8. 常见误解

“NOOP 不需要事务”错误：仍可能写 evidence。“`FOR UPDATE` 与 version 二选一”错误：前者串行化当前读取，后者是最终条件保护，唯一键还处理并发首次 INSERT。“Commit 返回错误就一定 rollback 成功”也错误：提交结果可能未知，defer 只会尝试 rollback，不能保证撤回；应返回失败并核查/幂等恢复。“把 item commit 后再写 evidence”错误：evidence 失败会留下无来源事实。

## 9. 当前实现与生产边界

已保证：commit 前的 item/evidence 操作在同一 MySQL transaction，所有 identity 查询携带 user_id，锁定/版本/唯一键共同防并发覆盖。demo 的局限是 inspection 先看 source messages：source 为 0 时会先 seed 6 条；即使 item/evidence 已终态也可能随后判定 complete 成功退出，新 source message IDs 与旧 evidence 引用可能不一致。因此并非所有“部分状态”都会报错，不能把该 demo 当成通用修复器。未保证：Commit 返回错误后的已提交性，或 MySQL 与 Qdrant 存在同一笔事务；需核查/幂等恢复与 outbox/worker、重放和对账，不能假装同步调用天然原子。

## 10. 面试怎么说

我把长期事实和原始证据分表，并在同一 MySQL 事务中先锁定 identity、确定性 resolve、写 item、再写 evidence。commit 前失败可 rollback；Commit 返回错则结果可能未知，必须返回失败并核查/幂等恢复。NOOP 不变更事实版本却能补充证据；向量库不参与这个事务，因此要用 outbox/重建处理跨库一致性。

## 11. 自检题与答案

问：已有 active item 的 NOOP 能写 evidence 吗？答：能。问：Commit 返回错误时能断言数据库已 rollback 吗？答：不能，结果可能未知，应核查后做幂等恢复。问：`--apply-schema` 会创建 recent_chat_messages 吗？答：不会。问：Qdrant 写失败会自动回滚已提交 MySQL 吗？答：不会，二者没有分布式事务。

## 12. 事实锚点

- 原课：[memory-item-store-sop.md](../memory-item-store-sop.md)
- Go 组件：`internal/memoryitem/{store,store_mysql}.go`
- schema：`sql/memory_items.sql`
- 演示：`cmd/memory-store-demo/main.go`
- 后续：[L23](L23-memory-qdrant.md)
