# L08：消息正文外面还有格式 token

对应工程课：[Tokenizer 小节 08](../qwen-message-format-sop.md)。本课展示当前工程如何把一条角色消息写成 Qwen 风格的 ChatML 字符串。

## 1. 本课一句话

计数和发送前必须先格式化消息：角色名、起止标记和换行都是实际字符串的一部分，也会占 token。

## 2. 先用人话理解

只数正文好比只称信纸、不称信封。模型看到的不是“你好”两个字，而是带有“这是 user 消息、内容到哪里结束”的完整包裹。

## 3. 系统里有什么

- `chatprompt.Message`：`Role` 与 `Content`。
- `chatprompt.QwenFormatter`：格式化单条消息，或顺序渲染整段对话。
- 合法 role：`system`、`user`、`assistant`、`tool`；其他 role 会被拒绝。
- 边界常量：`<|im_start|>` 与 `<|im_end|>`。

## 4. 一条完整链路

应用给出 role 和 content → `FormatMessage` 先校验 role → 写入起始标记、role、换行、content、结束标记和换行 → 后续 L09 对得到的完整字符串计数。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/message-format-demo --role user --content '你好，解释 token。'
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/message-format-demo --role tool --content '查询完成'
```

两条都是纯本地格式化，不加载 tokenizer、不访问 Ollama、不写业务数据；只可能写入可重建的 Go build cache，可安全重复。传入例如 `--role developer` 会因不受支持的 role 失败，而不是悄悄换成 user。

## 6. 结果怎么看

命令会先打印 `Role:`、`Content:` 和 `Formatted message:` 标签；后者之后的真实格式化字符串是：

```text
<|im_start|>user
你好，解释 token。<|im_end|>
```

代码还会在最后的 `<|im_end|>` 后写一个换行；代码块视觉上不强调末尾空行，但该换行属于被计数的真实字符串。上面的代码块不是命令的完整 stdout。

## 7. 算一遍或走一遍

把 `role=user`、`content=你好` 代入固定形状，得到：

```text
<|im_start|>user
你好<|im_end|>
```

这说明正文以外还包括两个标记、角色名和换行。具体 token 数不能只按字符数推断，应在 L09 用当前 tokenizer 对整个字符串实际编码。

## 8. 常见误解

“role 只是程序内部标签，不会给模型看”是错的；这里 role 被写入字符串。“所有聊天格式都接受同样 role”也不对；这里当前 formatter 只接受四个列出的 role。

## 9. 当前实现与生产边界

已保证：当前仓库 `QwenFormatter` 生成上述固定形状，并在未知 role 时失败。未保证：它自动读取每个 Ollama 模型的模板，也不证明所有 Qwen 变体和服务端模板逐字节相同；模型专属 template 的读取见 [L04](L04-ollama-model-metadata.md) 与 [L05](L05-render-prompt-template.md)。

## 10. 面试怎么说

消息 token 不能只数 content。我会把 role、起止标记和换行按模型消息协议拼成真实输入再计数；同时校验 role，避免未定义的消息形状进入预算。

## 11. 自检题与答案

问：`tool` 合法吗？答：合法。问：`developer` 在当前 formatter 中合法吗？答：不合法，会报错。问：末尾换行是否可忽略？答：不能，真实格式化字符串包含它。

## 12. 事实锚点

- 原课：[qwen-message-format-sop.md](../qwen-message-format-sop.md)
- Go 组件：`internal/chatprompt/qwen.go`
- 演示：`cmd/message-format-demo/main.go`
- 测试：`internal/chatprompt/qwen_test.go`
- 后续：[L09](L09-conversation-token-count.md)
