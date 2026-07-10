# Tokenizer 小节 07：规划 Context、Output 与 History 预算

这一小节解决：

- 已知模型上下文上限和当前 prompt token 后，还能给 recent history 留多少 token？

---

## Part 1：预算公式

本节使用最小可落地公式：

```text
history budget
= context limit
- fixed rendered prompt tokens
- output reserve
```

其中：

- `context limit`：从 Ollama `/api/show` 读取
- `fixed rendered prompt tokens`：上一节对完整模板计算的结果
- `output reserve`：提前给模型回答保留的容量
- `history budget`：剩余可用于最近聊天历史的容量

---

## Part 2：运行真实命令

执行：

```bash
go run ./cmd/prompt-budget-demo \
  --system '你是一个 Go 项目教学助手。' \
  --prompt '解释 token 是如何计算的。' \
  --output-reserve 2048
```

当前本机关键结果：

```text
Context length: 32768
Rendered prompt tokens: 88
Output reserve tokens: 2048
Available recent history tokens: 30632
```

计算：

```text
32768 - 88 - 2048 = 30632
```

所以在当前简化框架里，recent history 最多可以使用 `30632` tokens。

---

## Part 3：为什么必须预留输出

模型上下文不是只装输入，也要给回答保留空间。

如果输入已经用满 `32768`，模型就没有足够容量继续生成回答。

所以生产预算通常先确定：

```text
希望模型最多回答多少 token
-> 先扣掉 output reserve
-> 剩余容量再分给固定输入和历史
```

当前 demo 默认：

```text
--output-reserve 2048
```

这个值是项目预算选择，不是模型固定要求，可以按业务回答长度调整。

---

## Part 4：超预算会怎样

故意把输出预留设得过大：

```bash
go run ./cmd/prompt-budget-demo \
  --system '你是一个 Go 项目教学助手。' \
  --prompt '解释 token 是如何计算的。' \
  --output-reserve 32700
```

当前固定输入是 `88`，因此：

```text
88 + 32700 = 32788
```

它超过模型上限 `32768`，命令返回：

```text
fixed input and output reserve (32788) exceeds context limit (32768)
exit status 1
```

这比得到一个负数 history budget 更安全，也方便脚本或 CI 阻止错误配置。

---

## Part 5：Go 代码怎么实现

核心代码：

- [budget.go](/offline-rag-go-lab/internal/promptbudget/budget.go:1)

输入：

```go
Plan(contextLimit, fixedInputTokens, outputReserve)
```

输出：

```go
type BudgetPlan struct {
    ContextLimit           int
    FixedInputTokens       int
    OutputReserve          int
    AvailableHistoryTokens int
}
```

函数先校验：

1. context limit 必须大于 `0`
2. fixed input 和 output reserve 不能为负数
3. fixed input 加 output reserve 不能超过 context limit

校验通过后才计算 history budget。

---

## Part 6：当前框架的边界

当前 fixed input 包含：

- system prompt
- 当前 user prompt
- 当前模型模板包装
- assistant 生成前缀

当前还没有加入：

- retrieval context
- session summary
- tools schema
- 多轮 `/api/chat` 的精确模板拼接
- 安全余量

生产升级时，这些固定部分也应先从 context limit 中扣除，再把剩余容量分给 recent history。

本节先把预算框架和失败行为搭好，没有修改现有 `/chat` 的 `recent_token_budget`。

---

## Part 7：运行测试与构建

执行：

```bash
go test ./internal/promptbudget
go build ./cmd/...
```

测试代码：

- [budget_test.go](/offline-rag-go-lab/internal/promptbudget/budget_test.go:1)

当前覆盖：

1. 正确计算 available history tokens
2. 固定输入加输出超过上限时返回错误
3. context 为 `0` 或参数为负数时返回错误

---

## 本节重点

1. 上下文预算必须同时考虑输入和输出。
2. recent history 使用的是扣除固定输入与输出预留后的剩余容量。
3. 超预算应明确失败，不能继续返回负数或发送请求。
4. 当前已经搭好统一预算的最小框架，更多输入类型以后按同一公式追加。
