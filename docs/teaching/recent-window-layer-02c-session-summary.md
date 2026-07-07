# Recent Window Layer 02C

主题：为什么 `token-budget recent window` 之后还需要 `session summary`

这一段接在：

- [recent-window-layer-02b-token-budget.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02b-token-budget.md:1)

它解决的问题是：

- 如果已经从 count-based 升级到了 token-budget-based
- 为什么还不能直接说“记忆系统已经够用了”

这一章的核心结论只有一句：

**token budget 解决的是“最近上下文怎么更真实地装进去”，但它仍然不能保证“更早的重要信息不会丢”。**

---

## Part 1：token budget 已经解决了什么

到上一段为止，当前项目已经做到：

1. 不再只按 message 条数裁剪
2. 可以在请求前用本地 tokenizer 真实计算 token
3. 可以按 `recent_token_budget` 从最近消息开始往前装

也就是说，当前系统已经解决了：

- recent window 的容量控制
- recent window 的真实成本计算

这比 `recent_limit = 5` 这种按条数裁剪，已经更接近生产。

但这一步的边界也非常明确：

- 它解决的是“最近消息怎么装”
- 不是“更早的重要信息怎么保”

---

## Part 2：为什么 token budget 还不够

假设有这样一段会话：

1. 用户一开始说：`我叫小黄，后面的代码都用 Go 举例。`
2. 中间聊了很多轮实现细节
3. 最近几轮又产生了很多长消息
4. 当前 `recent_token_budget` 已经被最近几轮占满

这时就算 token budget 做得再精确，也还是会发生一件事：

- 最早那条“我叫小黄、都用 Go 举例”的重要约束，仍然可能不在当前窗口里

所以 token budget 的局限不是“算得不准”，而是：

**它天然偏向最近消息。**

它的工作方式决定了：

1. 从最近往前装
2. 装满就停

于是越早的信息，越容易掉出去。

---

## Part 3：session summary 想解决什么

session summary 不是要替代 recent window，而是补它的短板。

你可以把两者分工看成：

### recent window

负责保留：

- 最近发生的原始对话
- 还没有被压缩的上下文
- 模型马上就可能继续依赖的局部连续性

### session summary

负责保留：

- 更早但依然重要的信息
- 用户长期约束
- 已经形成结论的阶段性信息
- 不值得保留原文、但值得保留结论的内容

也就是说：

- recent window 是“最近原文”
- session summary 是“更早结论”

---

## Part 4：如果没有 summary，会具体丢什么

在真实系统里，最容易丢的通常不是“普通闲聊”，而是下面这些高价值信息：

1. 用户身份信息
   - 例如：我叫谁、我的角色是什么

2. 用户长期偏好
   - 例如：后面都用 Go 举例

3. 当前任务约束
   - 例如：不要模拟，要真实落地

4. 已经确认过的阶段性结论
   - 例如：第一层 recent window 已真实跑通

5. 已经决定好的工程方向
   - 例如：先做 A，再做 B

这些内容有一个共同特点：

- 它们不一定是最近说的
- 但后面很多轮都会继续依赖

所以它们最适合进入 summary，而不是永远指望 recent window 留住。

---

## Part 5：session summary 在链路里放哪里

当前 `recent-chat` 链路你已经很熟了：

```text
MySQL message store
-> recent window
-> Ollama
-> 写回 user/assistant
```

如果加入 session summary，生产里的常见思路会变成：

```text
summary
+ recent window
+ current user turn
-> Ollama
```

也就是说，新请求到来时，模型看到的输入不再只有：

- 最近消息
- 当前问题

而是变成：

1. 较早的重要摘要
2. 最近的原始消息窗口
3. 当前用户输入

这样做的效果是：

- 更早的重要信息不需要靠 recent window 硬撑
- recent window 可以专注保最近原文

---

## Part 6：summary 不是整段聊天全文压缩

这里有个很容易误解的点：

session summary 不是把整段聊天简单缩写成一大段“流水账”。

如果只是机械压缩，会有两个问题：

1. 信息太散
2. 重要性不够明确

更合理的 summary 通常更像：

- 当前任务目标
- 用户长期偏好
- 已确认事实
- 当前阶段决定
- 后续仍会依赖的约束

所以好的 summary 不是“聊天缩写版”，而是“高价值状态提炼版”。

---

## Part 7：在我们这个项目里，summary 最自然的第一版怎么做

如果按当前项目节奏继续落地，第一版 summary 最自然的做法不是一上来就做复杂 memory system，而是：

1. 新增一张 summary 表
2. 每个 `session_id` 保留一条当前摘要
3. 到达某个条件时更新摘要
   - 例如：每新增 6~10 条消息
   - 或每完成一轮教学小节
4. 新请求时先读 summary，再读 recent window

第一版 summary 需要保存的内容，可以非常克制：

1. 用户长期偏好
2. 当前阶段目标
3. 已确认的关键结论
4. 仍然有效的任务约束

也就是说，第一版 summary 完全没必要一开始就追求“自动抽取一切记忆项”。

---

## Part 8：当前项目和生产实现差在哪

到这一段为止，当前项目已经具备：

1. MySQL message store
2. count-based recent window
3. token-budget recent window
4. 本地 tokenizer 计数 demo

但还没有：

1. summary store
2. summary 更新策略
3. summary + recent 合并进同一请求链路
4. summary 的结构化字段设计

所以现在最准确的定位是：

**当前项目已经开始解决“最近上下文怎么更真实地装”，下一步才是解决“更早的重要信息怎么留下来”。**

---

## Part 9：这一段学完后应该确认什么

如果这一段真正理解了，应该能确认：

1. token budget 只能更真实地控制 recent window 容量
2. token budget 不能天然保住更早的重要信息
3. session summary 的作用是保留“更早但仍重要”的信息
4. summary 和 recent window 不是二选一，而是分工协作

---

## 这一段的归档定位

这一段是：

- `Recent Window Layer 02C`
- 第 2 层的第三个小段
- 主题是：`为什么 token-budget recent window 之后还需要 session summary`

它后面的自然下一段应该是：

- `session summary 和 long-term memory 的边界`
