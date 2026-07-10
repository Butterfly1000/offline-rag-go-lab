# Tokenizer 小节 03：用 SHA256 固定资产身份

这一小节只解决一个生产实践问题：

- 如何确认今天加载的 `tokenizer.json` 和之前验证过的是同一个文件？

使用的方法是 SHA256 文件指纹。

---

## Part 1：查看当前文件指纹

在项目根目录执行：

```bash
go run ./cmd/tokenizer-inspect
```

当前本机文件会输出：

```text
SHA256: b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8
```

SHA256 会读取文件的全部字节并计算固定长度摘要。

只要文件任意一个字节变化，摘要通常就会完全不同。因此它适合判断：

- 文件是否被替换
- 不同机器拿到的文件是否一致
- 部署时加载的是否是已经验证过的资产

---

## Part 2：校验预期指纹

把已经确认的 SHA256 作为预期值传入：

```bash
go run ./cmd/tokenizer-inspect \
  --expect-sha256 b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8
```

匹配时会输出：

```text
Fingerprint check: matched
```

命令退出码为 `0`，表示校验成功。

---

## Part 3：验证不匹配会失败

故意传入错误值：

```bash
go run ./cmd/tokenizer-inspect \
  --expect-sha256 wrong-hash
```

预期结果：

```text
tokenizer SHA256 mismatch: expected wrong-hash, actual ...
exit status 1
```

退出码 `1` 很重要，因为启动脚本、CI 或部署系统可以据此停止后续流程。

---

## Part 4：代码怎么实现

核心代码：

- [inspect.go](/offline-rag-go-lab/internal/tokenizerdemo/inspect.go:1)

计算使用 Go 标准库：

```go
fmt.Sprintf("%x", sha256.Sum256(data))
```

过程是：

```text
读取 tokenizer.json 的全部字节
-> sha256.Sum256(data)
-> 转成十六进制字符串
-> 与预期值比较
```

`VerifySHA256` 使用不区分大小写的比较，因为十六进制字母大小写不影响指纹含义。

---

## Part 5：SHA256 能证明什么，不能证明什么

它能证明：

- 两次检查的文件内容是否完全一致

它不能单独证明：

- 文件一定来自 Qwen 官方仓库
- 文件一定和 Ollama 中的 `qwen:7b` 匹配
- token IDs 一定和模型实际使用的 tokenizer 相同
- chat template 一定一致

因此，当前框架完成的是“资产身份固定”，不是“模型匹配证明”。

严格的模型来源和 token IDs 对照属于后续优化，已经记录在：

- [00-optimization-backlog.md](/offline-rag-go-lab/docs/teaching/00-optimization-backlog.md:1)

---

## Part 6：运行单元测试

执行：

```bash
go test ./internal/tokenizerdemo
```

当前测试覆盖：

1. 文件 SHA256 计算结果固定
2. SHA256 大小写不同仍可匹配
3. 指纹不同会返回包含预期值和实际值的错误

---

## 本节重点

1. SHA256 是文件身份指纹，不是模型兼容性证明。
2. `--expect-sha256` 可以把人工观察变成自动校验。
3. 正确值退出 `0`，错误值退出 `1`，因此可以接入 CI 和启动流程。
4. 当前主线已经有足够的资产一致性框架，更严格的模型对照后续按需优化。
