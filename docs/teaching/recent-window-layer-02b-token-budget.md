# Recent Window Layer 02B

主题：什么是 `token-budget-based recent window`

这一段接在：

- [recent-window-layer-02-count-distortion.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02-count-distortion.md:1)

它解决的问题是：

- 如果已经知道按 message 条数裁剪会失真
- 那么在当前项目里，怎么把 recent window 升级成按 token 预算裁剪

---

## Part 1：这次实现新增了什么

当前 `recent-chat` 现在已经支持一个新的请求字段：

- `recent_token_budget`

对应代码在：

- [types.go](/offline-rag-go-lab/internal/recentchat/types.go:1)

当前语义是：

- `recent_limit`：控制最多从 MySQL 取多少条历史消息
- `recent_token_budget`：控制最终 recent window 最多保留多少 token

也就是说，当前实现不是“完全废掉 `recent_limit`”，而是：

**先按条数限制数据库读取范围，再按 token 预算做第二次裁剪。**

---

## Part 2：代码链路怎么变了

核心代码在：

- [service.go](/offline-rag-go-lab/internal/recentchat/service.go:1)
- [window_token_budget.go](/offline-rag-go-lab/internal/recentchat/window_token_budget.go:1)

当前逻辑是：

1. 先按 `session_id` 从 MySQL 查最近消息
2. 如果没有传 `recent_token_budget`
   - 继续走老的 `CountWindowBuilder`
3. 如果传了 `recent_token_budget`
   - 改走 `TokenBudgetWindowBuilder`

所以它保持了向后兼容：

- 老请求还能继续用
- 新请求可以开始验证 token budget

---

## Part 3：token budget 版到底怎么裁剪

当前实现不是按 message 条数裁剪，而是：

1. 从最近一条历史消息开始往前看
2. 每条消息先算出 `message.Content` 的 token 数
3. 逐条累加
4. 一旦再加一条就超过预算，就停止

也就是：

```text
从最新消息开始
-> 计算 token
-> 累加 token
-> 超预算就停
-> 再翻回正序送给模型
```

这就是 token-budget recent window 的核心动作。

---

## Part 4：当前项目里，token 是怎么算的

当前实现复用了这次 A 阶段已经跑通的本地 tokenizer 组件：

- [internal/tokenizerdemo/tokenizer.go](/offline-rag-go-lab/internal/tokenizerdemo/tokenizer.go:1)

运行入口在：

- [cmd/recent-chat/main.go](/offline-rag-go-lab/cmd/recent-chat/main.go:1)

当前通过环境变量指定 tokenizer 路径：

```text
RECENT_CHAT_TOKENIZER_PATH=assets/tokenizers/qwen2/tokenizer.json
```

也就是说，当前 token-budget window 不是：

- 估算字符数
- 也不是请求后才看 usage

而是：

**请求前就用本地 tokenizer 对每条历史消息做真实 token 计数。**

---

## Part 5：怎么运行这版 recent-chat

先确认 `config/recent-chat.env` 至少包含：

```text
RECENT_CHAT_MYSQL_DSN=root:123456@tcp(127.0.0.1:3306)/offline_rag?parseTime=true
OLLAMA_BASE_URL=http://127.0.0.1:11434
RECENT_CHAT_TOKENIZER_PATH=assets/tokenizers/qwen2/tokenizer.json
```

再启动：

```bash
go run ./cmd/recent-chat
```

如果本机默认 Go 缓存目录权限有问题，也可以像这次验证一样显式指定：

```bash
env GOCACHE=/private/tmp/offline-rag-go-lab-gocache GOSUMDB=off go run -mod=mod ./cmd/recent-chat
```

---

## Part 6：怎么验证 token-budget window

你可以直接发：

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-token-001",
    "user_id":"u-001",
    "message":"你现在基于还能看到的历史，总结一下你记得的内容。",
    "model":"qwen:7b",
    "recent_limit":10,
    "recent_token_budget":20,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

重点看返回里的：

- `used_messages`
- `used_recent_tokens`
- `recent_window`

这 3 个字段组合起来就能说明：

1. 最终保留了几条消息
2. 最终用了多少 token
3. 进入模型的到底是哪几条历史消息

---

## Part 7：当前实现的边界

这次 B 虽然已经把 token budget 接进 `recent-chat`，但它还有明确边界：

1. 当前主要按 `message.Content` 计数
2. 还没有应用完整 chat template
3. 还没有把 `system prompt` / retrieval context / output reserve 合并进统一预算
4. 还没有 session summary

所以当前 B 的定位应该说得很准确：

**它已经从 count-based 升级到了本地真实 tokenizer 驱动的 token-budget recent window，但还不是完整生产版上下文预算系统。**

---

## Part 8：这一段学完后应该确认什么

如果这一段真正理解了，应该能确认：

1. `recent_limit` 和 `recent_token_budget` 现在是两层限制
2. token-budget window 的本质是“从最近往前装，装满预算为止”
3. 当前项目已经能在请求前用本地 tokenizer 真实计数
4. 这一步解决的是“容量控制更真实”，不是“重要性保留已经解决”
