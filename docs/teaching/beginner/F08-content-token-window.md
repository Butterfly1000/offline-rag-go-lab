# F08：content token window 是过渡方案

对应工程课：[Recent Window Layer 02B](../recent-window-layer-02b-token-budget.md)。本课说明历史的 content-only 过渡实现；它不等于后续完整 ChatML/template 计数。

## 1. 本课一句话

把窗口从“最近 N 条”升级为“按内容 token 装入”能控制文本成本，但旧 non-strict 版本有一个软预算例外：最新一条单独超 B 时仍会被保留，因此 `used` 可大于 B。只数 `message.Content` 也仍会漏掉 role、消息边界和模板开销。

## 2. 先用人话理解

这是先给行李称正文重量，却还没称包装、标签和运输箱。比只数件数好，但不是最终总重量。

## 3. 系统里有什么

- 过渡构造器：`NewTokenBudgetWindowBuilder` 的非 strict 形态会对 `message.Content` 计数。
- 组件：`internal/recentchat/window_token_budget.go`、`internal/tokenizerdemo`。
- 请求概念：`recent_limit` 控制取回候选数量，`recent_token_budget` 控制本阶段装入的内容 token。
- 旧软预算例外：尚未选中消息时，最新一条单独超预算仍强制保留；这**是唯一**允许 `used > budget` 的情况。
- 现状边界：当前 `cmd/recent-chat` 已接到后续的格式化 strict builder；strict 不会强塞超限最新消息，而是返回空窗口。不要把本课的 content-only 行为或旧 `used` 直接说成当前 HTTP `used_recent_tokens` 的运行证据。

## 4. 一条完整链路

候选历史 → 从最新向前取每条 `Content` 的 tokenizer count → 能装入就累加 → 超预算则停止并反转为旧到新顺序 → 送入模型。唯一例外是第一条（最新）已经超 B：旧 non-strict 构造器仍保留它并让 `used > B`；本历史阶段还漏算 role、边界与 chat template。

## 5. 实际怎么做

运行不访问外部服务的回归：

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/recentchat -run 'TestTokenBudgetWindowBuilderKeepsNewestMessagesWithinBudget|TestTokenBudgetWindowBuilderKeepsSingleNewestMessageWhenItAloneExceedsBudget'
```

- 等级：纯本地；依赖 Go、tokenizer 测试依赖与已预置 module cache；只会写仓库内 Go build cache。
- 重复执行：安全；测试不写 MySQL/Ollama/会话数据。
- 若运行真实 HTTP 验证，仍需 F04 的服务依赖，并应使用正整数 `recent_limit`；写回 flag 为 true 时会产生真实数据库副作用。

## 6. 结果怎么看

成功测试说明旧选择器从最新向前装、通常超预算停止并恢复正序，同时验证唯一软预算例外。测试中的 `budget=2`、最新 content=3 token 时，仍保留这条并得到 `used=3`。当前 HTTP 的 `used_recent_tokens` 来自后续格式化 strict builder，不能把它当作旧 content-only `used` 的直接运行证据；两者都不能代表完整模型 prompt 的总 token。

## 7. 算一遍或走一遍

通常情形：预算 B=20，最新三条 content 分别为 8、9、7 token。倒序装入先得 8，再得 17；再加 7 会到 24，所以保留两条并翻回正序。唯一例外：B=2、最新单条为 3 token 时，旧 non-strict 逻辑仍保留这条，`used=3`。若真实模板还为每条增加边界 token，20 不是最终请求的精确成本。

## 8. 常见误解

“本地 tokenizer 已经接入，所以预算必然精确”是错的。tokenizer 可以真实地数正文，却仍未被要求数 role/模板。删掉 token window 会回到只按条数，成本波动更大。

## 9. 当前实现与生产边界

本过渡层已解决：用真实 tokenizer 代替字符估算、从最新向前装、输出恢复正序，并以“至少保留最新一条”为代价允许一次软超限。未解决：完整消息格式、严格不超限、system/retrieval/current user/output 的统一预算；后续格式化 strict builder 对超限最新消息返回空窗口，L08–L12 再逐步补齐总预算。

## 10. 面试怎么说

30 秒版：content-token window 比 count window 更接近模型容量，因为它按 tokenizer 计正文；但旧 non-strict 版本为保证至少一条最新消息可出现一次 `used > budget`，正文也不是完整 prompt，所以只能算过渡预算。展开点：唯一超限例外、strict 为什么返回空窗口、为什么模板开销会漏算。

## 11. 自检题与答案

问：B=20 时 8+9+7 的三条都会入选吗？答：不会，最后一条会使累计到 24。问：旧 non-strict 里 B=2、最新一条=3 时会返回空吗？答：不会，唯一例外会保留它并使 used=3；strict builder 则返回空窗口。问：content token 等于完整 chat token 吗？答：不等于，role 和模板仍未计入。

## 12. 事实锚点

- 原课：[recent-window-layer-02b-token-budget.md](../recent-window-layer-02b-token-budget.md)
- Go 组件：`internal/recentchat/window_token_budget.go`、`internal/tokenizerdemo/tokenizer.go`
- 测试：`internal/recentchat/window_token_budget_test.go`
