# L09：整段对话一次编码才是 prompt 大小

对应工程课：[Tokenizer 小节 09](../conversation-token-count-sop.md)。本课计算完整对话加上生成起点的 token，而不是分别数几段正文后猜一个总数。

## 1. 本课一句话

先按原顺序渲染完整 conversation，再追加未完成的 assistant generation prefix，最后只对这一整个字符串调用一次 `Encode`。

## 2. 先用人话理解

把多轮对话拆开称重会漏掉每段之间的包装和相邻字符边界。正确做法是先把整份要寄给模型的包裹封好，再称一次总重量。

## 3. 系统里有什么

- `chatprompt.QwenFormatter.Render`：保留传入消息顺序，逐条格式化。
- `chatprompt.TokenCounter.Count`：调用 `Render` 后只调用一次底层 `CountText`。
- 当前 demo 顺序：可选 system → 可选历史 user → 可选历史 assistant → 当前 user。
- generation prefix：`<|im_start|>assistant\n`。

## 4. 一条完整链路

消息数组按时间顺序进入 formatter → 每条附带 ChatML 边界 → 最后一条当前 user 之后追加 assistant prefix → tokenizer 对完整 rendered conversation 一次 `Encode` → 返回 rendered 文本和总 token。prefix 没有 assistant 内容，也没有 `<|im_end|>`：那里正是模型即将生成内容的位置。

## 5. 实际怎么做

```bash
sh scripts/regression/lessons-09-10.sh
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/conversation-token-demo --tokenizer assets/tokenizers/qwen2/tokenizer.json
```

- 回归脚本和 demo 都是纯本地：依赖 Go、指定 tokenizer 资产和 Go 依赖，不访问 Ollama/MySQL，也不写业务数据；会写可重建的 build cache，可安全重复。
- 脚本会先校验 tokenizer SHA，再跑 focused tests 与固定输入的 golden 值；缺少资产会明确失败。

## 6. 结果怎么看

输出的 `Rendered conversation` 最后一行应是 `<|im_start|>assistant`，其后有实际换行；`Total prompt tokens` 是整段已渲染 prompt 的一次计数。它包含历史、格式标记和 generation prefix，不是“所有 content token 的简单相加”。

## 7. 算一遍或走一遍

在资产 SHA256 为 `b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8`、当前 formatter 与脚本给定文本下，完整 system + 两条历史 + 当前问题是 `100` token；把两条历史设为空后是 `56`；差值 `100 - 56 = 44`。这三个值只绑定该 SHA、这组输入和当前格式化器，任何一项变化都应重跑，不能把它们当成模型通用常数。

## 8. 常见误解

“assistant prefix 可以等模型回答后再算”不对：它在请求 prompt 中，必须先占容量。“每条消息分别 Encode 后相加等价”也不对：消息边界和 BPE 的整体输入都可能改变结果。

## 9. 当前实现与生产边界

已保证：本组件渲染完整数组并恰好一次调用 `CountText`，且可选择是否带 assistant prefix。未保证：它替代模型服务端的最终 tokenizer；本项目用本地、经 SHA 固定的资产做可复现预算，资产或模型协议变更需重新验证。

## 10. 面试怎么说

我把 conversation 当成一个最终 prompt：先按协议渲染所有 role/message，追加 assistant 的生成起点，再一次编码。这样 template 和生成边界不会从 token 预算里漏掉。

## 11. 自检题与答案

问：assistant prefix 后为什么没有 `<|im_end|>`？答：模型将在这里继续生成 assistant 内容。问：100、56、44 对任何输入都成立吗？答：不成立，只是绑定固定 SHA、文本和 formatter 的回归值。问：脚本会写聊天表吗？答：不会。

## 12. 事实锚点

- 原课：[conversation-token-count-sop.md](../conversation-token-count-sop.md)
- Go 组件：`internal/chatprompt/{qwen,count}.go`
- 演示：`cmd/conversation-token-demo/main.go`
- 回归：`scripts/regression/lessons-09-10.sh`
- 前置：[L08](L08-qwen-message-format.md)
