# Automatic History Budget SOP

主题：第 11 节，如何自动得到 recent history token budget

## 1. 为什么不能让调用方一直手填

第 10 节的请求仍然需要：

```json
"recent_token_budget": 50
```

`50` 能用于演示裁剪，但调用方不知道它是否合理。真正可用的历史预算取决于：

```text
模型上下文上限
- 当前固定输入
- 回答预留
= 历史可用预算
```

第 11 节把这三个来源接起来，自动计算最后一个数字。

## 2. 代码组件

- [automatic.go](/offline-rag-go-lab/internal/promptbudget/automatic.go:1)
- [automatic_test.go](/offline-rag-go-lab/internal/promptbudget/automatic_test.go:1)
- [ollama.go](/offline-rag-go-lab/internal/recentchat/ollama.go:1)
- [main.go](/offline-rag-go-lab/cmd/automatic-budget-demo/main.go:1)

### Part 1：`ContextProvider`

```go
type ContextProvider interface {
    ContextLength(model string) (int, error)
}
```

Go interface 描述的是“调用方需要什么行为”，而不是“必须使用哪个结构体”。规划器只需要根据模型名取得 context length，不需要知道 HTTP、Ollama JSON 或 `/api/show`。

`HTTPOllamaClient` 实现该方法：

```go
func (c *HTTPOllamaClient) ContextLength(model string) (int, error) {
    summary, err := c.Show(model)
    if err != nil {
        return 0, err
    }
    return summary.ContextLength, nil
}
```

因此真实运行使用 Ollama，单元测试可以使用 fake provider 返回 `32768`。

### Part 2：`ConversationCounter`

```go
type ConversationCounter interface {
    Count(
        messages []chatprompt.Message,
        includeAssistantPrefix bool,
    ) (chatprompt.TokenUsage, error)
}
```

规划器把 `includeAssistantPrefix` 固定为 `true`，因为等待模型生成时，这个前缀已经属于输入。

### Part 3：规划器的数据流

```go
contextLimit, err := p.contextProvider.ContextLength(model)
usage, err := p.counter.Count(fixed, true)
budget, err := Plan(contextLimit, usage.TotalTokens, outputReserve)
```

三步分别回答：

1. 模型最多能容纳多少
2. 不包含历史时已经占了多少
3. 扣除回答后还能给历史多少

已有 `Plan` 函数负责最后的纯算术，本节没有复制公式。

### Part 4：为什么任何失败都不能估算

如果 `/api/show` 失败或 tokenizer 失败，规划器直接返回 error：

```go
return AutomaticPlan{}, fmt.Errorf("read model context length: %w", err)
```

生产容量控制中，悄悄退回“中文字符数除以某个比例”会让同一请求在不同文本上产生不可预测的误差。当前项目选择明确失败，让配置或资产问题可见。

## 3. 实践 SOP

### SOP 0：运行第 11-12 节只读回归

```bash
sh scripts/regression/lessons-11-12.sh
```

脚本验证 Ollama 模型、context metadata、预算恒等式和超限失败。默认模式不启动
recent-chat，也不写 MySQL；真实 `/chat` 验证需要显式 `--live`。

### SOP 1：确认 Ollama 和模型

```bash
curl http://127.0.0.1:11434/api/tags
```

返回中应包含：

```text
qwen:7b
```

### SOP 2：运行自动预算命令

```bash
go run ./cmd/automatic-budget-demo \
  --model qwen:7b \
  --system '你是 Go 助手。' \
  --prompt '解释 recent window。' \
  --output-reserve 2048
```

输出结构：

```text
Model: qwen:7b
Rendered fixed prompt:
...
Context limit: 32768
Fixed input tokens: ...
Output reserve tokens: 2048
Available recent history tokens: ...
```

最后三个数字必须满足：

```text
fixed input + output reserve + available history = context limit
```

本次使用本机 `qwen:7b` 和当前 tokenizer 资产的实测结果：

```text
Context limit: 32768
Fixed input tokens: 56
Output reserve tokens: 2048
Available recent history tokens: 30664
```

核对：

```text
56 + 2048 + 30664 = 32768
```

### SOP 3：验证超限失败

把回答预留改成接近模型总容量：

```bash
go run ./cmd/automatic-budget-demo \
  --model qwen:7b \
  --output-reserve 32768
```

固定 prompt 还会占 token，所以命令应失败并包含：

```text
exceeds context limit
```

这证明规划器不是把负数历史预算改成 `0` 后继续，而是在固定输入和回答已经不可能同时容纳时拒绝请求。

本次超限实测为：

```text
fixed input and output reserve (32824) exceeds context limit (32768)
```

旧值 `64/30656/32832` 来自 Tokenizer 中文执行链修复前。模型 context limit 没有
变化，变化的是本地固定 prompt 的正确 token 计数。

### SOP 4：运行测试

```bash
go test ./internal/promptbudget
go test ./internal/recentchat -run TestHTTPOllamaClientContextLengthUsesShowMetadata
```

测试覆盖正常预算、provider 错误、tokenizer 错误、容量超限和 Ollama metadata 适配。

## 4. 和前面三节的连接

```text
第 08 节：一条消息如何格式化
第 09 节：固定 conversation 如何完整计数
第 10 节：给定历史预算后如何选择历史
第 11 节：历史预算本身如何自动得到
```

现在只差最后一步：把规划器接入 `/chat` 请求，使真实服务自动选择模式并返回预算明细。

## 5. 当前生产边界

当前命令每次执行都会调用一次 `/api/show`。对教学和单请求验证足够，生产服务通常会：

- 按模型缓存 context metadata
- 模型或配置更新时刷新
- 把 model、tokenizer、chat format 和 context limit 绑定到同一个 registry

本项目暂时不提前实现多模型缓存；它已经记录在优化清单，等引入第二个模型时再落地。

## 6. 本节重点

```text
automatic history budget
= model context limit
- tokenizer(complete fixed prompt + assistant prefix)
- output reserve
```

第 12 节完成 `/chat` 接入后，token 主线结课。
