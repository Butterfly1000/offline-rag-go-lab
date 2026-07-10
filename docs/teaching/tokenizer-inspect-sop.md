# Tokenizer 小节 02：查看 JSON 结构与组件

这一小节不计算 token，而是回答：

- `tokenizer.json` 里到底配置了哪些阶段？
- 为什么拿到 JSON 后通常不需要自己重写算法？
- 为什么“可以加载”仍不能直接证明“与模型完全匹配”？

---

## Part 1：运行结构检查命令

在项目根目录执行：

```bash
go run ./cmd/tokenizer-inspect
```

也可以显式指定文件：

```bash
go run ./cmd/tokenizer-inspect \
  --tokenizer assets/tokenizers/qwen2/tokenizer.json
```

命令入口：

- [main.go](/offline-rag-go-lab/cmd/tokenizer-inspect/main.go:1)

当前本机文件的实际输出是：

```text
Format version: 1.0
Model: BPE
Normalizer: NFKC
Pre-tokenizer: Sequence
Post-processor: TemplateProcessing
Decoder: ByteLevel
Base vocab entries: 50000
Added tokens: 23944
```

---

## Part 2：这些组件分别做什么

一次简化后的编码流程是：

```text
原始文本
-> Normalizer
-> Pre-tokenizer
-> Model
-> Post-processor
-> token IDs
```

当前文件还配置了 `Decoder`，它主要用于反方向把 token IDs 还原成文本。

### Normalizer：先统一文本

当前值是 `NFKC`。

它会按 Unicode NFKC 规则统一部分看起来相近、编码却不同的字符。这样可以减少同一种文本因为编码形式不同而产生不同 token。

### Pre-tokenizer：准备候选片段

当前值是 `Sequence`。

这表示它不是一条简单规则，而是按顺序执行 JSON 里配置的一组 pre-tokenizer。它负责在真正进入模型切分前准备文本片段。

### Model：真正映射 token

当前值是 `BPE`。

BPE 会使用词表和 merge 规则，把候选片段组合或拆分成 token，并得到 token ID。

### Post-processor：编码后的包装

当前值是 `TemplateProcessing`。

它可以按模板加入 special tokens，或者处理单句、双句编码的边界。本项目目前调用 `EncodeSingle(text, false)`，所以纯文本 demo 没有要求加入 special tokens。

### Decoder：反向还原

当前值是 `ByteLevel`。

它用于把 token 片段重新组合成可读文本。token 计数主要使用编码方向，但完整 tokenizer 仍需要知道如何解码。

---

## Part 3：Go 检查器做了什么

核心代码：

- [inspect.go](/offline-rag-go-lab/internal/tokenizerdemo/inspect.go:1)

它只使用 Go 标准库 `encoding/json`：

1. 读取 `tokenizer.json`
2. 提取顶层组件的 `type`
3. 统计基础词表条目
4. 统计 added tokens

它不会：

- 执行 NFKC
- 执行 BPE
- 自己实现 merge 规则
- 打印完整词表

真正编码仍然交给 tokenizer 库的 `FromFile` 和 `EncodeSingle`。

所以两个工具职责不同：

```text
tokenizer-inspect：看配置、排查兼容性
tokenizer-demo：加载配置并真实编码
```

---

## Part 4：为什么结构不能证明模型匹配

当前文件位于：

```text
assets/tokenizers/qwen2/tokenizer.json
```

但这个目录名只是项目约定，不是文件自身提供的身份证明。

当前文件的真实来源是：

```text
~/Dolphin/hf_model/tokenizer.json
```

我们已经证明：

1. 文件是有效的 Hugging Face 风格 tokenizer 配置
2. Go 库能够加载它
3. 它能够对文本做 token 编码

目前还没有证明：

1. 它来自 Ollama 中 `qwen:7b` 的同一上游模型仓库
2. 它和该 Ollama 模型使用完全相同的词表、merge 规则及 chat template

生产中要建立严格匹配，至少应记录：

- 模型的明确上游来源和版本
- tokenizer 文件来源和版本
- 文件 hash
- 一组与官方 tokenizer 对照的 token IDs 测试样例

---

## Part 5：运行单元测试

执行：

```bash
go test ./internal/tokenizerdemo
```

测试代码：

- [inspect_test.go](/offline-rag-go-lab/internal/tokenizerdemo/inspect_test.go:1)

当前测试覆盖：

1. 能读取各组件类型和统计数量
2. 无效 JSON 会返回错误
3. 能统计对象形式的 BPE 词表
4. 能统计数组形式的 Unigram 词表

---

## 本节重点

1. `tokenizer.json` 描述了一条编码流水线，不是一条“按行切分”规则。
2. 通用程序负责加载并执行配置，开发者通常不需要为每份 JSON 重写算法。
3. 不同 tokenizer 的组件和内部规则会变化，所选库必须支持这些配置类型。
4. 能加载、能计数只证明技术兼容；来源一致和对照测试才能证明模型匹配。
