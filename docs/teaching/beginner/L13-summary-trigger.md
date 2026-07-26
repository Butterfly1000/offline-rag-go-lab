# L13：什么时候值得生成会话摘要

对应工程课：[Summary 小节 13](../session-summary-trigger-sop.md)。本课定义摘要保存什么、已经处理到哪里，以及何时才值得调用模型生成新摘要。

## 1. 本课一句话

摘要记录 content、watermark 和 version；只有存在已被 recent window 驱逐的未摘要消息，并达到消息或 token 阈值时才触发。

## 2. 先用人话理解

滚动摘要像一本会议纪要：内容是仍有效的结论，watermark 是“已记到第几条”，version 是“第几次修订”。原文还在眼前时不急着写纪要；只有旧原文被窗口挤出去后，纪要才补上空缺。

## 3. 系统里有什么

- `SessionSummary.Content`：滚动摘要正文，不是完整聊天转录。
- `LastMessageID`：摘要已覆盖的最后一条消息 ID，即 watermark。
- `Version`：每次成功保存后的版本号，用于后续乐观锁。
- `TriggerInput`：未摘要消息数、未摘要 token 数、已驱逐消息数。

## 4. 一条完整链路

L14 给出 watermark 后的真实消息、token 和已驱逐前缀 → `TriggerPolicy.Decide` 先检查统计是否合法 → `evicted_messages=0` 直接返回 `no_evicted_messages` → 否则按 message/token 阈值决定是否进入 L15 生成。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/summary-trigger-demo --messages 10 --tokens 5000 --evicted 0
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/sessionsummary -run TestTrigger -count=1
```

- 两条均为纯本地检查：依赖 Go 与项目依赖，不需要 tokenizer、Ollama、MySQL 或配置；只可能写入可重建 build cache，不写业务数据，可安全重复。
- demo 的 flags 是人为输入的统计，用来观察决策，不会查询聊天表或生成摘要。

## 6. 结果怎么看

默认阈值为 `min_messages=8`、`min_tokens=2048`。若 `messages=10`、`tokens=5000`、`evicted=0`，尽管两个阈值都超过，仍输出 `Should summarize: false` 和 `Reason: no_evicted_messages`。这是“驱逐前置”规则，不是阈值计算错误。

## 7. 算一遍或走一遍

水位为 ID 20、当前真实未摘要消息为 21–28，则消息数是 8（实际生产统计，不能单靠 `28-20` 猜）。若其中有 2 条已经离开 recent window、总 token 为 1000，触发 message threshold；若只有 3 条但 token 为 3000 且驱逐数为 1，触发 token threshold。`evicted=0` 时两种算例都不触发。

## 8. 常见误解

“消息数达到阈值就一定摘要”错误，驱逐是前置条件。“token 阈值与 message 阈值必须同时达到”也错误；在已发生驱逐后两者是 OR，两个都达到时只返回更具体的 `both_thresholds` 原因。

## 9. 当前实现与生产边界

已保证：负数、`evicted > unsummarized` 等不可能统计会失败；`evicted=0` 不触发；reason 可解释。未保证：本 policy 自己读取 MySQL 或计算 token，它只消费 L14 已提供的真实统计，也不会在本节直接生成或保存摘要。

## 10. 面试怎么说

我把摘要触发拆为状态和策略：摘要带 watermark/version；只有原文已被 recent window 驱逐，才用“消息数 OR token 数”阈值决定是否摘要。这样既避免重复摘要，又覆盖少量超长消息。

## 11. 自检题与答案

问：`evicted=0`、`messages=100`、`tokens=99999` 是否触发？答：不触发。问：version 的用途是什么？答：后续保存时识别并拒绝并发覆盖。问：本 demo 会调用模型吗？答：不会。

## 12. 事实锚点

- 原课：[session-summary-trigger-sop.md](../session-summary-trigger-sop.md)
- Go 组件：`internal/sessionsummary/{types,trigger}.go`
- 演示：`cmd/summary-trigger-demo/main.go`
- 测试：`internal/sessionsummary/trigger_test.go`
- 后续：[L14](L14-summary-message-selection.md)、[L15](L15-summary-generation.md)
