# Conversation Token Count SOP

主题：第 09 节，完整聊天输入的 token 是怎么计算的

## 1. 这一节解决什么问题

第 08 节已经说明一条消息不是只有正文。真实请求还会同时包含多条消息：

```text
system
-> recent history
-> current user
-> assistant generation prefix
```

如果分别计算每段正文再相加，会漏掉角色、边界和生成前缀。当前实现改为：

```text
先渲染完整 conversation
-> 把完整字符串交给 tokenizer 一次
-> tokenizer 返回最终 token 数
```

## 2. 代码结构

- [qwen.go](/offline-rag-go-lab/internal/chatprompt/qwen.go:1)
- [count.go](/offline-rag-go-lab/internal/chatprompt/count.go:1)
- [count_test.go](/offline-rag-go-lab/internal/chatprompt/count_test.go:1)
- [main.go](/offline-rag-go-lab/cmd/conversation-token-demo/main.go:1)

### Part 1：`Render` 保持消息顺序

```go
for i, message := range messages {
    formatted, err := f.FormatMessage(message)
    if err != nil {
        return "", fmt.Errorf("format message %d: %w", i, err)
    }
    rendered.WriteString(formatted)
}
```

`range` 按 slice 顺序遍历消息。`i` 是从 `0` 开始的下标；错误里记录下标后，可以直接定位数据库返回的哪条历史消息不合法。

`%w` 会把原始 error 包装进去。上层既能看到“format message 2”这种业务位置，也能继续使用 `errors.Is` 或 `errors.As` 检查底层错误。

### Part 2：assistant prefix 为什么没有结束标记

完整历史 assistant 消息是：

```text
<|im_start|>assistant
旧回答<|im_end|>
```

但当前等待模型生成时，末尾只有：

```text
<|im_start|>assistant
```

它没有 content 和 `<|im_end|>`，因为这些内容正等待模型生成。这个前缀本身也占 token。

### Part 3：为什么 tokenizer 只调用一次

```go
total, _, _, err := c.counter.CountText(rendered)
```

Go 允许函数返回多个值。当前 `CountText` 返回：

```text
count, tokens, ids, error
```

这一层只关心最终 `count`，所以使用 `_` 忽略 token 字符串和 token IDs。

关键不是 `_`，而是传给 `CountText` 的参数是完整 `rendered`，而不是某一条 `Content`。

## 3. 实践 SOP

### SOP 1：确认 tokenizer 资产

默认路径：

```text
assets/tokenizers/qwen2/tokenizer.json
```

该文件被 `.gitignore` 忽略，只作为本地模型资产使用。如果本地资产放在其他位置，通过 `--tokenizer` 指定。

### SOP 2：计算完整 conversation

```bash
go run ./cmd/conversation-token-demo \
  --system '你是 Go 助手。' \
  --history-user '我叫小黄。' \
  --history-assistant '记住了。' \
  --prompt '我叫什么？'
```

输出结构：

```text
Messages: 4
Rendered conversation:
<|im_start|>system
你是 Go 助手。<|im_end|>
<|im_start|>user
我叫小黄。<|im_end|>
<|im_start|>assistant
记住了。<|im_end|>
<|im_start|>user
我叫什么？<|im_end|>
<|im_start|>assistant
Total prompt tokens: ...
```

`Total prompt tokens` 的具体数字由 tokenizer 资产决定。换 tokenizer 后数字可能改变，因此不能把某个示例数字写成所有 Qwen 模型的固定答案。

本次使用当前项目 tokenizer 资产的实测结果是：

```text
Messages: 4
Total prompt tokens: 122
```

### SOP 3：去掉历史做对照

```bash
go run ./cmd/conversation-token-demo \
  --history-user '' \
  --history-assistant ''
```

此时消息数量减少，完整 token 数也应减少。这说明历史窗口占用的是模型上下文容量，不是免费存储。

本次对照实测：

```text
Messages: 2
Total prompt tokens: 62
```

`122 - 62 = 60` 是本次两条历史消息及其角色/边界带来的完整增量。这个数字用于复现实验，不是其他 tokenizer 的固定结论。

### SOP 4：运行测试

```bash
go test ./internal/chatprompt
```

`TestTokenCounterCountsRenderedConversationOnce` 使用 recording fake 验证两件事：

1. tokenizer 只调用一次
2. tokenizer 收到的字符串与返回的 `TokenUsage.Rendered` 完全相同

## 4. 和上一节的关系

第 08 节：

```text
一条 Message -> 格式化字符串
```

第 09 节：

```text
多条 Message
-> 保持顺序逐条格式化
-> 增加 assistant prefix
-> 完整 tokenizer 计数
```

因此第 09 节不是重复第 08 节，而是把单条消息行为组合成完整 prompt 行为。

## 5. 生产边界

当前实现采用明确的 Qwen ChatML conversation 格式，适合：

- 教学完整 token 组成
- 给 recent window 分配更真实的预算
- 固定当前项目的可测试规则

仍未证明：

- Ollama `/api/chat` 内部对多轮 `.Prompt` 的逐 token 结果与本地格式完全一致
- 本地 tokenizer 和 Ollama 中的模型文件来自同一个上游 revision

这些严格一致性问题已经进入优化清单。本节不使用字符估算，也不把“可解释的本地真实 tokenizer 计数”冒充“Ollama 内部黄金结果”。

## 6. 本节重点

```text
完整 prompt token
= tokenizer(render(system + history + current user + assistant prefix))
```

下一节会把相同消息格式用于 recent window，解决旧实现只计算 `message.Content` 的问题。
