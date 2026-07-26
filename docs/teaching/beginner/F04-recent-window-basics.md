# F04：recent-chat 的最近消息窗口

对应工程课：[Recent Window Layer 01](../recent-window-layer-01.md)。这是独立的 **18093** `recent-chat` 服务，读取 MySQL、调用 Ollama；不要与 F01–F03 的 18092 mock gateway 混用。

## 1. 本课一句话

recent window 让模型在本轮提问前看到同一用户、同一会话最近的原始消息，从而得到最基础的多轮连续性。

## 2. 先用人话理解

像客服接起电话时先读“刚才几句聊天记录”：它只记得最近发生的对话，不等于已经保存了长期偏好或完整历史。

## 3. 系统里有什么

- 服务：`cmd/recent-chat/main.go`，默认端口 18093。
- 数据：MySQL 表 `recent_chat_messages`；隔离键是 `session_id + user_id`。
- 组件：`MySQLMessageStore` 读写、`CountWindowBuilder` 按条数取最近消息、`Service.Chat` 编排、Ollama `/api/chat` 生成。
- 启动配置：默认由 `--config config/recent-chat.env` 指定文件，`fileconfig.Load` 直接解析它；普通 shell 环境变量不会自动覆盖该文件。`RECENT_CHAT_TOKENIZER_PATH` 指向启动时必须能加载的 tokenizer 资产。
- 输入：`message`、`session_id`、`user_id`、`model`、`recent_limit` 与两个写回开关；输出含 `answer`、`used_messages`、`recent_window`。

## 4. 一条完整链路

`POST /chat` → 按 session/user 从 MySQL 查询最近消息并恢复正序 → 保留最后 N 条 → 加当前用户消息调用 Ollama → **仅在 Ollama 成功后**按 `store_user_turn`、`store_assistant_turn` 分别追加 user/assistant → 返回本次实际窗口。

数据是否写入 `recent_chat_messages` 取决于两个写回 flag 和每次独立 INSERT 的结果：Ollama 失败时两条都不写；user INSERT 成功但 assistant INSERT 失败时会留下半轮。Ollama 是外部模型服务。

## 5. 实际怎么做

准备步骤（只引用示例配置的占位值，不在教材中填写真实密码）：

```bash
mysql -u YOUR_USER -p YOUR_DATABASE < sql/recentchat_messages.sql
if test -e config/recent-chat.env; then
  printf '%s\n' 'config/recent-chat.env already exists; inspect it manually and do not overwrite it.'
else
  cp config/recent-chat.env.example config/recent-chat.env
fi
# 人工编辑刚创建或已有的 config/recent-chat.env，填写本机值和可读的 tokenizer 资产路径。
go run ./cmd/recent-chat --config config/recent-chat.env
```

- `mysql < sql/...`：业务写入（创建表）；目标为配置所指数据库的 `recent_chat_messages`。通常可重复执行，但实际权限/已有 schema 由 MySQL 决定；建表副作用不能自动恢复。
- 条件复制：纯本地写入被忽略的配置文件；已有真实配置时只提示并保留，绝不静默覆盖或提交。配置格式/必填 DSN、tokenizer 文件缺失或无法加载会在启动、监听 18093 前失败。
- `go run`：启动本地 HTTP 进程；默认配置路径就是 `config/recent-chat.env`，也可显式 `--config` 指向另一个文件。`sql.Open` 不会 Ping MySQL，Ollama 客户端也在启动时不请求服务，所以数据库连通性、表存在与 Ollama 可用性通常在第一条 `/chat` 的查询/调用时才暴露；启动本身不追加会话消息。

## 6. 结果怎么看

count 模式的 `recent_limit` 必须传正整数。`0` 会被带到 MySQL 的 `LIMIT 0`，导致空历史；负数不受该真实路径支持，不能把 `CountWindowBuilder` 单独面对非正数的行为外推到服务。第一次使用一个新的 `(session_id,user_id)` 且 `recent_limit` 为正数时，`used_messages=0`、`recent_window=[]` 是正常起点。后续同一对键请求时窗口非空才说明真正读到了已存历史。若监听前失败，先检查配置文件、必填 DSN 和 tokenizer 资产；若第一条 chat 才失败，再检查 MySQL 表/连通性和 Ollama。不要把 18092 的健康检查当作 18093 的证明。

## 7. 算一遍或走一遍

设正整数 `recent_limit=2`，某用户同一 session 已有按时间排列的 `[U1,A1,U2]`。数据库先按最新取回，Go 再恢复正序，窗口最终为 `[A1,U2]`，模型随后再看到本轮 `U3`。数字 2 是请求选择，不是模型上下文 token 上限；长短消息仍同样算“一条”。

## 8. 常见误解

“最近 N 条就是最重要 N 条”是错的。该层只偏向最新，早期偏好可能被挤掉；这正是 F06–F09、Session Summary 和 Memory 后续要补的能力。`session_id` 也不能替代 `user_id`：两者共同参与查询隔离。

## 9. 当前实现与生产边界

已保证：本课未启用/未验证可选能力时的 MySQL 持久化、session/user 过滤、时间正序窗口、Ollama 调用和可观察响应。未在本课验证：token 精确预算、summary、memory、检索等可选能力的具体行为。这是更真实的服务，但仍不等于完整生产记忆系统。

## 10. 面试怎么说

30 秒版：recent window 是短期会话记忆，先按用户和会话查出最近原始消息，再连同新问题送入模型，并可把本轮写回 MySQL。它解决连续性但不解决重要性或容量控制。展开点：为什么按 user/session 双重过滤、为何恢复正序、为何 N 条不是 N token。

## 11. 自检题与答案

问：第一轮 `used_messages=0` 是故障吗？答：不是，新会话且 `recent_limit` 为正数时没有历史。问：`recent_limit=0` 能表示“不限量”吗？答：不能，真实查询会变成 `LIMIT 0`。问：`recent_limit=2` 是否保证最多两个 token？答：不保证，它限制的是消息条数。

## 12. 事实锚点

- 原课：[recent-window-layer-01.md](../recent-window-layer-01.md)
- 入口：`cmd/recent-chat/main.go`
- Go 组件：`internal/recentchat/{service,store_mysql,window_count,http}.go`
- 配置/SQL：`config/recent-chat.env.example`、`sql/recentchat_messages.sql`
- 测试：`internal/recentchat/service_test.go`、`store_mysql_test.go`、`window_count_test.go`
