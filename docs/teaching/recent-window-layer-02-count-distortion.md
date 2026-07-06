# Recent Window Layer 02A

主题：为什么 `count-based recent window` 会失真

这一层不再讲“最近窗口有没有实现”，而是开始讲：

**为什么“只保留最近 N 条消息”这件事，在真实场景里很快就不够用。**

这一章是第 2 层的第一个小段，目标不是直接把 memory system 全讲完，而是先把一个关键事实讲清楚：

- `最近` 不等于 `重要`
- `按条数裁剪` 不等于 `按上下文价值裁剪`

---

## Part 1：当前实现到底在做什么

当前 `recent-chat` 的最近窗口策略非常直接，代码在：

- [window_count.go](/Users/huangyanyu/offline-rag-go-lab/internal/recentchat/window_count.go:1)

核心逻辑只有这一段：

```go
func (b CountWindowBuilder) Build(messages []Message, maxMessages int) []Message {
    if maxMessages <= 0 || len(messages) <= maxMessages {
        return messages
    }

    return messages[len(messages)-maxMessages:]
}
```

它的含义非常明确：

1. 如果历史消息数量本来就不超过 `maxMessages`，那就全保留
2. 如果历史消息超过限制，就只保留最后 `N` 条

也就是说，当前策略完全不判断：

- 哪条更重要
- 哪条更长
- 哪条是不是用户身份信息
- 哪条是不是后面还会继续依赖的任务约束

它只判断一件事：

**是不是“最新的几条”。**

### Part 1 重点

- 当前项目的 recent window 是 `count-based`
- 裁剪依据只有“条数”
- 当前实现没有 token 概念，也没有 importance 概念

---

## Part 2：为什么它会失真

“失真”在这里的意思不是代码错了，而是：

**模型最终看到的上下文，不再等于这段会话里真正重要的上下文。**

最常见的失真有 3 种。

### 1. 重要信息出现得早，但被后续闲聊挤掉

例如：

1. 用户先说：`我叫小黄，这个项目是 Go 写的。`
2. 中间又聊了几轮别的话题
3. 最后用户问：`你还记得我叫什么吗？`

如果当前 `recent_limit = 1`，模型可能只看到“最后一条历史消息”，而看不到最早那条自我介绍。

问题不在于“模型不聪明”，而在于：

- 这条重要信息根本没被送进去

### 2. 一条很长的消息和一条很短的消息，被当成同样的“1 条”

按条数裁剪时：

- 一条 10 个字的消息，算 1 条
- 一条 1000 个字的消息，也算 1 条

这会带来一个很现实的问题：

- `条数相同` 不代表 `上下文成本相同`

所以 count-based window 在“控制条数”这件事上是稳定的，但在“控制上下文长度”这件事上并不稳定。

### 3. assistant 的最近回答，经常会把真正更关键的 user 约束顶掉

当前实现按时间顺序截最后 N 条，并不会区分：

- user 提供的持久约束
- assistant 临时生成的解释性回答

结果就是：

- 最近一条 assistant 回答，可能被保留下来
- 更早但更关键的 user 约束，反而被裁掉

例如：

- 用户最早说：`后面都用 Go 举例`
- assistant 后面讲了一大段泛泛内容
- 如果只保留最后 1 条，留下的可能只是 assistant 的长回答，而不是用户最初的要求

### Part 2 重点

- `count-based` 的问题不是实现复杂度低，而是它默认“新 = 重要”
- 在真实对话里，这个假设经常不成立

---

## Part 3：当前项目里，失真是怎么发生的

当前业务链路在：

- [service.go](/Users/huangyanyu/offline-rag-go-lab/internal/recentchat/service.go:14)

关键步骤是：

1. `ListRecentBySession(session_id, recent_limit)` 从 MySQL 取最近消息
2. `CountWindowBuilder.Build(...)` 再按 `recent_limit` 取最后 N 条
3. 选中的窗口和本轮用户消息一起发给 Ollama

也就是：

```text
MySQL 原始历史消息
-> recent_limit 裁剪
-> selected recent window
-> Ollama messages
```

真正会失真的位置就是第 2 步：

- 它没有判断“哪些消息应该长期留在上下文里”
- 它只是机械地截最后 `N` 条

所以你可以把这一层理解成：

**当前系统已经解决了“会话连续性”，但还没有解决“会话重要性保留”。**

---

## Part 4：一个可执行验证场景

这一段的目标不是“证明模型一定答错”，而是更确定地证明：

**当 `recent_limit` 很小时，当前系统真的可能把重要信息排除出本轮上下文。**

### 4.1 场景设计

我们故意构造这样一段历史：

1. 第一轮：用户说身份信息
2. 第二轮：assistant 给出一大段说明
3. 第三轮：再问一个依赖第一轮信息的问题
4. 但只给模型 `recent_limit = 1`

这样就能观察到：

- 进入模型的 recent window 里，是否只剩最后一条 assistant 消息
- 更早的身份信息是否已经不在窗口中

### 4.2 执行命令

先发第一轮：

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-layer2-a",
    "user_id":"u-001",
    "message":"我叫小黄，后面的例子都请用 Go 来讲。",
    "model":"qwen:7b",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

再发第二轮：

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-layer2-a",
    "user_id":"u-001",
    "message":"你先随便总结一下 Go 的特点。",
    "model":"qwen:7b",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

最后发第三轮，只给 1 条历史：

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-layer2-a",
    "user_id":"u-001",
    "message":"你还记得我叫什么，以及我希望你后面用什么语言举例吗？",
    "model":"qwen:7b",
    "recent_limit":1,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

### 4.3 重点看什么

重点不要先盯着 `answer`，而是先看响应里的：

- `used_messages`
- `recent_window`

如果当前实现按预期工作，你大概率会看到：

1. `used_messages = 1`
2. `recent_window` 里只剩最后一条历史消息
3. 第一轮那条“我叫小黄，用 Go 举例”的用户约束，已经不在当前窗口里

这时就算模型还能猜对，也不能说明设计没问题，因为：

- 它答对，可能只是碰巧
- 但系统层面已经把重要上下文裁掉了

### 4.4 这一轮验证的结论

这一步验证的不是“模型一定失败”，而是：

**当前系统确实只把最后 1 条消息送给模型，而不会额外保留更早但更重要的信息。**

这就是 count-based recent window 的真实局限。

---

## Part 5：生产里为什么会升级到 token budget

生产系统先升级的，通常不是“让 AI 自己判断一切”，而是先把窗口控制方式从“按条数”升级到“按 token 预算”。

原因很现实：

### 1. 模型真正关心的是 token，不是 message count

模型上下文成本，最终是由 token 决定的，不是由“有几条消息”决定的。

所以生产里更常见的问题是：

- 不是“最多 10 条消息”
- 而是“最近窗口最多占 4000 token”

### 2. token budget 比 count 更接近真实容量约束

例如：

- 最近 2 条消息如果都特别长，可能已经超预算
- 最近 8 条消息如果都很短，可能仍然能安全保留

因此，生产系统更关注：

- 这段 recent window 真实占了多少 token
- 是否还能给 system prompt、tools、retrieval context 预留空间

### 3. token budget 仍然不能解决“重要性”

这里要注意一个关键点：

**token budget 比 count 更真实，但它仍然主要解决“容量控制”，不自动解决“重要信息保留”。**

也就是说：

- count-based 解决的是条数限制
- token-based 解决的是上下文容量限制
- 但“什么该长期保留”还需要下一层机制

---

## Part 6：为什么还要有 session summary

当会话继续变长时，单纯 recent window 就算改成 token-based，也还是会遇到问题：

- 很早以前的重要信息，迟早会被窗口淘汰

这时生产里常见的下一步就是：

- 把较早但仍重要的信息，压成一个 `session summary`

你可以把它理解成：

- recent window：保留最近发生的原始对话
- session summary：保留更早但还重要的会话结论

典型做法是：

1. 最近若干轮保留原文
2. 更早的内容定期摘要
3. 新请求到来时，把 `summary + recent window + current user turn` 一起送进模型

这样做的好处是：

- 不需要无限保留原始消息
- 又能保住更早的重要背景

### Part 6 重点

- token budget 解决“装多少”
- session summary 解决“早期重要信息如何不丢”

---

## Part 7：当前实现和生产实现差在哪

当前项目：

- 有 MySQL message store
- 有 count-based recent window
- 有真实 Ollama 对话
- 能验证 recent window 被真实使用

但还没有：

- token 计算
- token budget 裁剪
- summary store
- session summary 生成与滚动更新
- importance / memory item 提取

所以当前项目的位置可以表述得很准确：

**它已经跨过“完全没有会话记忆”，但还没有进入“生产级上下文管理”。**

---

## Part 8：这一小段学完后，应该得到什么理解

如果这一段真正理解了，应该能说清楚下面 4 件事：

1. 当前 `recent-chat` 的 recent window 是怎么裁剪的
2. 为什么 `recent_limit = 1` 或 `recent_limit = N` 会丢失更早但更重要的信息
3. 为什么生产里会先升级到 token-budget-based window
4. 为什么 token budget 之后还需要 session summary

---

## 这一段的归档定位

这一段是：

- `Recent Window Layer 02`
- 第一个小段
- 主题是：`count-based recent window 会失真`

它后面的自然下一段应该是：

- `什么是 token-budget-based recent window`
