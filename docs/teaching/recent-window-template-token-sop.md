# Recent Window Template Token SOP

主题：第 10 节，让 recent window 按完整消息 token 裁剪

## 1. 旧实现的问题

旧实现从最新消息往前选择，但计数对象是：

```go
messages[i].Content
```

例如两条正文都叫 `same`：

```text
user: same
assistant: same
```

正文相同不代表完整 token 开销相同，因为 role 不同：

```text
<|im_start|>user
same<|im_end|>

<|im_start|>assistant
same<|im_end|>
```

第 10 节把第 08 节的格式器接入窗口选择器。

## 2. 两种构造方式为什么同时保留

核心代码：

- [window_token_budget.go](/offline-rag-go-lab/internal/recentchat/window_token_budget.go:1)
- [window_token_budget_test.go](/offline-rag-go-lab/internal/recentchat/window_token_budget_test.go:1)
- [main.go](/offline-rag-go-lab/cmd/recent-chat/main.go:1)

旧构造器：

```go
NewTokenBudgetWindowBuilder(counter)
```

行为：

- 只计算 `Content`
- 最新消息即使超过预算，也强制保留
- 用于保留已有教学对照和兼容测试

新构造器：

```go
NewFormattedTokenBudgetWindowBuilder(
    counter,
    chatprompt.QwenFormatter{},
)
```

行为：

- 计算 role、content 和消息边界
- 使用严格预算
- 一条历史都放不下时返回空历史

真实 `cmd/recent-chat` 已切换到新构造器，因此实际服务不再使用旧的正文计数。

## 3. 窗口选择代码怎么运行

### Part 1：从最新消息向前

```go
for i := len(messages) - 1; i >= 0; i-- {
```

slice 下标从 `0` 开始。`len(messages)-1` 是最后一条，也就是当前列表中的最新消息。`i--` 每次向前移动一条。

### Part 2：格式化后再计数

```go
text, err := b.textForCount(messages[i])
count, _, _, err := b.counter.CountText(text)
```

新构造器设置了 formatter，所以 `textForCount` 返回完整 Qwen 消息。旧构造器没有 formatter，才直接返回 `Content`。

### Part 3：严格预算

```go
if used+count > budget {
    if len(selected) == 0 && !b.strict {
        // 只有旧模式会强塞最新消息。
    }
    break
}
```

严格模式中，只要加入当前消息会超预算，就停止。这样 `used` 永远不会大于 `budget`。

如果最新历史本身就放不下：

```text
selected = []
used = 0
```

当前用户问题仍会发送给模型；被舍弃的只是历史消息。

### Part 4：恢复正序

窗口是从新往旧选择的，但模型必须按旧到新阅读。最后使用首尾交换：

```go
for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
    selected[left], selected[right] = selected[right], selected[left]
}
```

这段循环原地反转 slice，不创建第二份消息数组。

## 4. 实践 SOP

### SOP 1：运行格式化窗口测试

```bash
go test ./internal/recentchat -run 'TestFormattedTokenWindow' -v
```

重点测试：

```text
TestFormattedTokenWindowCountsRoleAndBoundaries
TestFormattedTokenWindowDoesNotForceOversizedNewestMessage
TestFormattedTokenWindowReturnsRoleFormattingError
```

第一个测试给 user/assistant 的完整格式字符串设置不同 token 数。只有实现真的传入完整字符串，测试才会通过。

第二个测试设置：

```text
newest formatted message = 10 tokens
budget = 9 tokens
```

预期严格窗口为空，证明没有突破预算。

### SOP 2：启动真实 recent-chat

确认现有 `config/recent-chat.env` 和本地 tokenizer 资产可用后：

```bash
go run ./cmd/recent-chat
```

无需新增数据库表或配置字段。

### SOP 3：用手动 token budget 请求

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-template-token-001",
    "user_id":"u-001",
    "message":"基于你还能看到的历史回答。",
    "model":"qwen:7b",
    "recent_limit":10,
    "recent_token_budget":50,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

重点看：

- `used_messages`
- `used_recent_tokens`
- `recent_window`

现在 `used_recent_tokens` 表示所选历史消息的 Qwen 格式 token 总和，不再只是正文 token 总和。

## 5. 生产级判断

本节解决了两个直接影响容量正确性的问题：

1. 角色和消息边界进入预算。
2. 严格模式不会因为“至少保留一条”而突破预算。

还没有解决：

- 历史预算仍由调用方手动填写
- system、当前问题和回答预留还没有统一扣除
- user/assistant 可能被分开裁剪
- 与 Ollama 内部逐 token 一致性尚未做黄金对照

其中前两项由第 11、12 节解决。成对裁剪和严格 Ollama 对照进入优化清单，不阻塞 token 主线。

## 6. 本节重点

```text
手动 recent token budget
-> 从最新历史向前
-> 每条先格式化
-> 严格装入预算
-> 恢复旧到新顺序
```

下一节会自动计算这个 `recent token budget`，不再要求调用方猜一个 `50` 或 `500`。
