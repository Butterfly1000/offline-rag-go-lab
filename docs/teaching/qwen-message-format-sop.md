# Qwen Message Format SOP

主题：第 08 节，为什么历史消息的 token 不只来自正文

## 1. 这一节解决什么问题

旧的 recent window 只计算：

```go
messages[i].Content
```

但模型收到一条聊天消息时，还需要知道：

- 这是谁说的
- 消息从哪里开始
- 消息在哪里结束

当前本机 `qwen:7b` 使用的模板包含 Qwen ChatML 标记。因此本项目把一条消息表示为：

```text
<|im_start|>{role}
{content}<|im_end|>
```

`role` 和两个边界也会占 token，不能只计算 `{content}`。

## 2. 代码怎么实现

核心代码：

- [qwen.go](/offline-rag-go-lab/internal/chatprompt/qwen.go:1)
- [qwen_test.go](/offline-rag-go-lab/internal/chatprompt/qwen_test.go:1)
- [main.go](/offline-rag-go-lab/cmd/message-format-demo/main.go:1)

### Part 1：消息数据

```go
type Message struct {
    Role    string
    Content string
}
```

`Role` 说明说话方，`Content` 保存正文。当前支持：

- `system`
- `user`
- `assistant`
- `tool`

这里使用 `string` 而不是把格式器绑定到 `recentchat.MessageRole`，是为了让 `chatprompt` 保持独立：它不需要知道 MySQL 或聊天服务。

### Part 2：`strings.Builder`

```go
var formatted strings.Builder
formatted.WriteString("<|im_start|>")
formatted.WriteString(message.Role)
formatted.WriteByte('\n')
formatted.WriteString(message.Content)
formatted.WriteString("<|im_end|>")
formatted.WriteByte('\n')
```

`strings.Builder` 是 Go 标准库提供的字符串构建器。连续调用 `WriteString` 或 `WriteByte`，最后调用 `String()`，可以避免用很多 `+` 反复创建临时字符串。

这里的 `WriteByte('\n')` 写入真实换行符，不是写入两个字符 `\` 和 `n`。

### Part 3：为什么校验 role

```go
if !isSupportedRole(message.Role) {
    return "", fmt.Errorf("unsupported message role %q", message.Role)
}
```

如果直接接受任意 role，格式化结果虽然是字符串，却不一定是模型认识的聊天角色。生产代码应尽早返回明确错误，而不是把无效消息发给模型后再猜原因。

`fmt.Errorf` 用格式化字符串创建一个带上下文的 `error`；`%q` 会给值加引号，错误信息更容易定位。

## 3. 实践 SOP

### SOP 0：先运行跨环境回归

新机器或更新 Go 版本、依赖、Tokenizer 资产后，先执行：

```bash
sh scripts/regression/lesson-08.sh
```

它会同时验证第 8 节依赖的本地 `sugarme/tokenizer` 兼容版本、中文黄金 token
结果和消息格式。错误现象与排查顺序见
[跨环境回归与排坑](/offline-rag-go-lab/docs/teaching/00-cross-environment-regression.md:1)。

### SOP 1：运行合法 user 消息

在仓库根目录执行：

```bash
go run ./cmd/message-format-demo \
  --role user \
  --content '你好，解释 token。'
```

预期结构：

```text
Role: user
Content: 你好，解释 token。
Formatted message:
<|im_start|>user
你好，解释 token。<|im_end|>
```

重点不是记住字符串，而是确认一条消息进入 token 计算前，已经多了：

```text
role + start boundary + end boundary + newline
```

### SOP 2：验证非法角色

```bash
go run ./cmd/message-format-demo --role unknown --content '测试'
```

预期命令失败，并包含：

```text
unsupported message role "unknown"
```

### SOP 3：运行单元测试

```bash
go test ./internal/chatprompt
```

测试覆盖合法格式、四种支持角色、非法角色和 assistant 生成前缀。

## 4. 当前实现和生产实现的关系

当前实现已经具备生产代码需要的三个基础性质：

1. 格式规则集中在一个组件，不散落在业务代码中。
2. 非法角色会失败，不静默生成错误 prompt。
3. 单元测试固定了边界字符串，后续修改会触发回归失败。

当前边界也必须明确：

- 这是项目根据本机 Qwen 模板采用的 Qwen ChatML 格式。
- Ollama 如何把多轮 `/api/chat messages` 转为内部 `.Prompt`，尚未做逐 token 黄金对照。
- 因此这一节证明“角色和边界必须计数”，不宣称已经逐字节复刻 Ollama 内部运行时。

## 5. 本节重点

```text
message token
!= content token

message token
= role + content + message boundaries
```

下一节会把多条消息按顺序组合，并用 tokenizer 对完整对话只计数一次。
