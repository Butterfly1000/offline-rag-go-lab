# L14：只把真正离开窗口的连续旧消息交给摘要

对应工程课：[Summary 小节 14](../session-summary-selection-sop.md)。本课从 watermark 后的消息中找出 recent window 之前的连续前缀，并安全推进水位。

## 1. 本课一句话

候选是 watermark 后的消息；摘要输入只能取 recent 起点之前的连续前缀，ID 可以有空洞，水位取最后一条实际被选消息的 ID。

## 2. 先用人话理解

watermark 像书签，recent window 像还摊在桌上的最后几页。摘要只能抄书签之后、桌上页面之前的连续章节；不能跳过其中一页后却把书签翻过它。

## 3. 系统里有什么

- `SelectPrefix(messages, lastMessageID, recentStartID, counter)`：选择器。
- `Unsummarized`：watermark 后全部实际消息。
- `Evicted`：其中位于 recent 起点前的连续前缀。
- `NextWatermark`：本次 `Evicted` 的最后一个实际消息 ID；没有驱逐时保持旧水位。

## 4. 一条完整链路

消息按严格递增 ID 输入 → 过滤 `id > watermark`。源码有四个边界分支：`recentStartID<0` 是非法输入，立即校验失败；`recentStartID=0` 表示 recent window 为空，全部 Unsummarized 都是 Evicted，存在未摘要消息时水位推进到最后一条实际消息 ID；`0<recentStartID<=watermark` 表示 recent window 已伸入旧摘要，Evicted 为空、保持旧水位且不查找该 ID；只有 `recentStartID>watermark` 时才必须在过滤后的 Unsummarized 中找到它，位于它之前的连续前缀才是 Evicted，找不到则报错。每条未摘要消息格式化并只计数一次，同时累计总 token 与前缀 token，最后用实际前缀末尾 ID 推进水位。

## 5. 实际怎么做

```bash
sh scripts/regression/lessons-13-15.sh
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/summary-selection-demo --tokenizer assets/tokenizers/qwen2/tokenizer.json --ids '19,20,21,23,30' --watermark 20 --recent-start 30
```

- 回归与 demo 都依赖 Go、tokenizer 资产和 Go 依赖；demo 是纯本地，不访问 Ollama/MySQL、不写业务数据，只写 build cache，可安全重复。
- 完整回归还会在 L15 调用 Ollama `/api/chat` 生成摘要，因此它额外需要 curl、运行中的 Ollama/model；不写 MySQL 业务数据，输出文本不可保证逐字重复。

## 6. 结果怎么看

对 IDs `19,20,21,22,23,24,25,26`、watermark `20`、recent start `25`，未摘要是 `21,22,23,24,25,26`，Evicted 是 `21,22,23,24`，新水位是 `24`。recent start 为 0 表示 recent window 为空，此时所有未摘要消息都成为 Evicted，水位推进到 26；recent start 为 1–20 时 Evicted 为空，新水位保持 20；负数 recent start 是输入错误。

## 7. 算一遍或走一遍

ID 可空洞：`19,20,21,23,30`，watermark 为 20、recent start 为 30 时，Evicted 是 `21,23`，而非虚构的 `21,22,23`；新水位为实际最后 ID `23`。若 recent start 为 31（大于 watermark，却不在 Unsummarized 中），选择器报错；若 recent start 为 20，则不查找 20，返回空 Evicted 和旧水位 20；若为 0，则驱逐 21、23、30 并推进水位到 30。当前 SHA `b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8` 的资产对标准例实测未摘要 `129` token、Evicted `86` token；数值绑定资产、消息内容与 formatter，变化后应重跑。

## 8. 常见误解

“recent start 小于等于 watermark 都按空 Evicted 处理”错误：只有 `0<recentStartID<=watermark` 才这样；0 表示 recent window 为空，全部未摘要消息会被驱逐，负数则校验失败。“recent start 找不到就总是报错”也错误：只有它大于 watermark 时才在 Unsummarized 中查找。不能任意挑旧消息摘要，否则跳过的消息会永久落在 watermark 前。ID 可有空洞但实际输入必须严格递增；token 不能因 Evicted 又再编码一次，选择器每条未摘要消息只计数一次。

## 9. 当前实现与生产边界

已保证：负数 recent start、recent start 大于 watermark 却缺失、负数水位、非递增 ID 或 tokenizer/格式化失败会报错，避免错误推进水位。未保证：该函数自己从 MySQL 读消息、决定是否触发或保存摘要；这些分别由消息源、L13 和 L16 负责。

## 10. 面试怎么说

摘要输入不是“所有旧消息”，而是 watermark 后到 recent 起点前的连续真实消息前缀。我允许 ID 空洞但不允许跳选，token 每条算一次，新 watermark 指向实际最后被摘要的 ID。

## 11. 自检题与答案

问：水位 20、Evicted 为 21 和 23 时新水位是多少？答：23。问：recent start 为 0 时会怎样？答：recent window 为空，全部 Unsummarized 被驱逐，水位推进到最后实际 ID。问：recent start 31 大于水位 20、却找不到时能否继续？答：不能，应报错；1–20 则不查找并返回空 Evicted，负数直接校验失败。问：为什么不取 25、26？答：它们仍在 recent window，有原文可用。

## 12. 事实锚点

- 原课：[session-summary-selection-sop.md](../session-summary-selection-sop.md)
- Go 组件：`internal/sessionsummary/{select,token}.go`
- 演示：`cmd/summary-selection-demo/main.go`
- 回归：`scripts/regression/lessons-13-15.sh`
- 前置：[L13](L13-summary-trigger.md)
