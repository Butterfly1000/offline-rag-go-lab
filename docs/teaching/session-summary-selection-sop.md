# Session Summary Selection SOP

主题：第 14 节，如何选择真正离开 recent window 的连续旧消息

## 1. 这一节解决什么问题

第 13 节已经能决定“是否值得摘要”，但三个统计值还没有真实来源。第 14 节实现：

```text
按 ID 排序的消息
+ summary watermark
+ recent window 最早 ID
-> 未摘要消息
-> 已驱逐连续前缀
-> 两组真实 token
-> 安全的新 watermark
```

## 2. 三个边界

核心代码：

- [select.go](/offline-rag-go-lab/internal/sessionsummary/select.go:1)
- [token.go](/offline-rag-go-lab/internal/sessionsummary/token.go:1)
- [select_test.go](/offline-rag-go-lab/internal/sessionsummary/select_test.go:1)
- [main.go](/offline-rag-go-lab/cmd/summary-selection-demo/main.go:1)

输入示例：

```text
messages = 19,20,21,22,23,24,25,26
last_message_id = 20
recent_start_id = 25
```

分组结果：

```text
19,20       -> 已经在旧 summary 中
21,22,23,24 -> 已离开 recent window，应进入增量摘要
25,26       -> 仍保留原文，不应重复摘要
```

所以新水位只能是 `24`。

## 3. 为什么必须是连续前缀

假设只摘要 `21` 和 `23`，然后把 watermark 写成 `23`，消息 `22` 会落到水位之前，后续查询 `id > 23` 再也看不到它。

因此规则是：

```text
只能从 watermark 后第一条开始
连续选择到 recent_start 前一条
```

消息 ID 可以有空洞，例如 `21,23,30`。空洞可能来自数据清理，selector 不要求数字连续，但要求实际 slice 严格递增。

## 4. 代码行为

### Part 1：过滤水位

```go
for _, message := range messages {
    if message.ID > lastMessageID {
        unsummarized = append(unsummarized, message)
    }
}
```

`append` 把元素追加到 slice。这里创建新的 `unsummarized`，不修改调用方传入的消息。

### Part 2：寻找 recent 起点

`recent_start_id > 0` 时必须在未摘要消息中找到。找不到意味着调用方传错了窗口边界，继续推进 watermark 会有数据丢失风险，所以直接报错。

`recent_start_id = 0` 表示当前没有 recent 消息，此时全部未摘要消息都可视为已驱逐。

### Part 3：token 每条只算一次

selector 遍历全部未摘要消息一次：

```text
所有 count -> unsummarized_tokens
前缀 count -> evicted_tokens
```

`FormattedMessageCounter` 先生成：

```text
<|im_start|>{role}\n{content}<|im_end|>\n
```

再调用本地 tokenizer，因此统计包含 role 和消息边界，不是字符估算。

## 5. 实践 SOP

### SOP 1：标准边界

```bash
go run ./cmd/summary-selection-demo \
  --ids '19,20,21,22,23,24,25,26' \
  --watermark 20 \
  --recent-start 25
```

重点结果：

```text
Unsummarized IDs: 21,22,23,24,25,26
Evicted IDs: 21,22,23,24
Next watermark: 24
```

token 数由当前 tokenizer 资产决定。

本次实测：

```text
Unsummarized tokens: 129
Evicted tokens: 86
```

### SOP 2：验证 ID 空洞

```bash
go run ./cmd/summary-selection-demo \
  --ids '19,20,21,23,30' \
  --watermark 20 \
  --recent-start 30
```

预期：

```text
Evicted IDs: 21,23
Next watermark: 23
```

本次 token 实测为全部 `64`、驱逐前缀 `43`。

水位跟随实际最后一条消息，不使用 `latest_id - watermark` 推算数量。

### SOP 3：运行测试

```bash
go test ./internal/sessionsummary -run 'TestSelectPrefix|TestFormattedMessageCounter'
```

测试覆盖边界、空窗口、ID 空洞、无驱逐、非法顺序、缺失 recent 起点和 tokenizer 错误。

## 6. 和触发策略怎么连接

```go
TriggerInput{
    UnsummarizedMessages: len(selection.Unsummarized),
    UnsummarizedTokens:   selection.UnsummarizedTokens,
    EvictedMessages:      len(selection.Evicted),
}
```

第 13 节 policy 使用这些真实统计决定是否调用模型。第 15 节只把 `selection.Evicted` 交给 Ollama。

## 7. 当前生产边界

本节 selector 和 counter 可直接被后续 service 复用，但还没有：

- 从 MySQL 查询 watermark 后消息
- 生成摘要
- 保存新 watermark

这些分别由第 17、15、16 节完成。

## 8. 本节重点

```text
summary input = watermark 后、recent window 前的连续消息前缀
```

新 watermark 只能指向确实进入本次摘要的最后一条消息。
