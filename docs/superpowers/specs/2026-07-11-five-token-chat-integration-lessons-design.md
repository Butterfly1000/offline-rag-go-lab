# Five Token Chat Integration Lessons Design

日期：2026-07-11

## 目标

连续实现第 08 到第 12 节，把已经完成的 tokenizer、模型上下文、模板开销和预算计算接入真实 `recent-chat` 链路。

第 12 节结束时，token 教学主线必须形成下面的闭环：

```text
模型 tokenizer
-> 消息角色与边界格式
-> 完整固定输入 token
-> 模型上下文上限
-> 回答预留
-> 自动得到历史预算
-> 按预算选择 MySQL 历史
-> 调用 Ollama
-> 在 /chat 响应中解释预算结果
```

完成这个闭环后，下一章进入 `session summary`。严格复刻 Ollama 内部模板、多模型注册表和 tokenizer 官方同源验证属于生产增强，不阻塞 token 主线结课。

## 方案选择

采用“共享消息格式组件 + 向后兼容接入”的方案：

1. 新建 `internal/chatprompt`，只负责 Qwen ChatML 消息格式化和 token 计数。
2. `internal/promptbudget` 继续负责上下文预算规划。
3. `internal/recentchat` 组合两者，选择历史并调用 Ollama。
4. 旧请求仍可只传 `recent_limit` 或 `recent_token_budget`。
5. 新请求通过 `auto_token_budget=true` 显式启用自动预算。

不采用下面两种方案：

- 只扩展教学 demo：无法证明 token 预算已经进入真实聊天链路。
- 直接跳到 session summary：会留下“知道怎么算预算，但服务仍靠人工填写”的断层。

## 五个小节

### 第 08 节：历史消息格式化

新增 Qwen ChatML 消息格式器，把一条历史消息转换为：

```text
<|im_start|>{role}\n{content}<|im_end|>\n
```

这一节解决“为什么不能只计算 `message.Content`”的问题。格式器校验 `system`、`user`、`assistant`、`tool` 四种角色，非法角色直接报错。

实践入口使用独立 Go 命令，输入 role/content 后打印格式化结果。该格式是当前项目根据本机 Qwen 模板采用的可解释格式，不宣称逐 token 复刻 Ollama 内部实现。

### 第 09 节：完整对话 prompt 计数

在第 08 节基础上组合：

- system message
- 历史 user/assistant messages
- 当前 user message
- assistant generation prefix

完整对话只调用一次 tokenizer 计数，避免把正文 token 相加误当作最终输入 token。命令打印 rendered conversation、每类消息和总 token 数。

### 第 10 节：模板感知的 recent window

升级 `TokenBudgetWindowBuilder`：

- 从最新历史向前选择
- 每条消息先经过 Qwen ChatML 格式化
- token 预算包含 role 和消息边界
- 自动模式使用严格预算，单条消息超过预算时不强塞
- 最终仍按从旧到新的顺序发送给模型

旧的 content-only 构造方式保留，用于解释和兼容已有测试；`recent-chat` 运行入口切换到格式化、严格预算构造方式。

### 第 11 节：自动历史预算规划

新增自动规划器，输入：

- 请求模型名
- system prompt
- 当前 user message
- output token reserve

规划器执行：

1. 从 Ollama `/api/show` 读取模型 context length。
2. 按完整消息格式计算固定输入和 assistant prefix token。
3. 使用现有 `promptbudget.Plan` 扣除固定输入和回答预留。
4. 返回可用于 recent history 的 token 预算。

模型元数据读取失败、tokenizer 失败或固定输入已经超限时，请求必须失败，不能退回字符估算。

### 第 12 节：接入 `/chat` 与可观测验证

`ChatRequest` 新增：

```json
{
  "auto_token_budget": true,
  "output_token_reserve": 2048
}
```

约束：

- `auto_token_budget=true` 时 `output_token_reserve` 必须大于 0。
- 自动预算不能和手动 `recent_token_budget` 同时使用。
- `recent_limit` 继续控制从 MySQL 读取的候选消息上限；未传时使用当前教学默认值 `50`。

`ChatResponse` 新增预算明细：

- `budget_mode`
- `context_limit`
- `fixed_input_tokens`
- `output_token_reserve`
- `available_recent_tokens`
- `used_recent_tokens`

真实 curl 必须能看到“模型总容量如何被固定输入、回答预留和历史消息使用”。原有 count/manual 请求保持可用。

## 代码边界

### `internal/chatprompt`

只负责消息格式和计数，不访问 Ollama、MySQL 或 HTTP。

### `internal/promptbudget`

只负责把模型容量、固定输入和回答预留转换成历史预算。自动规划器依赖抽象的 context provider，不依赖具体 HTTP client。

### `internal/recentchat`

负责请求校验、读取历史、选择窗口、调用 Ollama、持久化和返回预算明细。

## 错误处理

- 非法消息角色：返回明确错误。
- tokenizer 计数失败：中止请求并保留原错误上下文。
- 模型 context metadata 缺失：中止自动预算。
- 固定输入加回答预留超过 context limit：中止请求。
- 自动和手动预算冲突：请求校验失败。
- 严格历史预算放不下最新历史：返回空历史，不突破预算。

## 生产边界

本批次完成的是可落地的单模型 token 预算框架，明确不包含：

1. Ollama 多轮内部 prompt 的逐 token 黄金样例对照。
2. 当前 tokenizer 与 Ollama `qwen:7b` 上游 revision 的严格绑定。
3. 多模型 tokenizer/template 注册表。
4. 模型元数据和固定 system prompt 的缓存优化。
5. user/assistant 成对保留和摘要回填。

这些内容记录到 `docs/teaching/00-optimization-backlog.md`。其中第 5 项会在 session summary 和上下文质量课程继续处理。

## 验收标准

每小节必须满足：

1. 新行为有先失败后通过的单元测试。
2. 有可运行的 Go 命令、测试命令或 curl。
3. 有独立教学 SOP，解释代码、运行结果和生产边界。
4. 提交前运行 review、`gofmt`、目标测试和回归检查。
5. 创建一个独立 commit，不执行 push。

整个批次必须满足：

1. 第 08 到第 12 节各有一个实现 commit。
2. `go test ./...`、`go vet ./...`、`go build ./cmd/...` 和 `git diff --check` 通过。
3. 原有 count/manual recent window 测试继续通过。
4. 自动预算 service 测试证明完整数据流。
5. 若本机 MySQL 和 Ollama 可用，执行真实 curl；否则提供经过单元测试覆盖的 SOP，并明确未执行原因。
6. 更新学习进度，明确 token 主线完成、下一章为 session summary。
7. 工作区无遗漏文件，只有本地 commits，没有 push。
