# Session Summary Update SOP

主题：第 17 节，把触发、选择、生成和保存组合成滚动摘要更新服务

## 1. 完整行为

```text
Get current summary
-> List messages with id > last_message_id
-> Select evicted prefix before recent window
-> Count tokens and decide trigger
-> Generate(previous summary + evicted prefix)
-> Save(new summary, expected old version)
```

核心代码：

- [update.go](/offline-rag-go-lab/internal/sessionsummary/update.go:1)
- [source_mysql.go](/offline-rag-go-lab/internal/sessionsummary/source_mysql.go:1)
- [main.go](/offline-rag-go-lab/cmd/summary-update-demo/main.go:1)

## 2. 为什么必须按这个顺序

旧 summary 的 `last_message_id` 是查询起点。MySQL 只读取：

```sql
WHERE session_id = ?
  AND user_id = ?
  AND id > ?
ORDER BY id ASC
```

这里按 `id`，不按 `created_at`。消息 ID 是 watermark 的比较单位，升序结果才能安全选择“最老的一段连续前缀”。

只有已离开 recent window 的消息才进入 generator。模型成功还不够，必须等带 expected version 的 `Save` 成功后，watermark 才算提交。

## 3. recent 起点早于 watermark

假设旧 watermark 是 `20`，新消息是 `21,22`，但完整 recent window 从 `19` 开始：

```text
recent: 19,20,21,22
unsummarized: 21,22
evicted unsummarized: empty
```

这时 `21,22` 都仍在 recent window，不能摘要。selector 因此把 `recent_start_id <= watermark` 明确解释为“没有新消息被驱逐”，而不是报“找不到 recent start”。

## 4. Go 编排接口

```go
type MessageSource interface {
    ListAfter(sessionID, userID string, lastMessageID int64) ([]SourceMessage, error)
}

type UpdateRequest struct {
    SessionID       string
    UserID          string
    Model           string
    RecentStartID   int64
    MaxOutputTokens int
}
```

`UpdateService` 只依赖 `SummaryStore`、`MessageSource`、`MessageTokenCounter`、`TriggerDecider` 和 `SummaryGenerator` 五个小接口。单元测试注入 fake，真实命令注入 MySQL、Qwen tokenizer 和 Ollama。

## 5. 失败时的状态

- 未触发：不调用 Ollama，不保存
- tokenizer/selector 失败：不调用 Ollama，不保存
- Ollama 失败：不保存，旧 watermark 保持不变
- MySQL version conflict：返回错误，不覆盖其他请求的摘要
- 保存成功：返回数据库接受的新 version/watermark

发生 version conflict 时不能只重试 `Save`。另一个请求可能已经生成了不同摘要，必须从 `Get current summary` 开始重跑完整流程。

## 6. 真实实践 SOP

命令使用专用 session，并且只有显式传 `--seed` 才写入 6 条教学消息：

```bash
go run ./cmd/summary-update-demo \
  --seed \
  --session-id summary-update-demo \
  --user-id summary-update-user \
  --model qwen:7b \
  --recent-keep 2 \
  --min-messages 4 \
  --min-tokens 100000 \
  --max-output-tokens 256
```

这里故意把 token 阈值设得很高，让 6 条未摘要消息通过 message 阈值触发；最后 2 条保留原文，最老 4 条进入摘要。

预期结构：

```text
Seeded messages: 6
Unsummarized IDs: <六个升序真实 ID>
Evicted IDs: <前四个 ID>
Recent start ID: <第五个 ID>
Decision: message_threshold
Updated: true
Summary version: 1
Summary watermark: <第四个 ID>
Summary content: <qwen 生成内容>
```

ID 是 `recent_chat_messages` 全表自增主键，不要求从 1 开始，也不要求连续。

本次真实结果：消息 IDs 为 `7..12`，evicted 为 `7..10`，recent start 为 `11`，触发原因是 `message_threshold`，保存为 `version=1, watermark=10`。qwen 保留了小黄、Go、真实生产教学、token 自动预算和 MySQL version 乐观锁等事实，但仍添加“更新后的摘要正文如下”引导语，该质量项已记录到优化清单。

随后不带 `--seed` 再运行一次，结果为 `unsummarized=11,12`、evicted 为空、`no_evicted_messages`、`updated=false`，version/watermark 保持 `1/10`，证明未重复写消息且未无意义调用模型。

## 7. 配置

`summary-update-demo`、`summary-store-demo` 共用 [fileconfig.go](/offline-rag-go-lab/internal/fileconfig/fileconfig.go:1)，直接读取被忽略的：

```text
config/recent-chat.env
```

使用键：

```text
RECENT_CHAT_MYSQL_DSN
OLLAMA_BASE_URL
RECENT_CHAT_TOKENIZER_PATH
```

没有把 DSN 写入环境变量、命令参数或 Git。

## 8. 测试

```bash
go test ./internal/sessionsummary ./internal/fileconfig ./cmd/summary-update-demo
```

覆盖不触发、首次摘要、滚动摘要、模型失败、保存冲突、watermark 不推进、MySQL 升序查询和 recent 跨 watermark 边界。

## 9. 本节重点

```text
生成摘要不是提交摘要
只有 version Save 成功，summary 与 watermark 才同时生效
```
