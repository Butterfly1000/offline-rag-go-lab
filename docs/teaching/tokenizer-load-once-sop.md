# Tokenizer 小节 01：加载一次，再重复计算

这一小节回答两个实践问题：

1. 拿到一份兼容的 `tokenizer.json` 后，Go 程序怎么使用它？
2. 每计算一段文本，是否都要重新读取这个 JSON？

结论是：

```text
程序启动
-> 加载一次 tokenizer.json
-> 得到内存中的 tokenizer
-> 每次收到文本只调用 EncodeSingle
-> 用 Encoding.Len() 得到 token 数
```

不需要人工阅读每一份 JSON，再为它重写一套切分算法。

---

## Part 1：加载发生在哪里

代码入口：

- [tokenizer.go](/offline-rag-go-lab/internal/tokenizerdemo/tokenizer.go:1)

`LoadCounter` 里的关键调用是：

```go
tk, err := pretrained.FromFile(tokenizerPath)
```

`FromFile` 会读取 `tokenizer.json`，并根据里面的配置创建：

- normalizer
- pre-tokenizer
- model
- post-processor
- decoder

成功后，`Counter` 保存的是已经构造好的 tokenizer：

```go
type Counter struct {
    tokenizer *tokenizer.Tokenizer
}
```

因此文件路径错误、JSON 格式错误或当前 Go 库不支持某项规则，都会在程序启动阶段直接报错。

---

## Part 2：token 数如何得到

加载完成后，每次计算只执行：

```go
encoding, err := c.tokenizer.EncodeSingle(text, false)
```

这里的 `false` 表示这次纯文本演示不额外加入 special tokens。

编码结果包含：

- `encoding.Tokens`：切分后的 token 片段
- `encoding.Ids`：每个 token 对应的词表 ID
- `encoding.Len()`：token 数

所以 token 数不是字符数，也不是我们自行统计 JSON 规则条数，而是 tokenizer 执行完整编码流程后的结果长度。

---

## Part 3：传入任意文本实践

在项目根目录执行：

```bash
go run ./cmd/tokenizer-demo --text 'Tokenizer 不需要我们逐份重写规则。'
```

命令入口：

- [main.go](/offline-rag-go-lab/cmd/tokenizer-demo/main.go:1)

重点观察输出中的：

```text
Text:
Token count:
First tokens:
First token ids:
```

替换 `--text` 后面的内容，就能比较中文、英文、标点或代码的 token 结果。

也可以显式指定 tokenizer 文件：

```bash
go run ./cmd/tokenizer-demo \
  --tokenizer assets/tokenizers/qwen2/tokenizer.json \
  --text 'func main() { println("hello") }'
```

---

## Part 4：验证启动时加载失败

故意传一个不存在的文件：

```bash
go run ./cmd/tokenizer-demo \
  --tokenizer assets/tokenizers/not-found/tokenizer.json \
  --text 'hello'
```

预期行为：

- 程序在开始计算前报错退出
- 错误信息包含 tokenizer 文件路径

这证明文件加载属于初始化行为，不会等到处理很多条消息之后才发现配置错误。

---

## Part 5：运行单元测试

执行：

```bash
go test ./internal/tokenizerdemo
```

测试代码：

- [tokenizer_test.go](/offline-rag-go-lab/internal/tokenizerdemo/tokenizer_test.go:1)

当前测试固定了两个行为：

1. tokenizer 文件不存在时，`LoadCounter` 必须返回错误
2. `Counter` 能使用已经加载的 tokenizer 计算 token、token 文本和 token ID

---

## 本节重点

1. 通用用法是 `load -> encode -> Len`，不是人工重写 JSON 规则。
2. `tokenizer.json` 内部规则因模型而异，但调用流程通常相同。
3. tokenizer 应在服务启动时加载一次，后续请求复用内存对象。
4. “调用流程通用”不等于“所有 tokenizer 文件都能被任意 Go 库兼容”。兼容性是下一小节的内容。
