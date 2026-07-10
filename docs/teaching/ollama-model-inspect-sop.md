# Tokenizer 小节 04：从 Ollama 读取上下文与模板

这一小节回答两个真实问题：

1. `qwen:7b` 的 `32768` 从哪里读取？
2. 用户消息进入模型前，除了 `message.Content` 还增加了什么？

答案来自 Ollama 的 `/api/show`，不是代码里写死的常量。

Ollama 官方接口说明：

- [Show model details](https://docs.ollama.com/api-reference/show-model-details)

---

## Part 1：先确认 Ollama 正在运行

执行：

```bash
curl http://127.0.0.1:11434/api/tags
```

预期能看到本地模型列表，其中包含：

```text
qwen:7b
```

如果连接失败，应先启动 Ollama，而不是修改本节 Go 代码。

---

## Part 2：直接调用 `/api/show`

执行：

```bash
curl http://127.0.0.1:11434/api/show \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen:7b"}'
```

这是一个 `POST` 请求，请求体只需要模型名：

```json
{
  "model": "qwen:7b"
}
```

原始响应会包含 license、tensor 等大量信息。本节真正需要的只有：

- `details`
- `model_info`
- `parameters`
- `template`
- `capabilities`

---

## Part 3：运行项目里的摘要命令

为了避免每次阅读完整 JSON，执行：

```bash
go run ./cmd/ollama-model-inspect --model qwen:7b
```

也可以显式指定 Ollama 地址：

```bash
go run ./cmd/ollama-model-inspect \
  --base-url http://127.0.0.1:11434 \
  --model qwen:7b
```

当前本机真实输出的摘要是：

```text
Model: qwen:7b
Family: qwen2
Architecture: qwen2
Parameter size: 8B
Quantization: Q4_0
Context length: 32768
Capabilities: completion
```

命令入口：

- [main.go](/offline-rag-go-lab/cmd/ollama-model-inspect/main.go:1)

---

## Part 4：32768 是怎么取出来的

核心客户端：

- [ollama.go](/offline-rag-go-lab/internal/recentchat/ollama.go:1)

当前响应里有：

```json
{
  "general.architecture": "qwen2",
  "qwen2.context_length": 32768
}
```

代码没有直接写死 `qwen2.context_length`，而是：

```text
读取 general.architecture
-> 得到 qwen2
-> 拼出 qwen2.context_length
-> 读取 32768
```

这样以后换成其他架构时，仍然使用同一套读取框架。

这里的 `32768` 表示模型元数据报告的上下文能力上限。某次运行实际使用的 context size 还可能被 Ollama 运行参数设置得更小，这个差异已经记录到优化 backlog。

---

## Part 5：当前真实模板怎么看

本机 `qwen:7b` 返回的模板是：

```text
{{ if .System }}<|im_start|>system
{{ .System }}<|im_end|>{{ end }}<|im_start|>user
{{ .Prompt }}<|im_end|>
<|im_start|>assistant
```

把模板压成行为就是：

```text
如果有 system prompt
-> 加 system 开始标记
-> 放 system 内容
-> 加结束标记

放 user 开始标记
-> 放用户 prompt
-> 加结束标记

放 assistant 开始标记
-> 等模型继续生成回答
```

所以模型实际接收的内容并不只有：

```text
我叫小黄。
```

还会包含类似：

```text
<|im_start|>user
我叫小黄。<|im_end|>
<|im_start|>assistant
```

这些角色、换行和结束标记同样会消耗 token。

---

## Part 6：为什么当前 content-only 会少算

当前 recent window 主要执行：

```go
counter.CountText(message.Content)
```

它只计算消息正文，没有计算：

- `<|im_start|>`
- 角色名
- `<|im_end|>`
- 模板中的换行
- assistant 生成前缀

所以 content-only 适合先建立 token budget 框架，但不是最终的完整请求 token 数。

---

## Part 7：运行单元测试

执行：

```bash
go test ./internal/recentchat
```

测试代码：

- [ollama_show_test.go](/offline-rag-go-lab/internal/recentchat/ollama_show_test.go:1)

测试覆盖：

1. 请求必须发送到 `POST /api/show`
2. 请求体必须包含模型名
3. 能提取架构、上下文上限和模板
4. Ollama 返回非 2xx 时必须返回错误
5. 上下文元数据缺失时必须报错，不能静默显示成 `0`

---

## 本节重点

1. `32768` 来自 Ollama 模型元数据，不是项目猜出来的值。
2. `/api/show` 能返回模型的 prompt template。
3. 模板会增加角色、特殊标记和换行，所以 content-only 会少算。
4. 本节只负责读取真实模板；下一小节再把模板包装纳入可观察的 token 计算。
