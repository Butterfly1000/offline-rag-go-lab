# L06：完整 rendered prompt 比正文多多少 token

对应工程课：[Tokenizer 小节 06](../prompt-template-token-overhead-sop.md)。本课比较同一份 tokenizer 对两段正文与其整体渲染结果的编码长度。

## 1. 本课一句话

template overhead 不是估算常数，而是 `完整 rendered prompt token 数 - system/user 正文各自 token 数之和`。

## 2. 先用人话理解

正文像信件内容，template 像信封、收件人栏和格式；两封同样正文的信，换信封后总重量可能不同。

## 3. 系统里有什么

- 输入对象：`system` 正文、`prompt` 正文、L05 得到的 rendered prompt。
- 计数器：本地 `tokenizerdemo.Counter`；比较函数：`internal/promptbudget/count.go` 的 `CompareTokens`。
- 输出：`SystemTokens`、`PromptTokens`、`ContentTokens`、`RenderedTokens`、`TemplateOverhead`。
- 公式：`content = system + prompt`；`overhead = rendered - content`。

## 4. 一条完整链路

模型 template + 两段输入 → L05 render 整体字符串 → tokenizer 分别编码 system、prompt、rendered 三个**实际字符串** → 相减得到 overhead。不是把特殊标记列表逐项猜测后相加。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/prompt-budget-demo --base-url http://127.0.0.1:11434 --model qwen:7b --system '你是一个 Go 项目教学助手。' --prompt '解释 token 是如何计算的。'
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/promptbudget
```

- demo：外部只读 + 纯本地计算；依赖 Ollama `/api/show`、模型、template、tokenizer 资产，不调用生成接口或写业务数据，只写 build cache。
- test：纯本地，使用 fake counter 验证比较公式；可安全重复。

## 6. 结果怎么看

输出的 `Content-only total` 是两段正文分别编码后相加；`Rendered prompt tokens` 是渲染后**整个字符串**一次编码；`Template overhead tokens` 是后者减前者。若 rendered 小于 content 或计数失败，应当检查输入/template/counter，而不是把负数解释为“免费 token”。

## 7. 算一遍或走一遍

对本课命令中的同一 system/prompt，使用当前已知 SHA256 `b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8` 的资产与当前 `qwen:7b` template，实测为 `13 + 11 = 24` content、`67` rendered、`43` overhead。公式仍是 `43 = 67 - 24`。这些数值只绑定这份资产、当次模型 template 和这两段输入；资产 SHA、模型/template 或输入任一变化，都应重新运行命令，不能继续期待 43。

## 8. 常见误解

“每个 special token 固定加几个 token，所以可手算 overhead”是错的。BPE 的编码会受相邻字符和边界影响，正确对象是完整 rendered 字符串的一次实际编码。

## 9. 当前实现与生产边界

已保证：对三个明确字符串调用同一个 counter，并返回可解释差值。未保证：多轮 history、retrieval、assistant generation prefix 和输出预留已经进入同一总预算；这些由后续 L07–L12 处理。

## 10. 面试怎么说

30 秒版：我不会只数用户正文，而是先按模型 template 渲染真实输入，再用同一个 tokenizer 比较正文和整体编码；差值就是该次输入的 template overhead。展开点：为什么不是常数、为何分别计数、为什么 render 必须先于 count。

## 11. 自检题与答案

问：43 能直接套到任何 prompt 吗？答：不能，只是当前指定资产、template 与输入的实测示例，变化后应重跑。问：overhead 的计算对象是什么？答：同一 tokenizer 对完整 rendered 字符串的 count 减两段正文 count。问：本命令会生成模型回答吗？答：不会，只读 show。

## 12. 事实锚点

- 原课：[prompt-template-token-overhead-sop.md](../prompt-template-token-overhead-sop.md)
- 命令：`cmd/prompt-budget-demo/main.go`
- Go 组件：`internal/promptbudget/{render,count}.go`、`internal/tokenizerdemo/tokenizer.go`
- 测试：`internal/promptbudget/count_test.go`
- 前置：[L04](L04-ollama-model-metadata.md)、[L05](L05-render-prompt-template.md)
