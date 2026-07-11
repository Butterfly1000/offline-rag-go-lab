# Recent Window Runtime SOP

主题：`recent-chat` 第一层真实运行验证手册

这份文档不是实现设计文档，而是运行验证手册。目标是让你在本地按固定顺序验证：

`MySQL messages -> recent window -> Ollama -> /chat response`

---

## 1. 前置条件

执行下面的 SOP 之前，先确认这些条件已经满足：

- Ollama 已启动
- 本地可用模型是 `qwen:7b`
- MySQL 数据库和表已经准备好
- `recent-chat` 服务已经能启动
- `config/recent-chat.env` 已配置

当前 `recent-chat` 入口代码在：

- [cmd/recent-chat/main.go](/offline-rag-go-lab/cmd/recent-chat/main.go:1)

核心业务链路在：

- [internal/recentchat/service.go](/offline-rag-go-lab/internal/recentchat/service.go:1)
- [internal/recentchat/store_mysql.go](/offline-rag-go-lab/internal/recentchat/store_mysql.go:1)
- [internal/recentchat/window_count.go](/offline-rag-go-lab/internal/recentchat/window_count.go:1)
- [internal/recentchat/http.go](/offline-rag-go-lab/internal/recentchat/http.go:1)

---

## 2. SOP 6：第一轮请求

### 2.1 执行命令

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"我叫小黄，这个项目是 Go 写的。",
    "model":"qwen:7b",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

### 2.2 预期结果

第一轮请求时，这个 `session_id` 还没有历史消息，所以正常预期是：

- `answer` 有内容
- `used_messages = 0`
- `recent_window = []`

一个典型返回会像这样：

```json
{
  "answer": "...",
  "used_messages": 0,
  "session_id": "s-001",
  "model": "qwen:7b",
  "recent_window": []
}
```

### 2.3 代码里发生了什么

HTTP 入口在：

- [http.go](/offline-rag-go-lab/internal/recentchat/http.go:24)

真正业务逻辑在：

- [service.go](/offline-rag-go-lab/internal/recentchat/service.go:15)

这轮会依次发生：

1. `req.Validate()` 校验输入
2. `s.store.ListRecentBySession(req.SessionID, req.RecentLimit)` 去 MySQL 查最近消息
3. `s.window.Build(recent, req.RecentLimit)` 截 recent window
4. 组装 `ollamaMessages`
5. `s.ollama.Chat(...)` 调 Ollama
6. 写回 user/assistant 消息到 MySQL

因为这是第一轮，查 recent messages 时数据库里还没有 `s-001` 的历史，所以：

- `recent = []`
- `selected = []`
- 最终 `used_messages = 0`

这一步不是异常，而是第一层 recent window 的正常起始状态。

---

## 3. SOP 7：检查 MySQL 写回

### 3.1 执行 SQL

```sql
USE offline_rag;
SELECT id, session_id, user_id, role, content, created_at
FROM recent_chat_messages
WHERE session_id = 's-001'
ORDER BY id ASC;
```

### 3.2 预期结果

如果第一轮请求成功，表里应该至少有两条记录：

1. 一条 `role = user`
2. 一条 `role = assistant`

### 3.3 代码里发生了什么

写回逻辑在：

- [service.go](/offline-rag-go-lab/internal/recentchat/service.go:65)

当请求里这两个开关都是 `true` 时：

```json
"store_user_turn": true,
"store_assistant_turn": true
```

代码会执行两次 `Append(...)`：

```go
if req.StoreUserTurn {
    s.store.Append(...)
}
if req.StoreAssistTurn {
    s.store.Append(...)
}
```

真实 SQL 插入在：

- [store_mysql.go](/offline-rag-go-lab/internal/recentchat/store_mysql.go:58)

也就是说，第一轮的 user 消息和 assistant 回答会一起变成下一轮 recent window 的原料。

---

## 4. SOP 8：第二轮请求，验证 recent window 生效

### 4.1 执行命令

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"那你现在总结一下你记得的内容。",
    "model":"qwen:7b",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

### 4.2 预期结果

第二轮时，这个 `session_id` 已经有历史消息了，所以正常预期是：

- `used_messages > 0`
- `recent_window` 不为空
- `recent_window` 里应能看到上一轮的消息

例如已经验证过的一种结果：

- `used_messages = 2`
- `recent_window` 里有第一轮的 user 和 assistant 两条记录

### 4.3 代码里发生了什么

查 recent messages 的逻辑在：

- [store_mysql.go](/offline-rag-go-lab/internal/recentchat/store_mysql.go:16)

SQL 会先按倒序查最近消息：

```sql
ORDER BY created_at DESC, id DESC
LIMIT ?
```

然后在 Go 里再反转回正序：

```go
for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
    messages[left], messages[right] = messages[right], messages[left]
}
```

这样做的意义是：

- 数据库里方便取“最近几条”
- 送给模型时仍保持“从旧到新”的对话顺序

最终 recent window 会被拼进 Ollama 请求消息列表，再加上本轮用户新消息，一起发给模型。

---

## 5. SOP 9：验证 `recent_limit` 裁剪

### 5.1 执行命令

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"如果你只能看到很少的历史，你还能记住什么？",
    "model":"qwen:7b",
    "recent_limit":1,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

### 5.2 预期结果

这一步重点看两个字段：

- `used_messages`
- `recent_window`

因为当前 `recent_limit = 1`，正常预期是：

- `used_messages` 最多接近 `1`
- `recent_window` 只保留最近的一条历史消息

### 5.3 代码里发生了什么

当前 recent window 策略在：

- [window_count.go](/offline-rag-go-lab/internal/recentchat/window_count.go:5)

实现是：

```go
func (b CountWindowBuilder) Build(messages []Message, maxMessages int) []Message {
    if maxMessages <= 0 || len(messages) <= maxMessages {
        return messages
    }

    return messages[len(messages)-maxMessages:]
}
```

也就是说，当前第一层 recent window 的裁剪方式非常直接：

- 不是按 token
- 不是按主题
- 不是按摘要
- 就是按“最近 N 条消息”

这一步验证的是：

**第一层不仅能记忆，还会裁剪。**

---

## 6. 如何判断第 1 层已经跑通

如果下面这些现象都成立，就说明第一层已经真实跑通：

1. 第一轮请求成功，`used_messages = 0`
2. MySQL 表里出现两条记录：user + assistant
3. 第二轮请求成功，`used_messages > 0`
4. 第二轮的 `recent_window` 能看到上一轮历史
5. `recent_limit = 1` 时，`recent_window` 被裁成只剩最近一条

满足以上 5 条，就可以认为：

**Recent Window Layer 01 已经从“设计”进入“真实可运行状态”。**

---

## 7. 当前层的已知限制

这层当前已经真实可用，但仍然有明显边界：

- 只做 recent window，不做 session summary
- 不做 long-term memory
- 不做 memory item 提取
- 不按 token budget 裁剪
- 不和文档知识检索做联合上下文拼接

这些限制不会影响第 1 层验证，但会影响后续生产能力。

---

## 8. 下一步建议

如果 SOP 9 也验证通过，下一步最自然的升级顺序是：

1. 把 count-based recent window 升级成 token-budget-based recent window
2. 增加 session summary
3. 增加 memory item 提取和存储
4. 再把 memory retrieval 和文档 retrieval 合并

---

## 9. SOP 10：自动 token budget

旧 SOP 6 到 SOP 9 仍用于理解 count-based recent window。当前项目已经继续升级为 automatic token budget。

完整运行手册：

- [recent-chat-automatic-token-budget-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-automatic-token-budget-sop.md:1)

新请求核心字段：

```json
{
  "auto_token_budget": true,
  "output_token_reserve": 256
}
```

新响应核心字段：

```text
budget_mode
context_limit
fixed_input_tokens
output_token_reserve
available_recent_tokens
used_recent_tokens
```

第 1 层 count window 已经用于打基础；automatic token budget 是第 2 层容量控制的真实落地。
