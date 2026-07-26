# L02：tokenizer.json 里有哪些编码组件

对应工程课：[Tokenizer 小节 02](../tokenizer-inspect-sop.md)。本课看配置结构，不执行完整 BPE 编码，也不证明资产与某个模型匹配。

## 1. 本课一句话

`tokenizer.json` 描述一条编码流水线；结构检查告诉我们文件声明了哪些组件，而不是证明模型一定会按它工作。

## 2. 先用人话理解

像检查食谱的目录：能看到“清洗、切配、烹饪”的步骤，却没有真的做出菜，更不能证明这就是某家餐厅正在用的原始食谱。

## 3. 系统里有什么

- 入口：`cmd/tokenizer-inspect`；默认读取 `assets/tokenizers/qwen2/tokenizer.json`。
- 检查器：`internal/tokenizerdemo/inspect.go`，使用 `encoding/json` 读取顶层字段。
- 输出：格式版本、model/normalizer/pre-tokenizer/post-processor/decoder 类型、词表和 added-token 数量、文件 SHA256。
- 与 L01 的区别：inspect 不调用 `FromFile` 或 `EncodeSingle`，demo 才会真实加载和编码。

## 4. 一条完整链路

文件字节 → JSON 解码 → 提取各组件的 `type` → 统计 vocab/added tokens → 输出摘要与 SHA。它不会执行 NFKC、BPE merge、后处理或解码。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/tokenizer-inspect --tokenizer assets/tokenizers/qwen2/tokenizer.json
```

- 等级：纯本地；依赖 F07 准备的资产与 Go 依赖，只会写 Go build cache。
- 重复执行：安全；同一文件会得到同一结构摘要和 SHA。
- 没有 MySQL、Ollama、模型生成或业务写入副作用。

## 6. 结果怎么看

常见输出字段如 `Model: BPE`、`Normalizer: NFKC`、`Pre-tokenizer: Sequence`、`Post-processor: TemplateProcessing`、`Decoder: ByteLevel`。它们分别表示词表切分模型、文本规范化、预切分组合、编码后包装规则和反向解码方式。无效 JSON 或不可读文件会报错，说明结构检查失败。

## 7. 算一遍或走一遍

若摘要显示基础词表 50,000、added tokens 23,944，这只是该**当前资产**中两个 JSON 集合的数量；它不能推出输入“hello”一定是一个 token，也不能说明该资产与模型词表相同。

## 8. 常见误解

“能解析组件结构 = 与 Ollama 模型匹配”是错的。解析只说明 JSON 形状满足检查器；模型匹配还需要来源、版本、指纹和与实际 tokenizer 的对照。

## 9. 当前实现与生产边界

已保证：读取顶层组件类型、计数和指纹，且不泄露完整词表。未保证：组件能被执行库完整支持、资产与模型/模板匹配、编码 token ID 正确；这些分别要用 L01、L03、L04–L06 等证据补足。

## 10. 面试怎么说

30 秒版：inspect 把 tokenizer.json 当配置清单，快速展示编码管线有哪些阶段并统计资产规模；它适合诊断，但不执行算法，因此结构有效不能证明模型匹配。展开点：inspect 和 encode 的职责差异、为何不打印词表、为何 SHA 另有边界。

## 11. 自检题与答案

问：`Normalizer: NFKC` 代表 inspect 已执行 NFKC 吗？答：不是，只是读取配置声明。问：看到 `Model: BPE` 能证明它匹配 qwen:7b 吗？答：不能。

## 12. 事实锚点

- 原课：[tokenizer-inspect-sop.md](../tokenizer-inspect-sop.md)
- 命令：`cmd/tokenizer-inspect/main.go`
- Go 组件：`internal/tokenizerdemo/inspect.go`
- 测试：`internal/tokenizerdemo/inspect_test.go`
- 前置：[F07](F07-tokenizer-asset-setup.md)、[L01](L01-tokenizer-load-once.md)
