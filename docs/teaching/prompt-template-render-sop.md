# Tokenizer 小节 05：渲染真实 Ollama Prompt Template

这一小节解决：

- 已经从 `/api/show` 拿到 template 后，Go 如何把 system 和 user prompt 填进去？

本节只渲染，不计算 token。

---

## Part 1：运行真实命令

确认 Ollama 正在运行，然后在项目根目录执行：

```bash
go run ./cmd/prompt-budget-demo \
  --system '你是一个 Go 项目教学助手。' \
  --prompt '解释 token 是如何计算的。'
```

当前本机输出的渲染结果是：

```text
<|im_start|>system
你是一个 Go 项目教学助手。<|im_end|><|im_start|>user
解释 token 是如何计算的。<|im_end|>
<|im_start|>assistant
```

这证明模型输入不是两段裸文本，而是模板渲染后的完整 prompt。

---

## Part 2：命令的数据流

命令入口：

- [main.go](/offline-rag-go-lab/cmd/prompt-budget-demo/main.go:1)

执行顺序：

```text
调用 Ollama /api/show
-> 取得当前模型 template
-> 读取 --system 和 --prompt
-> 调用 promptbudget.Render
-> 打印 rendered prompt
```

当前没有把 template 复制成项目常量。模型换了，命令会读取新模型自己的 template。

---

## Part 3：Go 如何执行模板

核心代码：

- [render.go](/offline-rag-go-lab/internal/promptbudget/render.go:1)

Ollama 当前返回的是 Go template 风格文本，因此代码使用标准库：

```go
template.New("ollama-prompt").Parse(templateText)
```

再把下面的数据交给模板：

```go
type TemplateData struct {
    System string
    Prompt string
}
```

执行：

```go
tmpl.Execute(&rendered, TemplateData{
    System: system,
    Prompt: prompt,
})
```

模板里的：

- `{{ .System }}` 读取 `System`
- `{{ .Prompt }}` 读取 `Prompt`
- `{{ if .System }}` 在 system 非空时才输出 system 区块

---

## Part 4：为什么不手写包装字符串

不推荐直接在项目里写死：

```text
<|im_start|>user
...
```

原因是不同模型可以使用不同：

- 角色标记
- special tokens
- 换行
- system 条件
- assistant 生成前缀

更稳妥的框架是：

```text
模型提供 template
-> 项目负责加载和执行
```

---

## Part 5：运行单元测试

执行：

```bash
go test ./internal/promptbudget
```

测试代码：

- [render_test.go](/offline-rag-go-lab/internal/promptbudget/render_test.go:1)

当前覆盖：

1. system 非空时渲染 system、user、assistant 包装
2. system 为空时省略 system 区块
3. template 语法错误时返回错误

---

## 本节重点

1. template 是可执行的文本规则，不只是说明文档。
2. `text/template` 负责解释 `if` 和字段引用。
3. 项目使用模型返回的 template，不为每个模型重写包装算法。
4. 本节只观察渲染结果，不改变 `/chat`，下一节再计算包装后的 token 开销。
