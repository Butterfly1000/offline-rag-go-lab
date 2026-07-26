# F09：为什么 token window 后仍需要 session summary

对应工程课：[Recent Window Layer 02C](../recent-window-layer-02c-session-summary.md)。这是概念桥接课：本课不创建 summary 表、不调用模型、不宣称已经执行 summary 功能。

## 1. 本课一句话

token budget 控制“最近原文装多少”，summary 解决“更早但仍重要的结论如何继续保留”。

## 2. 先用人话理解

recent window 像桌面上摊开的最近几页笔记；summary 像把早期决定写进便签。桌面再大也有限，便签不能代替原始最近页面。

## 3. 系统里有什么

- 已有概念：recent window、token budget、当前用户输入。
- 本课提出的未来对象：同一 session 的滚动摘要、摘要触发条件和摘要存储。
- 本课没有运行入口、表写入、Ollama 请求或新配置；真实实现从 L13 开始进入 `internal/sessionsummary`。

## 4. 一条完整链路

概念上的未来输入是 `较早摘要 + 最近原文窗口 + 当前问题`。摘要来自更早消息的压缩结论，recent window 保留尚未压缩的局部原文；本课仅说明这个分工，不执行它。

## 5. 实际怎么做

本课的正确操作是**不执行业务命令**：不要为本课手建 summary 表、不要把任意聊天全文贴给模型总结。先用 F06/F08 的结果指出“早期约束被裁掉”的现象，下一课序列再学习触发、选择、生成和保存。

- 等级：纯概念阅读；无服务、资产、表或 collection 依赖，也没有可恢复/不可恢复副作用。
- 重复执行：安全；它不改变任何状态。

## 6. 结果怎么看

本课没有 JSON 输出或数据库结果。完成标志是能明确说出：token budget 没有坏掉，它只是天然优先最近消息；summary 的职责是保留较早的高价值结论。

## 7. 算一遍或走一遍

会话先出现“后续都用 Go 举例”，之后 30 条新消息占满 token budget。即使窗口计数完全准确，最早偏好仍可能不在输入。把偏好和已确认任务状态压缩进摘要，recent window 就能继续专注最近原文。

## 8. 常见误解

“summary 就是把完整聊天全文缩短”是错的。摘要应保留仍有效的偏好、决定和约束，而不是无选择的流水账；更不能把它混成跨 session 的长期 memory。

## 9. 当前实现与生产边界

本课只建立设计边界，未验证或启用 summary。后续真实系统需决定何时摘要、哪些连续旧消息可被摘要、如何避免并发覆盖和如何把摘要纳入总 token 预算。

## 10. 面试怎么说

30 秒版：recent window 保留最近原文，summary 保留更早的高价值结论，两者互补。token budget 解决容量却不解决早期信息丢失，因此 summary 不是优化花活，而是分层记忆的职责拆分。展开点：为什么不替代 recent、为什么不是长期 memory、为何需要触发和水位线。

## 11. 自检题与答案

问：本课是否已经把摘要写入数据库？答：没有，这是概念桥接。问：summary 能替代所有 recent 原文吗？答：不能，最近对话的细节仍要由窗口承担。

## 12. 事实锚点

- 原课：[recent-window-layer-02c-session-summary.md](../recent-window-layer-02c-session-summary.md)
- 后续实现入口：`internal/sessionsummary`、`cmd/summary-trigger-demo`
- 前置概念：F06、F08
