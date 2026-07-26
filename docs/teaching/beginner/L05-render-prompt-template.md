# L05：用模型自己的 template 渲染 prompt

对应工程课：[Tokenizer 小节 05](../prompt-template-render-sop.md)。本课只渲染，不把包装字符串手写到业务代码，也不调用模型生成。

## 1. 本课一句话

项目从模型元数据取得 template，再用 Go `text/template` 把本次 system 与 user prompt 填进去，得到模型真正看到的包装文本。

## 2. 先用人话理解

template 像厂商给的表单：项目只填“系统指令”和“用户问题”两个格子，不自己重画表单边框或猜角色标记。

## 3. 系统里有什么

- 模板来源：L04 的 Ollama `/api/show` 响应 `template` 字段。
- 渲染函数：`internal/promptbudget/render.go` 的 `Render`。
- 数据对象：`TemplateData{System, Prompt}`；模板可用 `{{ .System }}`、`{{ .Prompt }}` 和条件 `{{ if .System }}`。
- 命令：`cmd/prompt-budget-demo`；它先 show、再 render、随后还会做计数/预算展示，L05 只关注其中 rendered prompt。

## 4. 一条完整链路

`/api/show` 的模型 template → `text/template.Parse` → `TemplateData` → `Execute` → rendered prompt。模型换了，命令读取新 template；项目不维护一份固定的 `<|...|>` 包装常量。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/prompt-budget-demo --base-url http://127.0.0.1:11434 --model qwen:7b --system '你是一个 Go 项目教学助手。' --prompt '解释 token 是如何计算的。'
```

- 等级：外部只读 + 纯本地计算；依赖 Ollama、模型、tokenizer 资产与 Go。该命令只读 `/api/show`，不调用 `/api/chat`、不生成回答、不写业务数据；会写 build cache。
- 重复执行：安全；同一模型 template、资产与输入下 rendered 文本应相同。

## 6. 结果怎么看

看 `Rendered prompt:` 后的完整文本。system 非空时可看到 system 区块；system 为空时由模板条件省略该区块。若模板语法无效会失败，说明不能可靠渲染；不要以手写包装来掩盖失败。

## 7. 算一遍或走一遍

template 若含 `{{ if .System }}`，System 为“规则 A”时输出 system 包装和内容；System 为空时该段完全不输出。无论哪种，`{{ .Prompt }}` 都填入当前用户 prompt。这是模板行为，不是项目对某个角色标记的硬编码。

## 8. 常见误解

“既然当前模型看起来用 `<|im_start|>`，就能永远手写它”是错的。不同模型的 role、换行、条件和 assistant 前缀都可不同；手写很容易在升级时悄悄产生错误 token 输入。

## 9. 当前实现与生产边界

已保证：从 show 所得 template 用标准库解析、带当前两项数据执行，并对缺失键/语法错误报错。未保证：这个 template 与本地 tokenizer 资产同源、完整多轮 conversation 已被正确构造；这些在后续课程继续验证。

## 10. 面试怎么说

30 秒版：prompt template 应归模型所有，应用只负责读取并渲染。这里用 Go text/template 和 `System/Prompt` 数据对象，避免把模型专属 wrapper 写死。展开点：为何 system 为空可省略、为何 render 不等于 generate、为何 template 会影响 token。

## 11. 自检题与答案

问：本命令会调用 `/api/chat` 吗？答：不会，只读 `/api/show`。问：template 来自项目常量吗？答：不是，来自模型元数据。问：渲染成功是否已证明 token 数正确？答：还没有，L06 才比较计数。

## 12. 事实锚点

- 原课：[prompt-template-render-sop.md](../prompt-template-render-sop.md)
- 命令：`cmd/prompt-budget-demo/main.go`
- Go 组件：`internal/promptbudget/render.go`、`internal/recentchat/ollama.go`
- 测试：`internal/promptbudget/render_test.go`
- 前置：[L04](L04-ollama-model-metadata.md)
