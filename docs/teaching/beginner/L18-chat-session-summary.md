# L18：把摘要、recent 和当前问题一起送进 `/chat`

对应工程课：[Summary 小节 18](../recent-chat-session-summary-sop.md)。本课解释 summary 模式怎样在不超上下文预算的前提下接入真实主对话。

## 1. 本课一句话

summary 模式要求自动预算：先预留 summary 空间并选择 recent、再更新/读取 summary；只有 summary 存在时才把 summary + recent + 当前 user 组成主 chat 并二次计划，否则 recent + 当前 user 直接使用第一次 plan；摘要已保存后主 chat 失败不会回滚摘要。

## 2. 先用人话理解

主 prompt 像一张容量有限的桌子：较早历史先压成一页摘要，最近几页保留原文，当前问题放在最上面。必须先给摘要留座位，否则“摘要有多长”与“最近保留多少”会互相循环决定。

## 3. 系统里有什么

- 请求开关：`use_session_summary=true`，并且必须配合 `auto_token_budget=true` 和正的 `output_token_reserve`。
- reserve：`summary_input_reserve` 为 summary system message（含说明和 ChatML）预留容量；`summary_output_limit` 限制生成摘要的最大输出。
- 主输入顺序：合并后的 system（原 system + summary block）→ selected recent → 当前 user。
- 响应：`session_summary_used`、`session_summary_updated`、`session_summary_version`、`session_summary_watermark`、`session_summary_trigger_reason`。

## 4. 一条完整链路

第一次 automatic plan（当前 system/user + 输出预留）→ 扣掉 `summary_input_reserve` 得到保守 recent 额度 → 选择 recent；若 selected recent 为空，传给 updater 的 `recentStartID=0`，否则传其最早 ID → updater 更新后重新读取 summary。只有 summary `exists=true` 时，才计数 summary block、合并 system、执行第二次 automatic plan，并验证最终 history 容量不小于保守额度；首次未触发等没有 summary 时，直接使用第一次 plan 进入主 Ollama `/api/chat` → 按开关分别写当前 user/assistant。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/recentchat ./internal/sessionsummary ./internal/fileconfig
curl -X POST http://127.0.0.1:18093/chat -H 'Content-Type: application/json' -d '{"session_id":"summary-chat-demo-001","user_id":"summary-chat-user-001","message":"请总结你记得的项目要求。","model":"qwen:7b","auto_token_budget":true,"output_token_reserve":2048,"use_session_summary":true,"store_user_turn":false,"store_assistant_turn":false}'
```

- test 是纯本地 fake 验证，不需私有配置、MySQL 或 Ollama；只写可重建 build cache，可安全重复。
- curl 是 live 外部调用：依赖已启动的 recent-chat、MySQL、Ollama、tokenizer 和由 `config/recent-chat.env.example` 派生的私有配置。两个 store flag 为 false，所以它不写本轮 user/assistant 聊天记录；但若历史发生触发，L17 仍可能写或更新 `session_summaries`。重复同一 `(session,user)` 可能改变 summary version/watermark；恢复前应先精确检查该专用键。

## 6. 结果怎么看

`session_summary_updated=true` 表示 updater 本轮生成并成功保存；`session_summary_used=true` 表示主 chat 实际读到并注入了 summary。version/watermark 是本轮重新读取、真正使用的已提交版本。trigger reason 的完整取值是 `no_evicted_messages`、`message_threshold`、`token_threshold`、`both_thresholds`、`below_threshold`；因此 `used` 与 `updated` 不必总是同值。

## 7. 算一遍或走一遍

假设第一次自动计划给历史 1000 token，`summary_input_reserve=200`，则先只用 800 token 选择 recent。若 updater 保存并 re-read 到 summary，summary block 计为 150 token，才做第二次计划重算完整 system；最终可用历史仍至少 800 时保持同一 recent。若 updater 未触发且 re-read 不存在 summary，则不计数/合并 summary，也不做第二次 plan，直接使用第一次 plan 主 chat。主 chat 随后网络失败时，已成功保存的 summary 仍在数据库中：它不是主 chat 事务的一部分，不能回滚。

## 8. 常见误解

“summary 是一个额外 user 消息”错误，它被包装为 system 中的历史上下文。“先生成 summary 后再随意缩 recent”错误，会改变本次摘要边界；最终容量小于保守额度时服务失败。“主回答失败就撤销摘要”也不对，摘要在主 chat 前已独立 versioned 保存。

## 9. 当前实现与生产边界

已保证：summary 模式没有 automatic budget 会被请求校验拒绝；已存在 summary 超 reserve、存在 summary 时最终预算退化、或 updater/read 失败都会在主 Ollama 前失败；成功主 chat 后 user/assistant 仍按各自 store flag 独立写入。未保证：未触发时会凭空创建 summary、主 chat 失败会回滚已提交 summary，或自动压缩 summary 来强行满足 reserve。

## 10. 面试怎么说

我先留固定 summary reserve 选择 conservative recent，再更新并重读 summary：仅当 summary 存在时才把它放入受预算保护的 system 历史块并二次计数/预算；不存在时 recent + 当前 user 直接使用第一次 plan，最后才调用主模型。这避免循环依赖，也让已保存摘要和主 chat 的失败边界可解释。

## 11. 自检题与答案

问：`use_session_summary=true` 能走 manual token budget 吗？答：不能，必须 automatic。问：selected recent 为空时 updater 收到什么 recentStartID？答：0。问：未触发且没有 summary 时是否仍做第二次 plan？答：不做，直接用第一次 plan。问：主 chat 失败时已保存 summary 会回滚吗？答：不会。

## 12. 事实锚点

- 原课：[recent-chat-session-summary-sop.md](../recent-chat-session-summary-sop.md)
- Go 组件：`internal/recentchat/{types,service,http}.go`
- 测试：`internal/recentchat/service_summary_test.go`
- 前置：[L17](L17-summary-update.md)
