# Recent Chat Automatic Token Budget SOP

主题：第 12 节，把 token 预算完整接入真实 `/chat`

## 1. 这一节完成了什么

前四节分别实现了：

```text
消息格式
-> 完整 prompt 计数
-> 格式化历史裁剪
-> 自动历史预算
```

第 12 节把它们接入真实服务。调用方不再手填 `recent_token_budget`，只需要表达两个决定：

```json
{
  "auto_token_budget": true,
  "output_token_reserve": 256
}
```

服务会自动读取模型上限、计算固定输入、分配历史预算、选择 MySQL 历史、限制 Ollama 最大回答长度并返回预算明细。

## 2. 请求字段

核心类型：

- [types.go](/offline-rag-go-lab/internal/recentchat/types.go:1)

新增字段：

```go
AutoTokenBudget    bool `json:"auto_token_budget"`
OutputTokenReserve int  `json:"output_token_reserve"`
```

语义：

- `auto_token_budget=true`：让服务自动计算历史预算
- `output_token_reserve`：为本轮模型回答预留的最大 token 数

校验规则：

```text
automatic + manual recent_token_budget -> 拒绝
automatic + output reserve <= 0 -> 拒绝
```

不能同时自动和手动，是因为两套历史预算会产生优先级歧义。

## 3. 三种预算模式

服务返回 `budget_mode`：

```text
count
manual
automatic
```

选择逻辑：

```text
auto_token_budget = true
-> automatic

否则 recent_token_budget > 0
-> manual

否则
-> count
```

旧请求没有被删除：

- `recent_limit` 仍可走 count 模式
- `recent_token_budget` 仍可走 manual 模式
- 新请求才走 automatic 模式

## 4. 自动模式的完整代码链路

核心代码：

- [service.go](/offline-rag-go-lab/internal/recentchat/service.go:1)
- [http.go](/offline-rag-go-lab/internal/recentchat/http.go:1)
- [ollama.go](/offline-rag-go-lab/internal/recentchat/ollama.go:1)
- [main.go](/offline-rag-go-lab/cmd/recent-chat/main.go:1)

### Part 1：构造固定输入

```go
fixed := make([]chatprompt.Message, 0, 2)
if req.SystemPrompt != "" {
    fixed = append(fixed, chatprompt.Message{
        Role:    "system",
        Content: req.SystemPrompt,
    })
}
fixed = append(fixed, chatprompt.Message{
    Role:    "user",
    Content: req.Message,
})
```

固定输入不包含历史，只包含：

- 可选 system prompt
- 当前用户问题
- 规划器自动增加的 assistant prefix

### Part 2：计算历史预算

```go
automaticPlan, err = s.automaticBudget.Plan(
    req.Model,
    fixed,
    req.OutputTokenReserve,
)
historyBudget = automaticPlan.AvailableHistoryTokens
```

数据流：

```text
Ollama /api/show context limit
- tokenizer(fixed prompt + assistant prefix)
- output reserve
= history budget
```

### Part 3：读取候选历史

```go
fetchLimit := req.RecentLimit
if fetchLimit <= 0 && budgetMode != BudgetModeCount {
    fetchLimit = 50
}
```

`recent_limit` 在 automatic 模式中不是最终容量预算，而是 MySQL 候选消息上限。未传时使用教学默认值 `50`，避免一次读取无限历史。

### Part 4：严格选择历史

```go
selected, usedRecentTokens, err = s.tokenWindow.Build(
    recent,
    historyBudget,
)
```

automatic 模式强制要求 strict formatted window。这样：

```text
used_recent_tokens <= available_recent_tokens
```

即使历史一条都放不下，也返回空历史，不突破模型容量。

### Part 5：回答预留必须限制真实生成

```go
ollamaRequest.Options = &OllamaChatOptions{
    NumPredict: req.OutputTokenReserve,
}
```

只在预算公式里减去 `output_token_reserve` 还不够。如果不限制模型，它仍可能生成超过预留的回答。

Ollama `/api/chat` 支持 `options` 运行参数，`num_predict` 表示最大生成 token 数：

- [Ollama Chat API](https://docs.ollama.com/api/chat)
- [Ollama Modelfile Reference](https://docs.ollama.com/modelfile)

因此 automatic 模式让同一个数字同时用于：

```text
预算扣除 + 模型生成上限
```

## 5. 响应字段怎么读

自动模式返回：

```json
{
  "budget_mode": "automatic",
  "context_limit": 32768,
  "fixed_input_tokens": 61,
  "output_token_reserve": 256,
  "available_recent_tokens": 32451,
  "used_recent_tokens": 541,
  "used_messages": 6
}
```

关系一：容量分配

```text
61 + 256 + 32451 = 32768
```

关系二：历史没有超预算

```text
541 <= 32451
```

关系三：`used_messages=6`

说明最终从 MySQL 候选历史中选择了 6 条并发送给 Ollama。

所有预算数字即使为 `0` 也保留在 JSON 中。`0` 可能表示“没有历史容量”，不能因为 `omitempty` 被隐藏。

## 6. 真实运行 SOP

### SOP 0：回归模式选择

默认非业务写入回归：

```bash
sh scripts/regression/lessons-11-12.sh
```

服务已经启动后，执行真实写库回归：

```bash
sh scripts/regression/lessons-11-12.sh --live
```

`--live` 使用独立 `regression-lesson-12-*` session，并向 MySQL 写入一轮 user/assistant
消息。当前回归请求没有开启 `use_memory` 或 `use_knowledge`，因此不会调用 Dual
Retrieval，不要求 Qdrant 或 embedding 模型。

### SOP 1：启动依赖

确认：

- MySQL 中存在 `recent_chat_messages`
- `config/recent-chat.env` 已配置
- Ollama 已启动并有 `qwen:7b`
- tokenizer 资产存在

### SOP 2：启动服务

```bash
go run ./cmd/recent-chat
```

看到：

```text
recent-chat listening on :18093
```

### SOP 3：发送 automatic 请求

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"根据历史说明这个项目使用什么语言。",
    "model":"qwen:7b",
    "recent_limit":50,
    "auto_token_budget":true,
    "output_token_reserve":256,
    "store_user_turn":false,
    "store_assistant_turn":false
  }'
```

这里把两个 store 开关设为 `false`，因此不会新增聊天业务记录；Go 构建过程仍可能更新项目内的 `.cache`。

### SOP 4：核对返回

本次真实结果：

```text
budget_mode: automatic
context_limit: 32768
fixed_input_tokens: 61
output_token_reserve: 256
available_recent_tokens: 32451
used_recent_tokens: 541
used_messages: 6
```

模型回答确认：项目使用 Go。

### SOP 5：验证冲突参数

在 automatic 请求中再加入：

```json
"recent_token_budget": 100
```

请求应返回 `400`，错误包含：

```text
auto_token_budget and recent_token_budget cannot be used together
```

### SOP 6：运行自动测试

```bash
go test ./internal/recentchat
```

主要覆盖：

- 三种 budget mode
- 自动规划器输入
- 默认 fetch limit `50`
- 严格历史选择
- 预算响应字段
- `options.num_predict`
- 参数冲突和错误传播

## 7. 为什么 token 主线到这里算完成

现在项目已经具备一条可运行闭环：

```text
真实 tokenizer 资产
-> Qwen 消息格式
-> 完整固定输入计数
-> Ollama context limit
-> output reserve / num_predict
-> 自动 history budget
-> MySQL recent history
-> 严格 token 裁剪
-> Ollama chat
-> API 预算明细
```

这不代表所有生产优化已经完成，而是表示 token 容量控制已经从“概念和 demo”进入真实聊天链路。

## 8. 已知生产增强

后续优化包括：

- tokenizer 与 Ollama 模型 revision 严格绑定
- 多轮 prompt 与 Ollama 内部 `prompt_eval_count` 黄金对照
- 多模型 registry
- model metadata 缓存
- user/assistant 成对裁剪
- 根据实际 `eval_count` 监控回答预留利用率

这些不会阻塞下一章。下一章进入 `session summary`，解决“旧信息被 recent window 裁掉后如何保留语义”。

## 9. 本节重点

```text
token 主线完成
= 计算正确
+ 裁剪接入
+ 生成上限接入
+ API 可观测
+ 真实链路验证
```
