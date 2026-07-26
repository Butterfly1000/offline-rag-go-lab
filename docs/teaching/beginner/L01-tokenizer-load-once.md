# L01：tokenizer 在启动时加载一次，文本来时只编码

对应工程课：[Tokenizer 小节 01](../tokenizer-load-once-sop.md)。本课讲 `cmd/tokenizer-demo` 和 `internal/tokenizerdemo` 的真实加载/编码边界。

## 1. 本课一句话

服务启动时把 `tokenizer.json` 构造成内存 tokenizer；之后每段文本只做 Encode 并读取 token 数，不重复读文件或重写规则。

## 2. 先用人话理解

像开店前把词典装上书架：开门时装一次，之后每个顾客来只查词典。每次提问都重新搬整套书，既慢也更晚发现资产错误。

## 3. 系统里有什么

- 命令：`cmd/tokenizer-demo`；默认资产路径是 `assets/tokenizers/qwen2/tokenizer.json`。
- 加载器：`LoadCounter` 调用 `pretrained.FromFile`，把 normalizer、pre-tokenizer、model、post-processor、decoder 构造成内存对象。
- 编码器：`Counter.CountText` 调用 `EncodeSingle(text, false)`，输出 count、token 片段和词表 ID。
- 前置：F07 的用户提供资产与本地 replace 代码库；tokenizer 资产和模型 chat template 仍是不同对象。

## 4. 一条完整链路

路径 → `LoadCounter` 一次读取/构造 → `Counter` 保存内存 tokenizer → 每次 `CountText` 编码 → `Encoding.Len()` 返回 count。加载失败发生在初始化，编码失败发生在处理文本时。

## 5. 实际怎么做

先确保约定路径存在用户提供的资产，再运行：

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/tokenizer-demo --text 'Tokenizer 不需要我们逐份重写规则。'
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/tokenizerdemo
```

- 等级：纯本地；依赖 Go、本地 replace 与 tokenizer 资产。命令只会写仓库内 Go build cache，不写 MySQL/Ollama/业务数据。
- 重复执行：安全；相同资产与输入应产生相同编码结果。
- 资产路径不存在时可用 `--tokenizer assets/tokenizers/not-found/tokenizer.json` 验证启动期失败；这也是纯本地操作。
- 这两条命令**不校验 SHA256**：`cmd/tokenizer-demo` 没有 `--expect-sha256` 参数，`go test ./internal/tokenizerdemo` 也不是对当前文件指纹的自动门禁。

## 6. 结果怎么看

demo 输出 `Tokenizer path`、`Token count`、`First tokens`、`First token ids`。`Len()` 是本次 tokenizer 执行后的 token 数；片段显示为 byte-level/BPE 样式时可能看起来异常，但不等于错误。不存在的文件应在开始编码前报出路径错误。

## 7. 算一遍或走一遍

当前已知 SHA256 为 `b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8` 的资产上，中文黄金文本“我叫小黄，这个项目是 Go 写的。”的回归值是 15 token；“我”是 1 token，ID 56023。它们是当前已知资产/本地执行链的事实，但本节的两条命令不会自动证明自己正在使用这份 SHA；它们不是所有 Qwen、所有版本或所有 tokenizer 的固定事实。

## 8. 常见误解

“load once”表示每次 `CountText` 都不做工作是错的；它仍然执行 Encode，只是不再解析整个 JSON。“token 数等于字符数”也错，必须以目标 tokenizer 实际编码为准。

## 9. 当前实现与生产边界

已保证：本地资产成功加载后复用内存对象，纯文本计数不加 special tokens。未保证：本节 demo/test 所用资产的 SHA 自动守卫、资产与运行模型严格匹配、完整 chat template token、跨机器同资产可用性。自动指纹守卫在后续 `cmd/tokenizer-inspect --expect-sha256 ...` 的课程中建立；模型元数据和模板也需后续课共同验证。

## 10. 面试怎么说

30 秒版：tokenizer 是模型输入的计量器，正确做法是启动时从 `tokenizer.json` 构造一次内存对象，请求时只 Encode 并取长度。这样把资产/兼容错误前置，也避免重复解析。展开点：加载与编码的区别、为什么 `false` 不加 special tokens、为何黄金数字必须绑定 SHA。

## 11. 自检题与答案

问：每段文本都重新读取 JSON 吗？答：不需要，Counter 复用已构造对象。问：本节 `cmd/tokenizer-demo` 会自动验证 b6f… 指纹吗？答：不会，它没有 `--expect-sha256` 参数；自动守卫在后续 `tokenizer-inspect`。问：15 token 是所有中文文本的规律吗？答：不是，只是指定文本在已知 SHA 资产上的回归值。问：文件不存在会在何时失败？答：加载阶段，编码前。

## 12. 事实锚点

- 原课：[tokenizer-load-once-sop.md](../tokenizer-load-once-sop.md)
- 命令：`cmd/tokenizer-demo/main.go`
- Go 组件：`internal/tokenizerdemo/tokenizer.go`
- 测试：`internal/tokenizerdemo/tokenizer_test.go`、`scripts/regression/lesson-08.sh`
- 资产/环境：[F07](F07-tokenizer-asset-setup.md)、[AI_INITIALIZATION.md](../../../AI_INITIALIZATION.md)
