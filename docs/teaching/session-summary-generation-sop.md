# Session Summary Generation SOP

主题：第 15 节，用 Ollama 真实生成增量滚动摘要

## 1. 数据流

```text
旧 summary + 第 14 节 Evicted 消息
-> BuildUpdatePrompt
-> Ollama /api/chat
-> trim + empty check
-> 新 summary
```

核心代码：

- [prompt.go](/offline-rag-go-lab/internal/sessionsummary/prompt.go:1)
- [generator.go](/offline-rag-go-lab/internal/sessionsummary/generator.go:1)
- [ollama.go](/offline-rag-go-lab/internal/recentchat/ollama.go:1)
- [main.go](/offline-rag-go-lab/cmd/summary-generate-demo/main.go:1)

## 2. Prompt 为什么分两块

```text
<previous_summary>旧摘要</previous_summary>
<new_messages>[id=21 role=user] ...</new_messages>
```

旧摘要提供累计状态，新消息只提供本次增量。消息正文使用 `html.EscapeString` 转义 `<`、`>`、`&`，避免用户文本伪造边界。

system 指令要求只输出新摘要，并保留偏好、目标、约束、事实和已确认决定；删除重复与临时过程，不允许编造。

## 3. Go 接口

```go
type TextGenerator interface {
    GenerateText(model, system, prompt string, maxTokens int) (string, error)
}
```

`sessionsummary` 只依赖这个小接口，不依赖 `recentchat`。真实命令注入 `HTTPOllamaClient`，测试注入 fake。

`GenerateText` 使用两个 Ollama message：system 和 user，并把 `maxTokens` 传为 `options.num_predict`。

## 4. 实践 SOP

```bash
go run ./cmd/summary-generate-demo \
  --model qwen:7b \
  --previous '用户叫小黄，代码示例使用 Go。' \
  --max-tokens 256
```

输入的新增消息固定为 IDs `21,22,23`，内容包含“真实落地”“token 自动预算完成”“继续 session summary”。

预期输出的新摘要应同时保留：

- 用户叫小黄
- 使用 Go 示例
- 偏好真实落地
- token 自动预算已完成
- 下一步 session summary

模型具体措辞可以不同，事实范围不能超出输入。

本次真实输出保留了全部关键事实：小黄、Go、真实落地、token 自动预算已接入 `/chat`、继续 session summary。首次输出带 `<updated_summary>` wrapper，review 后增加了确定性 wrapper 清理并强化 system 指令；第二次仍出现“以下是更新后的摘要正文”引导语，说明内容质量不能只靠 prompt 保证，后续需要结构化输出或质量评估。

测试：

```bash
go test ./internal/sessionsummary -run 'TestBuildUpdatePrompt|TestGenerator'
go test ./internal/recentchat -run TestHTTPOllamaClientGenerateText
```

## 5. 错误边界

- 没有新增消息：不调用模型
- model 为空或 max token 非正：失败
- Ollama 错误：保留 `generate summary` 上下文
- 模型只返回空白：拒绝作为摘要
- 常见 `<updated_summary>` wrapper：确定性移除

生成成功仍不代表 watermark 已推进。第 16 节只有在 MySQL 安全保存成功后才提交新 content/version/watermark。

## 6. 本节重点

```text
rolling summary 不是重写全文
= previous summary + newly evicted messages
```
