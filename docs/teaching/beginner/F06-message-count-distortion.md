# F06：按消息条数裁剪为什么会失真

对应工程课：[Recent Window Layer 02A](../recent-window-layer-02-count-distortion.md)。本课讨论的是 18093 `recent-chat` 的 count 模式，不是 18092 mock gateway。

## 1. 本课一句话

“保留最近 N 条”能控制条数，却不能控制模型真正付出的 token 成本，也不能保证保住重要信息。

## 2. 先用人话理解

把消息当作行李件数会出错：一张便签和一个大箱子都是“一件”，而最早放进箱子的护照也可能被最新的闲聊挤出去。

## 3. 系统里有什么

- 服务：`cmd/recent-chat`，端口 18093；历史来自 MySQL `recent_chat_messages`。
- 组件：`internal/recentchat/store_mysql.go` 先取最近记录，`CountWindowBuilder` 再保留末尾 N 条。
- 输入：count 模式的 `recent_limit` 必须是正整数；输出的 `used_messages` 与 `recent_window` 是可观察证据。

## 4. 一条完整链路

正整数 `recent_limit` → MySQL 按 `created_at DESC, id DESC` 取最近 N 条 → Go 恢复为旧到新顺序 → 送入 Ollama。它只问“新不新”，不问消息多长、是否重要。

## 5. 实际怎么做

先完成 F04/F05 的配置和服务启动，再在 F05 创建的同一 shell、同一唯一 session/user 中，把第三次请求的 `recent_limit` 设为 `1`。

- 等级：业务写入；若两个写回 flag 为 true，Ollama 成功后会分别尝试写 user/assistant 两条记录。
- 依赖：18093、MySQL、Ollama、tokenizer 资产和已有历史；`recent_limit` 必须为正整数。
- 重复执行：同一键会继续追加历史，窗口随之改变；使用新的测试键可隔离，不能自动回滚写入。

不要用 `0` 表示“不限量”：真实服务会把它带入 MySQL `LIMIT 0`，因此得到空历史；负数不受 count 服务路径支持。不要把 `CountWindowBuilder` 单独面对非正值的行为外推到 HTTP 服务。

## 6. 结果怎么看

`recent_limit=1` 时，`used_messages` 应至多为 1，`recent_window` 只能显示最后一条已存历史。answer 即使碰巧正确，也不能证明早期信息仍在模型输入里。

## 7. 算一遍或走一遍

已有 `[U1: 我叫小黄且偏好 Go, A1: 长解释, U2: 新问题]` 时，limit 为 1 只选 `U2`。三条消息都按“一条”计算；若 `A1` 有 2,000 token、`U1` 有 10 token，count 模式仍不会区分成本。

## 8. 常见误解

“最近就是最重要”是错的。删除早期偏好或任务约束会让回答失去连续性；而“限 10 条就不会超上下文”也错，因为每条长度不同。

## 9. 当前实现与生产边界

已保证：按 user/session 隔离、按最近顺序取数、按条数裁剪。未保证：token 上限、价值判断、摘要保存。下一步 F08 先解决成本计数，F09 再解释为什么仍要 summary。

## 10. 面试怎么说

30 秒版：count window 是最简单的短期记忆策略，但 message count 不是模型成本，且会把“最近”误当“重要”。我会看 `recent_window` 而不是只看 answer 来确认哪些历史真正进了模型。展开点：正整数 limit、双键隔离、为什么 0 不是不限量。

## 11. 自检题与答案

问：`recent_limit=0` 是否保留全部历史？答：不是，真实查询是 `LIMIT 0`。问：10 条短消息与 10 条长消息成本相同吗？答：不相同，count 模式看不见该差异。

## 12. 事实锚点

- 原课：[recent-window-layer-02-count-distortion.md](../recent-window-layer-02-count-distortion.md)
- Go 组件：`internal/recentchat/{service,store_mysql,window_count}.go`
- 测试：`internal/recentchat/window_count_test.go`、`service_test.go`
