# L16：安全保存一份滚动摘要

对应工程课：[Summary 小节 16](../session-summary-store-sop.md)。本课将摘要写入 MySQL，并用主键、watermark 和 version 防止写错对象或悄悄覆盖并发更新。

## 1. 本课一句话

`session_summaries` 以 `(session_id,user_id)` 为主键；首次保存 insert 为 version 1，后续以 expected version 条件 update，冲突时拒绝覆盖且不自动重试。

## 2. 先用人话理解

每个用户在每个会话只有一本当前笔记。第一次放入书架时编号为第 1 版；以后改笔记时必须说“我基于第几版修改”。如果别人已先改过，书架管理员拒绝旧稿，而不是猜测怎么合并。

## 3. 系统里有什么

- 表 `session_summaries`：主键 `(session_id, user_id)`，保存 content、`last_message_id`、version 与时间戳。
- `SummaryStore.Get`：按主键读取当前摘要。
- `SummaryStore.Save(next, expectedVersion)`：`expectedVersion=0` 走首次 insert，正数走更新。
- `ErrVersionConflict` 与 `ErrWatermarkRegression`：分别表示并发/对象变化和水位倒退。

## 4. 一条完整链路

L15 生成非空摘要与新 watermark → 读取当前 `(session,user)` → 不存在时以 expected version 0 insert，固定保存 version 1 → 存在时检查当前 version 和 watermark → SQL `UPDATE ... WHERE version=? AND last_message_id<=?` 成功影响恰好一行才保存 version+1 → 0 行或重复主键都返回冲突。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/sessionsummary -run 'Test(Store|MySQLSummary)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/summary-store-demo --config /absolute/path/to/your/recent-chat.env --session-id summary-store-demo-001 --user-id summary-store-user-001 --content '用户偏好真实落地，代码示例使用 Go。' --watermark 24
```

- test 是纯本地 fake SQL 验证，不需 MySQL、配置或 Ollama，不写业务数据；只可能写 build cache，可安全重复。
- demo 需要可访问 MySQL、已存在 `session_summaries` 表，以及由 `config/recent-chat.env.example` 派生并妥善保管的私有配置文件；它会先读、再对该精确 `(session_id,user_id)` insert 或 update，是业务写入。首次示例插入；重复运行会更新同一行、版本递增，故不是幂等。恢复应先查询该精确主键，确认后再由操作者删除或修正这条专用 demo 记录；本课不自动执行恢复动作。

## 6. 结果怎么看

首次使用 `summary-store-demo-001` / `summary-store-user-001`、watermark 24 时，输出应显示 `Existed before save: false`、`Expected version: 0`、`Saved version: 1`、`Saved watermark: 24`。第二次同键运行会先读到 version 1，再以 expected 1 成功更新为 version 2。

## 7. 算一遍或走一遍

现有摘要为 `(session=s-7,user=u-2,watermark=23,version=4)`。新内容覆盖到消息 ID 30，调用 `Save(next, 4)`：检查 `30 >= 23`，SQL 条件命中一行，保存 version `5`。若另一个写者已先保存 version 5，本次 `expected=4` 的 update 影响 0 行，返回 version conflict；若新 watermark 为 20，则先返回 watermark regression。两种情况都不覆盖、不自动重试。

## 8. 常见误解

“同一个 session 就是一条摘要”不完整，user 也是主键组成部分。“遇到冲突重试 update”在这里不安全：重试必须重新读取、重新选择和重新生成，不能把旧摘要盲写回去。“`CREATE TABLE IF NOT EXISTS` 等于 schema migration”也错误，它只可重复创建缺失表，不会修复既有表结构。

## 9. 当前实现与生产边界

已保证：session/user/content 非空、水位为正、expected version 非负；首次 insert 的重复键、更新的 version 不匹配或 affected rows 非 1 都拒绝。未保证：自动合并两个并发模型摘要、自动重试、自动删除演示数据或替用户管理凭据。若需创建缺失表，`--apply-schema` 会执行 `sql/session_summaries.sql` 的写 SQL；该 `CREATE TABLE IF NOT EXISTS` 可重复，但生产 schema 变更应走正式 migration。

## 10. 面试怎么说

滚动摘要按 `(session_id,user_id)` 隔离。创建用 expected version 0 得到 version 1；更新把 expected version 和 watermark 都放进条件，只有一行受影响才成功。冲突 fail closed，要求上层重新读取并重新生成，而不是自动覆盖。

## 11. 自检题与答案

问：为什么第二次 demo 不是幂等？答：它会将同一行 version 从 1 更新到 2。问：expected version 4、数据库已为 5 时怎么办？答：拒绝为 version conflict，不自动重试。问：能否用 `--apply-schema` 修复所有旧表结构？答：不能，它不是 migration。

## 12. 事实锚点

- 原课：[session-summary-store-sop.md](../session-summary-store-sop.md)
- Go 组件：`internal/sessionsummary/{store,store_mysql}.go`
- schema：`sql/session_summaries.sql`
- 演示：`cmd/summary-store-demo/main.go`
- 前置：[L15](L15-summary-generation.md)
