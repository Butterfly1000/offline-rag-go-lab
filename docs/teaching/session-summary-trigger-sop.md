# Session Summary Trigger SOP

主题：第 13 节，摘要保存什么，以及什么时候应该生成摘要

## 1. 这一节解决什么问题

token budget 已经能安全地裁剪 recent window，但它没有回答：

```text
哪些旧消息需要进入摘要？
什么时候值得调用一次模型？
上次摘要已经覆盖到哪里？
```

第 13 节建立三个基础：

1. `SessionSummary` 数据结构
2. `last_message_id` 摘要水位
3. “驱逐前置 + message/token 双阈值”触发策略

本节不生成摘要。它先把生产系统中的触发行为变成可测试、可解释的代码。

## 2. 代码与 SQL 在哪里

- [types.go](/offline-rag-go-lab/internal/sessionsummary/types.go:1)
- [trigger.go](/offline-rag-go-lab/internal/sessionsummary/trigger.go:1)
- [trigger_test.go](/offline-rag-go-lab/internal/sessionsummary/trigger_test.go:1)
- [main.go](/offline-rag-go-lab/cmd/summary-trigger-demo/main.go:1)
- [session_summaries.sql](/offline-rag-go-lab/sql/session_summaries.sql:1)

## 3. Summary 数据结构

```go
type SessionSummary struct {
    SessionID     string
    UserID        string
    Content       string
    LastMessageID int64
    Version       int64
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### Part 1：`Content`

保存当前 session 的滚动摘要，例如：

```text
用户叫小黄；代码示例使用 Go；当前已完成 token 自动预算；
下一步实现 session summary；用户要求真实落地而不是模拟。
```

它不是聊天全文，而是后续仍然有效的状态和结论。

### Part 2：`LastMessageID`

假设摘要已经覆盖消息 `1` 到 `20`：

```text
last_message_id = 20
```

下一次更新摘要时只选择：

```sql
WHERE session_id = ?
  AND id > 20
```

这叫 watermark，也就是处理水位。它防止每次重新摘要整个会话。

这里依赖 message 表的自增 `id`，不是依赖时间。原因是同一秒可能写入多条消息，而 `id` 仍能给出稳定先后顺序。

### Part 3：`Version`

每次摘要成功更新后版本加一：

```text
version 1 -> version 2 -> version 3
```

后续可使用 version 做乐观并发控制，避免两个请求同时更新摘要时互相覆盖。本节只定义字段，还没有实现并发 upsert。

## 4. MySQL 表怎么设计

建表文件：

```sql
CREATE TABLE IF NOT EXISTS session_summaries (
    session_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    last_message_id BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (session_id, user_id),
    INDEX idx_session_summaries_user_updated (user_id, updated_at)
);
```

主键 `(session_id, user_id)` 表示：一个用户的一个 session 只有一条当前滚动摘要。

`CREATE TABLE IF NOT EXISTS` 让建表命令可重复执行；它不会自动修改已存在表的字段结构，因此后续 schema 变更仍应使用正式 migration。

### 本节不自动执行 SQL

先查看：

```bash
sed -n '1,200p' sql/session_summaries.sql
```

确认后由你手动执行：

```bash
mysql -u root -p offline_rag < sql/session_summaries.sql
```

执行后验证：

```sql
SHOW CREATE TABLE session_summaries;
```

实现本节时没有执行这些 SQL，因此当前数据库不会因为代码实现自动变化。

## 5. 三个触发输入

```go
type TriggerInput struct {
    UnsummarizedMessages int
    UnsummarizedTokens   int
    EvictedMessages      int
}
```

### `UnsummarizedMessages`

`last_message_id` 之后一共有多少条新消息。

例如水位是 `20`，现在最新消息 ID 是 `28`，且中间没有删除：

```text
unsummarized_messages = 8
```

生产查询应按真实结果计数，不能直接假设 `28 - 20 = 8`，因为消息可能被清理或迁移。

### `UnsummarizedTokens`

所有未摘要消息经过当前模型匹配 tokenizer 后的总 token 数。

为什么同时看 token：

- 8 条短消息可能很小
- 1 条超长代码消息可能很大

只看 message 数会漏掉“条数少但容量大”的情况。

### `EvictedMessages`

未摘要消息中，已经不在本轮 recent window 的消息数。

它是触发的前置条件：

```text
evicted_messages == 0
-> 不生成摘要
```

因为原文仍完整保留在 recent window 时，立即摘要只会重复信息并增加模型调用成本。

## 6. 触发规则

默认教学阈值：

```text
min_messages = 8
min_tokens = 2048
```

代码顺序：

```text
先校验三个输入
-> 没有驱逐：不触发
-> 两个阈值都达到：触发
-> message 阈值达到：触发
-> token 阈值达到：触发
-> 否则不触发
```

message 和 token 使用 OR：

```text
messages >= min_messages
OR
tokens >= min_tokens
```

这样既能处理很多短消息，也能处理少量超长消息。

## 7. Go 代码怎么实现

### Part 1：policy 构造时校验配置

```go
func NewTriggerPolicy(minMessages int, minTokens int) (TriggerPolicy, error) {
    if minMessages <= 0 {
        return TriggerPolicy{}, fmt.Errorf(...)
    }
    if minTokens <= 0 {
        return TriggerPolicy{}, fmt.Errorf(...)
    }
    return TriggerPolicy{...}, nil
}
```

阈值是配置错误，不应等到运行很多次后才发现，所以在构造 policy 时立即拒绝。

### Part 2：`switch` 表达互斥决策

```go
switch {
case messagesReached && tokensReached:
    return TriggerDecision{true, ReasonBothThresholds}, nil
case messagesReached:
    return TriggerDecision{true, ReasonMessageThreshold}, nil
case tokensReached:
    return TriggerDecision{true, ReasonTokenThreshold}, nil
default:
    return TriggerDecision{false, ReasonBelowThreshold}, nil
}
```

Go 的无表达式 `switch` 会从上到下执行第一个为 `true` 的 case。因此两个阈值必须放在单独阈值之前，否则 `both_thresholds` 永远无法返回。

### Part 3：为什么返回 reason

只返回 `bool`，生产排查时只能看到“触发了”，却不知道原因。

当前 reason 包括：

```text
no_evicted_messages
both_thresholds
message_threshold
token_threshold
below_threshold
```

后续可按 reason 统计触发分布，再调整阈值，而不是凭感觉修改配置。

## 8. 实践 SOP

### SOP 1：消息很多，但没有驱逐

```bash
go run ./cmd/summary-trigger-demo \
  --messages 10 \
  --tokens 5000 \
  --evicted 0
```

结果：

```text
Should summarize: false
Reason: no_evicted_messages
```

即使两个阈值都达到，只要原文仍在 recent window，就不触发。

### SOP 2：message 阈值触发

```bash
go run ./cmd/summary-trigger-demo \
  --messages 8 \
  --tokens 1000 \
  --evicted 2
```

结果：

```text
Should summarize: true
Reason: message_threshold
```

### SOP 3：token 阈值触发

```bash
go run ./cmd/summary-trigger-demo \
  --messages 3 \
  --tokens 3000 \
  --evicted 1
```

结果：

```text
Should summarize: true
Reason: token_threshold
```

### SOP 4：有驱逐，但仍低于阈值

```bash
go run ./cmd/summary-trigger-demo \
  --messages 3 \
  --tokens 1000 \
  --evicted 1
```

结果：

```text
Should summarize: false
Reason: below_threshold
```

### SOP 5：验证非法统计

```bash
go run ./cmd/summary-trigger-demo \
  --messages 2 \
  --tokens 1000 \
  --evicted 3
```

命令以非零状态退出：

```text
evicted messages (3) cannot exceed unsummarized messages (2)
```

### SOP 6：运行测试

```bash
go test ./internal/sessionsummary
```

测试覆盖五种决策和非法 policy/input。

## 9. 当前实现与生产落地的距离

已经完成：

- summary record 和 watermark 定义
- MySQL schema
- 可配置、确定性、可解释的触发 policy
- 命令和自动测试

尚未完成：

- 从 MySQL 查 `last_message_id` 之后的消息
- 判断哪些消息已经离开 recent window
- 用 tokenizer 计算真实 unsummarized token
- 调 Ollama 生成增量摘要
- 原子更新 content、watermark 和 version
- 把 summary 拼入下一轮 `/chat`

本节不是“只有 demo”。`TriggerPolicy` 是无外部依赖的生产核心，后续 service 直接调用它；命令只是让同一逻辑可以独立观察。

## 10. 下一节

下一节实现：

```text
读取旧 summary
+ 选择 watermark 之后且已被驱逐的消息
-> 构造增量摘要 prompt
-> 真实调用 Ollama
-> 得到 new summary
```

MySQL upsert 可以和生成分开讲，避免一次把读取、生成、并发写入混在一起。

## 11. 本节重点

```text
是否摘要
= 已经有原文离开 recent window
AND
(未摘要消息数达到阈值 OR 未摘要 token 达到阈值)
```

`last_message_id` 负责回答“上次处理到哪”，TriggerPolicy 负责回答“这次值不值得处理”。

## 12. 实现验证记录

- RED：目标测试因 `TriggerPolicy`、`TriggerInput` 和 reason 常量不存在而编译失败
- GREEN：`go test ./internal/sessionsummary` 通过
- 并发检查：`go test -race ./internal/sessionsummary` 通过
- 回归测试：`go test ./...` 通过
- 静态检查：`go vet ./...` 通过
- 命令构建：`go build ./cmd/...` 通过
- 四个实践场景返回预期 reason，非法统计以非零状态退出
- SQL 没有执行，MySQL、Ollama 和 Qdrant 状态均未修改
- Review 修正：`last_message_id/version` 改为与 Go `int64` 和现有 message ID 一致的有符号 `BIGINT`
- 最终 Review：未发现其他 Critical 或 Important 问题
