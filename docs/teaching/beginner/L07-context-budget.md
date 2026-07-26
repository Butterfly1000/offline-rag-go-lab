# L07：给历史消息留下多少 token 空间

对应工程课：[Tokenizer 小节 07](../context-budget-plan-sop.md)。本课把模型可接受的总长度拆成固定输入、未来输出和最近历史三份。

## 1. 本课一句话

历史预算等于 `context limit - fixed input tokens - output reserve`；若前两项已超过总长度，计划必须失败。

## 2. 先用人话理解

上下文像一个固定大小的行李箱。系统提示词和本轮问题已经放进去了，模型回答还要预留位置；最后剩下的空间才可以装旧对话，不能把箱子当成无限大。

## 3. 系统里有什么

- `ContextLimit`：模型声明的上下文上限。
- `FixedInputTokens`：已经确定会发送的输入 token，例如 system 和本轮 user。
- `OutputReserve`：为模型尚未生成的回答预留的 token。
- `AvailableHistoryTokens`：`internal/promptbudget.Plan` 算出的历史额度。

## 4. 一条完整链路

先取得 context limit，再得到完整固定 prompt 的 token 数，随后选择 output reserve，最后调用 `Plan`。只有三者的关系成立，历史窗口才获得额度；这一步尚不挑选具体历史消息。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/prompt-budget-demo --base-url http://127.0.0.1:11434 --model qwen:7b --system '你是 Go 助手。' --prompt '解释预算。'
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/promptbudget
```

- demo 是外部只读加本地计算：读取 Ollama `/api/show` 与本地 tokenizer，绝不调用生成接口或写业务数据；会写入可重建的 Go build cache，可安全重复。
- test 是纯本地 fake 数据验证；不依赖 Ollama，也不写业务数据，可安全重复。

## 6. 结果怎么看

输出中的 `Available recent history tokens` 就是可以交给历史窗口的上限。它不是模型“已经用掉”的数量，也不是可无限追加的建议值；实际历史还要在 L10 按格式化消息逐条挑选。

## 7. 算一遍或走一遍

假设 context 为 `1000`，固定输入为 `120`，输出预留为 `200`：`1000 - 120 - 200 = 680`，所以历史最多占 680 token。若固定输入为 `850`、输出预留为 `200`，已使用 `1050 > 1000`，`Plan` 返回 `fixed input and output reserve (1050) exceeds context limit (1000)`；这是明确失败，不会把历史额度悄悄改成 0 或负数。

## 8. 常见误解

“历史预算就是 context limit 减本轮问题”不完整：system、检索结果、消息格式和 assistant generation prefix 都可能属于固定输入。“超了就压缩一点”也不是这个函数的行为；本实现先报错，让调用方决定如何缩短。

## 9. 当前实现与生产边界

已保证：正的 context、非负 fixed/reserve 和不超限时的精确减法。未保证：模型宣称的上限必然与任意运行时后端完全一致，或系统自动摘要/删改固定提示来化解超限；这些是上游模型读取和后续策略的职责。

## 10. 面试怎么说

我把上下文预算拆成总容量、已确定输入、未来输出和历史余量：`history = context - fixed - reserve`。固定输入加预留超限时必须显式失败，因为继续选择历史只会掩盖一个无法发送的请求。

## 11. 自检题与答案

问：固定输入 20、预留 30、context 100 时历史额度多少？答：50。问：固定输入加预留大于 context 会怎样？答：返回错误，不产生“负历史额度”。问：本课 demo 会写聊天记录吗？答：不会。

## 12. 事实锚点

- 原课：[context-budget-plan-sop.md](../context-budget-plan-sop.md)
- Go 组件：`internal/promptbudget/budget.go`
- 演示：`cmd/prompt-budget-demo/main.go`
- 测试：`internal/promptbudget/budget_test.go`
- 后续：[L10](L10-template-aware-recent-window.md)、[L11](L11-automatic-history-budget.md)
