# L17：按顺序更新一份滚动摘要

对应工程课：[Summary 小节 17](../session-summary-update-sop.md)。本课把前四节摘要能力编排成一条可失败、不会乱推进水位的更新流程。

## 1. 本课一句话

摘要更新必须严格按 source → select → trigger → generate → store 执行；只有最后保存成功，新的 summary 内容、version 与 watermark 才一起生效。

## 2. 先用人话理解

这像修改会议纪要：先取上次纪要，再找之后的原始记录，确认哪些记录已离开桌面，再决定是否值得整理，写好新稿，最后才替换柜中的旧稿。中途失败不能把“已经读到哪里”的书签提前移动。

## 3. 系统里有什么

- source：`MessageSource.ListAfter(session,user,lastMessageID)`，按消息 ID 升序读取水位之后的记录。
- select：L14 `SelectPrefix` 选出 recent window 之前的 Evicted 连续前缀。
- trigger：L13 policy 依据真实条数/token/驱逐数决定是否生成。
- generate：L15 用旧 summary 和 Evicted 消息生成新文本。
- store：L16 用当前 version 保存内容和新 watermark。

## 4. 一条完整链路

`Get current summary` → source 只取 `id > last_message_id` → select → trigger。未触发就结束；触发后才 generate(previous summary + Evicted) → `Save(new summary, expected old version)`。source、select、trigger、generate、store 的顺序不能交换：没有 select 就不知道哪些原文真正离开窗口，没有 store 成功就不能承认水位已推进。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/sessionsummary ./internal/fileconfig ./cmd/summary-update-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/summary-update-demo --config /absolute/path/to/your/recent-chat.env --seed --session-id summary-update-demo-001 --user-id summary-update-user-001 --model qwen:7b --recent-keep 2 --min-messages 4 --min-tokens 100000 --max-output-tokens 256
```

- test 是纯本地 fake 验证，不需 MySQL、Ollama 或私有配置；只可能写 build cache，不写业务数据，可安全重复。
- demo 需要 MySQL、Ollama、tokenizer 和由 `config/recent-chat.env.example` 派生的私有配置。第一次对空的专用 `(session,user)` 加 `--seed` 会写 6 条聊天消息，并在触发后写 `session_summaries`；模型输出不固定。相同专用键重复带 `--seed` 不会再次 seed，但现有状态可能改变结果；恢复前应先按精确键检查，再由操作者处理专用 demo 的消息和摘要。

## 6. 结果怎么看

若选出 6 条未摘要记录、保留最新 2 条，则最旧 4 条成为 Evicted；message 阈值设为 4 时，reason 是 `message_threshold`，成功后 `Updated: true`，watermark 是那 4 条中最后一条的实际 ID。消息表 ID 为全表自增值，不要求从 1 开始或连续。

## 7. 算一遍或走一遍

假设旧摘要为 `version=3, watermark=100`，source 得到 IDs `101,103,104,108`，recent 起点是 104。select 的 Evicted 是 `101,103`，新水位候选为 103；触发后生成新文本，`Save(..., expectedVersion=3)` 成功才得到 `version=4, watermark=103`。若 Save 冲突，100 仍是已提交水位，不能把 103 当作已摘要。

## 8. 常见误解

“模型已经生成新摘要，就可以更新水位”错误，生成不是提交。“冲突时只重试 Save”也错误：另一个请求可能已写入不同摘要，必须从 source 开始重新读取、选择、触发、生成。summary 是同一 session 的压缩，不是跨会话长期事实库；后者从 L19 开始。

## 9. 当前实现与生产边界

未触发、tokenizer/select 失败时不调用 Ollama、也不保存；Ollama 失败时不保存；version conflict 返回错误且不覆盖；仅 Save 成功时返回数据库接受的新 version/watermark。未保证自动合并并发生成的两份摘要或自动清理 demo 数据；这些不能靠静默重试掩盖。

## 10. 面试怎么说

我把滚动摘要拆成 source、select、trigger、generate、store 五个可测接口。watermark 是提交状态，不是模型过程状态：任何生成或并发失败都保留旧版本，成功保存才原子地推进内容和水位。

## 11. 自检题与答案

问：生成器失败后 watermark 能否变成候选新值？答：不能。问：水位 100 后实际 IDs 为 101、103，能否把新水位写成 102？答：不能，必须写最后实际 Evicted 的 ID 103。问：未触发会调用 Ollama 吗？答：不会。

## 12. 事实锚点

- 原课：[session-summary-update-sop.md](../session-summary-update-sop.md)
- Go 组件：`internal/sessionsummary/update.go`、`internal/sessionsummary/source_mysql.go`
- 演示：`cmd/summary-update-demo/main.go`
- 前置：[L13](L13-summary-trigger.md)–[L16](L16-summary-store.md)
