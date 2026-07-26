# L10：按真实消息格式挑选最近历史

对应工程课：[Tokenizer 小节 10](../recent-window-template-token-sop.md)。本课用格式化后消息的实际 token 选择最近历史，并保持严格不超预算。

## 1. 本课一句话

formatted strict window 从最新消息向前计数；一条放不下就停止，若最新一条自身超限则返回空窗口，选完再恢复时间正序。

## 2. 先用人话理解

拿最近聊天记录装进一个小盒子时，先试最新的一张；能放就继续试更早的。第一张都太大时，盒子里就什么也不放，而不是硬塞一张把盒子撑破。

## 3. 系统里有什么

- `recentchat.NewFormattedTokenBudgetWindowBuilder`：创建带 `QwenFormatter` 的 strict builder。
- `Build(messages, budget)`：返回选中的 `[]Message` 与实际 `used`。
- 每条历史先按 role/content 格式化，再交给 tokenizer 计数。
- 旧的 `NewTokenBudgetWindowBuilder` 是 content-only、non-strict 的历史实现，见 [F08](F08-content-token-window.md)。

## 4. 一条完整链路

历史按旧到新传入 → builder 从最后一条开始 → 格式化该消息并计数 → 若 `used + count` 不超过 budget 则暂存 → 遇到第一条放不下立即停止 → 将暂存结果反转回旧到新顺序后返回。

## 5. 实际怎么做

```bash
sh scripts/regression/lessons-09-10.sh
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/recentchat -run TestFormattedTokenWindowDoesNotForceOversizedNewestMessage -count=1
```

- 两条命令都是纯本地验证：需要 Go、tokenizer 资产和 Go 依赖，不访问 Ollama/MySQL，不写业务数据；只会写可重建 build cache，可安全重复。
- 回归脚本还会验证 L09 的完整对话计数；测试名专门验证“最新消息超预算不能强塞”。

## 6. 结果怎么看

如果最新一条的格式化 token 数为 9、预算为 8，strict builder 的结果是空 `[]Message`、`used=0`。如果最新一条正好放得下，再向前遇到第一条放不下，结果只包含已经放下的较新连续后缀，并且返回顺序仍是旧到新。

## 7. 算一遍或走一遍

设历史按 `[旧A, 新B, 最新C]` 排列，格式化后计数分别为 3、4、5，预算为 9。先取 C，`used=5`；再取 B，`used=9`；试 A 会得到 12，立即停止；内部暂存是 `[C,B]`，反转后返回 `[B,C]`。若预算为 4，C 先放不下，直接返回空，不会改取较旧的 B。

## 8. 常见误解

“每条只数 content 就够了”会漏掉 role 和 ChatML 边界。“从最新往前选”不等于把结果倒序发送；选择方向是新的到旧的，最终交给模型的历史顺序仍必须是旧到新的。

## 9. 当前实现与生产边界

当前 formatted strict 路径保证预算不超，并在 `budget <= 0` 时返回空。它与 [F08](F08-content-token-window.md) 的旧 non-strict content-only 路径故意不同：旧路径在一个都没选中时会保留超预算的最新消息；本课路径绝不这样做。两者不能混用来解释 HTTP 当前的 `used_recent_tokens`。

## 10. 面试怎么说

我会按最终消息协议计算每条历史，而非只数正文；从最新向前累加以优先保留近因，第一条超限就停止，并恢复时间顺序。严格模式下最新单条超限宁可为空，也不违反预算。

## 11. 自检题与答案

问：预算 4、最新消息计数 5 时返回什么？答：空窗口和 0。问：为什么最后要反转？答：算法倒着挑选，但模型需要正常时间顺序。问：F08 旧路径的“强留最新”规则适用于这里吗？答：不适用。

## 12. 事实锚点

- 原课：[recent-window-template-token-sop.md](../recent-window-template-token-sop.md)
- Go 组件：`internal/recentchat/window_token_budget.go`
- 测试：`internal/recentchat/window_token_budget_test.go`
- 回归：`scripts/regression/lessons-09-10.sh`
- 对比：[F08](F08-content-token-window.md)、[L08](L08-qwen-message-format.md)
