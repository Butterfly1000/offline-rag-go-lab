# Session Summary Store SOP

主题：第 16 节，用 MySQL 持久化摘要并通过 version 防止并发覆盖

## 1. 数据结构

一条 `(session_id, user_id)` 只保存一份当前摘要：

```text
content          当前滚动摘要
last_message_id  已被摘要覆盖到的最后一条消息 ID
version          每次成功保存后加 1
```

建表文件：[session_summaries.sql](/offline-rag-go-lab/sql/session_summaries.sql:1)

核心代码：

- [store.go](/offline-rag-go-lab/internal/sessionsummary/store.go:1)
- [store_mysql.go](/offline-rag-go-lab/internal/sessionsummary/store_mysql.go:1)
- [main.go](/offline-rag-go-lab/cmd/summary-store-demo/main.go:1)

## 2. 第一次保存

调用方还没有读到摘要时，传 `expectedVersion = 0`：

```go
saved, err := store.Save(next, 0)
```

Store 写入 `version = 1`。如果相同 session/user 已被另一个请求抢先插入，MySQL 唯一键返回 `1062`，Store 将它转换成 `ErrVersionConflict`，不会覆盖已有摘要。

## 3. 后续更新

先读取当前记录：

```go
current, exists, err := store.Get(sessionID, userID)
saved, err := store.Save(next, current.Version)
```

核心 SQL 条件：

```sql
WHERE session_id = ?
  AND user_id = ?
  AND version = ?
  AND last_message_id <= ?
```

假设两个进程都读到 `version = 2`，第一个更新后变为 `3`。第二个仍以 `2` 更新，影响行数是 `0`，因此收到 `ErrVersionConflict`，必须重新执行“读取消息、生成摘要、保存”的完整流程。

`last_message_id <= next watermark` 是数据库中的第二道保护。业务代码会先拒绝回退，SQL 条件还会防止检查与写入之间发生的并发变化。

## 4. 配置

真实 DSN 只写在被 `.gitignore` 排除的本地文件：

```text
config/recent-chat.env
```

格式参考：

```text
RECENT_CHAT_MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/offline_rag?parseTime=true
```

`parseTime=true` 让 MySQL driver 把 `TIMESTAMP` 扫描成 `time.Time`。demo 直接读取该文件，不依赖进程环境变量，也不要求把密码写进命令行。

## 5. 实践 SOP

真实实践会写入专用键，不使用聊天中的 `s-001/u-001`：

```bash
go run ./cmd/summary-store-demo \
  --apply-schema \
  --session-id summary-store-demo \
  --user-id summary-store-user \
  --content '第一版：用户偏好真实落地。' \
  --watermark 20
```

第一次预期：

```text
Applied schema: sql/session_summaries.sql
Existed before save: false
Expected version: 0
Saved version: 1
Saved watermark: 20
```

第二次执行：

```bash
go run ./cmd/summary-store-demo \
  --session-id summary-store-demo \
  --user-id summary-store-user \
  --content '第二版：用户偏好真实落地，代码示例使用 Go。' \
  --watermark 24
```

第二次预期：

```text
Existed before save: true
Expected version: 1
Saved version: 2
Saved watermark: 24
```

本次真实执行结果与预期一致：第一次 `exists=false, expected=0, saved=1, watermark=20`；第二次 `exists=true, expected=1, saved=2, watermark=24`。两次都只使用专用 demo 键。

确认数据库：

```sql
SELECT session_id, user_id, content, last_message_id, version
FROM session_summaries
WHERE session_id = 'summary-store-demo'
  AND user_id = 'summary-store-user';
```

## 6. 测试

```bash
go test ./internal/sessionsummary ./cmd/summary-store-demo
```

测试不连接真实 MySQL：fake queries 验证业务行为，fake SQL 验证查询集中在 adapter 且更新包含 version/watermark 条件。

## 7. 本节重点

```text
version 保护“没有覆盖别人刚生成的摘要”
watermark 保护“没有遗忘已经摘要过的消息范围”
```

只有 `Save` 成功后，系统才可以把新 watermark 当成已提交状态。
