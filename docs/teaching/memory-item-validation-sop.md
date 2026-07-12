# Memory Item Validation SOP

主题：第 19 节，先定义什么才是一条可写入长期记忆的候选

## 1. 这一节解决什么问题

Session Summary 解决的是“同一个 session 的旧消息如何压缩”。Long-term Memory Item 解决的是另一个问题：

```text
用户在某次会话里说过一个稳定事实
-> 把事实提取成可命名的 item
-> 以后跨 session 更新、遗忘和检索
```

模型不能直接写数据库。模型可能输出错误类型、不同格式的 key、虚构来源，甚至把 assistant 的回答误认为用户事实。因此第一步不是调用模型，而是先建立 Go 侧不可绕过的校验边界。

## 2. Part 1：三个核心结构

代码：

- [types.go](/offline-rag-go-lab/internal/memoryitem/types.go:1)

`SourceMessage` 是原始证据：

```go
type SourceMessage struct {
    ID        int64
    SessionID string
    UserID    string
    Role      string
    Content   string
}
```

`Candidate` 是模型提出、尚未被信任的候选：

```go
type Candidate struct {
    Operation        Operation
    Kind             Kind
    Key              string
    Value            string
    Confidence       float64
    SourceMessageIDs []int64
}
```

`Item` 才是通过校验并经过生命周期决策后，可以进入 MySQL 的当前事实。

重点不是结构体名字，而是状态转换：

```text
SourceMessage -> untrusted Candidate -> validated Candidate -> Item
```

## 3. Part 2：允许的行为和分类

当前 operation 只有两个：

- `upsert`：新增事实或修正旧事实
- `forget`：用户明确要求遗忘或否定旧事实

当前 kind 只有五个：

- `identity`：姓名、角色等身份信息
- `preference`：教学方式、工具或表达偏好
- `project_fact`：项目语言、架构等稳定事实
- `goal`：长期目标
- `constraint`：明确限制，例如“不允许 push”

不支持的值直接报错。这里故意不用“未知类型也先存下来”的宽松策略，因为错误分类一旦持久化，后续去重和检索都会变得不可靠。

## 4. Part 3：校验函数具体做什么

代码：

- [validate.go](/offline-rag-go-lab/internal/memoryitem/validate.go:1)

入口：

```go
func ValidateAndNormalizeCandidate(
    userID string,
    sessionID string,
    candidate Candidate,
    messages []SourceMessage,
) (Candidate, error)
```

执行顺序：

1. 校验当前 `user_id` 和 `session_id` 非空。
2. operation 和 kind 转成统一的小写值，并检查白名单。
3. key 转成 snake_case，例如 `Implementation Language` 变为 `implementation_language`。
4. upsert 的 value 不能为空，最长 4096 个字符。
5. confidence 必须是 0 到 1 的有限数，NaN 和 Inf 也会拒绝。
6. 每个来源 ID 必须存在于本次输入。
7. 来源消息必须属于当前 user 和当前 session。
8. 来源 role 必须是 `user`，正文不能为空。
9. 重复来源 ID 去重，但保留第一次出现的顺序。

为什么还要检查 `session_id`：只检查 user 不够。同一个用户可能同时有多个 session，如果上游错误地混入另一会话的消息，证据归属仍然会出错。

## 5. Part 4：运行 demo

在仓库根目录执行：

```bash
go run ./cmd/memory-item-validate-demo
```

代码：

- [main.go](/offline-rag-go-lab/cmd/memory-item-validate-demo/main.go:1)

预期输出：

```text
Valid candidate: project_fact/implementation_language=Go
Sources: 101
Rejected assistant-only source: source message 102 must have role user
```

第一段证明格式不稳定的模型候选可以先规范化成稳定 key。第二段证明 assistant 即使说“你喜欢 Rust”，也不能成为用户长期偏好的唯一证据。

运行单元测试：

```bash
go test ./internal/memoryitem
```

测试还覆盖未知类型、非法 key、空值、超长值、非法 confidence、跨用户、跨 session、未知来源和重复消息 ID。

## 6. 当前实现和生产级做法

当前实现已经具备生产正确性所需的最小边界：

- 白名单类型
- 稳定 key
- 用户与会话隔离
- 原始用户消息证据
- 明确 forget，不因“本轮没提到”而删除

当前没有解决跨 key 的语义去重。例如：

```text
implementation_language
coding_language
```

它们可能表达同一件事，但当前会被视为两个 key。生产里通常通过受控 ontology、key alias 或离线语义合并解决；这不影响先建立确定性主线。

## 7. 总结与重点

1. Session Summary 是会话压缩，Memory Item 是跨会话的结构化事实。
2. 模型输出只能叫 candidate，不能直接叫 memory。
3. 一条可接受候选必须回指当前用户、当前 session 的原始 user 消息。
4. assistant 回答不能独立证明用户事实。
5. Go 校验器决定“能不能进入下一步”，后续 Ollama、MySQL 和 Qdrant 都不能绕过它。
