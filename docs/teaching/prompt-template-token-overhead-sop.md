# Tokenizer 小节 06：计算 Prompt Template 的 Token 开销

这一小节解决：

- 只计算 system/user 正文，与计算完整 rendered prompt，结果差多少？

---

## Part 1：运行真实命令

在项目根目录执行：

```bash
go run ./cmd/prompt-budget-demo \
  --system '你是一个 Go 项目教学助手。' \
  --prompt '解释 token 是如何计算的。'
```

当前本机真实结果：

```text
System content tokens: 24
User content tokens: 15
Content-only total: 39
Rendered prompt tokens: 88
Template overhead tokens: 49
```

计算关系：

```text
正文总量 = 24 + 15 = 39
模板开销 = 88 - 39 = 49
```

---

## Part 2：为什么多出 49 tokens

完整 rendered prompt 还包含：

- `<|im_start|>`
- `system`
- `user`
- `<|im_end|>`
- 换行
- assistant 开始标记

tokenizer 对完整字符串执行编码后得到 `88`。

这里的 `49` 是整体差值，不应简单理解为“每个标记固定几个 token”。因为 BPE 会受到相邻字符和边界影响，整体编码结果不一定等于每段孤立编码结果机械相加。

---

## Part 3：代码如何比较

核心代码：

- [count.go](/offline-rag-go-lab/internal/promptbudget/count.go:1)

接口只要求一个能力：

```go
type TextTokenCounter interface {
    CountText(text string) (count int, tokens []string, ids []int, err error)
}
```

当前传入的是已经实现的本地 tokenizer counter。

`CompareTokens` 依次计算：

```text
system 正文
-> user prompt 正文
-> rendered prompt 整体
-> rendered - content-only
```

返回：

```go
type TokenComparison struct {
    SystemTokens     int
    PromptTokens     int
    ContentTokens    int
    RenderedTokens   int
    TemplateOverhead int
}
```

---

## Part 4：命令新增了什么

命令入口：

- [main.go](/offline-rag-go-lab/cmd/prompt-budget-demo/main.go:1)

本节新增参数：

```text
--tokenizer assets/tokenizers/qwen2/tokenizer.json
```

完整执行顺序：

```text
读取 Ollama model template
-> Render system/user prompt
-> 加载本地 tokenizer
-> 计算正文和 rendered prompt
-> 输出差值
```

顺序很重要：必须先生成 `rendered`，才能计算 rendered prompt tokens。

---

## Part 5：当前数字的边界

`39 / 88 / 49` 是当前本机 tokenizer 资产对当前文本的真实计算结果。

它能证明：

- 模板包装会产生不可忽略的 token 开销
- content-only 不等于完整 prompt token 数

它暂时不能证明：

- 当前 tokenizer 与 Ollama `qwen:7b` 完全一致
- `88` 与 Ollama 内部最终统计绝对相同
- 多轮 `/api/chat` 的 `.Prompt` 拼接已经精确复刻

这些严格一致性问题已经记录到优化 backlog，不阻塞当前预算框架。

---

## Part 6：运行测试和构建

执行：

```bash
go test ./internal/promptbudget
go build ./cmd/...
```

测试代码：

- [count_test.go](/offline-rag-go-lab/internal/promptbudget/count_test.go:1)

当前覆盖：

1. 正确计算正文总量、渲染总量和模板差值
2. tokenizer 计数失败时保留“哪一部分失败”的错误上下文
3. 所有命令入口都必须能编译

---

## 本节重点

1. 正文 token 只是请求成本的一部分。
2. 应对完整 rendered prompt 做一次整体编码。
3. 模板开销用 `rendered - content-only` 观察最直接。
4. 本节建立了可比较框架；下一节用 rendered token 数计算上下文预算。
