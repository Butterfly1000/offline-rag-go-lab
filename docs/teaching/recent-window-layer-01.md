# Recent Window Layer 01

主题：第一层真实会话记忆现在已经落地成一个独立的 recent-chat 服务。

这一层先解决一件事：把同一个 `session_id` 下最近几轮消息从 MySQL 里取出来，按时间顺序拼成 recent window，再把这段上下文和本次用户消息一起发给 Ollama。

---

## Part 1：这一层到底实现了什么

当前第一层不是完整 memory system，而是一个独立、可运行的最近窗口层，入口在 [cmd/recent-chat/main.go](/Users/huangyanyu/offline-rag-go-lab/cmd/recent-chat/main.go:1)，核心实现集中在 [internal/recentchat](/Users/huangyanyu/offline-rag-go-lab/internal/recentchat)。

它现在已经具备 4 个最小能力：

- 用 MySQL 表 `recent_chat_messages` 持久化消息
- 按 `session_id` 读取最近消息
- 按 `recent_limit` 截取最近窗口
- 把 recent window 和当前用户消息一起发给 Ollama

你可以把这条链路先理解成：

`POST /chat -> 读最近消息 -> 截窗口 -> 调 Ollama -> 可选写回 user/assistant turn`

这就是第一层的边界。它只关心“最近聊了什么”，还不关心长期记忆、摘要记忆、候选知识沉淀或检索增强。

### Part 1 重点

- `MySQLMessageStore` 负责消息读写。
- `CountWindowBuilder` 负责最近窗口截取。
- `Service.Chat(...)` 负责把 store、window、ollama 串起来。
- `/chat` 返回 `answer`，也返回这次实际使用的 `recent_window`。

---

## Part 2：当前怎么使用

### 1. 先建表

在 MySQL 里执行：

```sql
SOURCE sql/recentchat_messages.sql;
```

这会创建：

- `session_id`
- `user_id`
- `role`
- `content`
- `created_at`

也就是 recent window 这一层真正依赖的最小消息表。

### 2. 配置并启动 recent-chat 服务

```bash
cd /Users/huangyanyu/offline-rag-go-lab
mkdir -p config
cp config/recent-chat.env.example config/recent-chat.env
```

然后编辑 `config/recent-chat.env`，至少填写：

```text
RECENT_CHAT_MYSQL_DSN=root:123456@tcp(127.0.0.1:3306)/offline_rag?parseTime=true
OLLAMA_BASE_URL=http://127.0.0.1:11434
```

再启动：

```bash
cd /Users/huangyanyu/offline-rag-go-lab
go run ./cmd/recent-chat
```

默认监听：

```text
http://127.0.0.1:18093
```

### 3. 发起一次 recent-window chat

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"帮我总结一下我们刚才聊了什么",
    "model":"llama3",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

如果链路正常，响应里会看到几类关键信息：

- `answer`：Ollama 返回的回答
- `used_messages`：这次 recent window 实际用了几条消息
- `recent_window`：这次送进模型之前的历史消息窗口

---

## Part 3：你现在应该怎么理解它

这层的重点不是“记忆已经很强”，而是“真实最近消息链路已经打通”。

也就是说，当前项目已经从教学版 mock chat 之外，多了一条真实会话路径：

- 消息会落到 MySQL
- 新请求会把最近消息再读出来
- 模型看到的不再只有当前一句话，而是最近几轮上下文

这就是第一层 recent window 的价值：先让多轮对话具备最基础的连续性。

---

## Current Usage Notes

- `recent_limit` 决定最多带多少条历史消息进入当前请求。
- `store_user_turn` 和 `store_assistant_turn` 决定这次对话是否写回数据库。
- `system_prompt` 可选；有值时会先进入 Ollama 消息列表。
- 当前窗口策略是“按条数截最近消息”，不是按 token、主题或摘要裁剪。
- 当前运行入口要求 `RECENT_CHAT_MYSQL_DSN` 存在。
- 默认会优先读取 `config/recent-chat.env`；如果 shell 里已存在同名环境变量，则以 shell 为准。

## Current Real Limitations

- Recent window 仍然是按消息条数裁剪，不是按 token budget 裁剪。
- 这一层只解决“最近对话连续性”，还没有 session summary。
- 这一层还没有 long-term memory，也没有 memory item 提取。
- 当前 `/chat` 只把 recent window 发给 Ollama，还没有把知识检索结果和 recent memory 合并进同一个真实服务链路。

## Next Upgrade Path

1. 把 `CountWindowBuilder` 升级成 token-budget-based recent window。
2. 增加 session summary 表和滚动摘要更新逻辑。
3. 增加 memory item 提取、分类、去重和写入。
4. 把高价值 memory items 向量化，并和文档知识一起做召回。

---

## Layer 01 归档结论

状态：已学会，且已完成真实运行验证。

本层已经被验证的事实：

1. 第一轮请求没有历史消息时，`used_messages = 0`，`recent_window = []`。
2. 第一轮结束后，user/assistant 两条消息会真实写入 MySQL。
3. 第二轮同一 `session_id` 请求时，系统会从 MySQL 读出历史消息，`used_messages > 0`，`recent_window` 非空。
4. 当 `recent_limit = 1` 时，当前实现会只保留最后一条历史消息，证明最近窗口裁剪已经真实生效。

这一层的最终理解：

- 第 1 层解决的是“最近对话连续性”。
- 第 1 层已经真实可运行，不再只是设计。
- 第 1 层的裁剪规则当前是“按消息条数”，不是“按 token 预算”。
- 第 1 层不等于真正可用的长期记忆系统，它只是 memory system 的第一层基础。
