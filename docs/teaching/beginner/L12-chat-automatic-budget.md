# L12：让 `/chat` 自动守住 token 预算

对应工程课：[Tokenizer 小节 12](../recent-chat-automatic-token-budget-sop.md)。本课把 L07–L11 的预算计算接到真实聊天请求，并把结果放进 API 响应。

## 1. 本课一句话

`/chat` 有 count、manual、automatic 三种历史预算模式；automatic 同时限制历史和真实生成长度，且把预算明细返回给调用方。

## 2. 先用人话理解

手动模式像你自己决定“旧聊天最多装多少”；自动模式像服务先看车型载重、称当前货物，再决定还能带多少旧货，并要求司机不要超过预留的回答长度。

## 3. 系统里有什么

- `recent_limit`：count 模式按消息条数取最近记录。
- `recent_token_budget`：manual 模式的历史 token 上限。
- `auto_token_budget` 与 `output_token_reserve`：automatic 模式的开关和回答预留。
- 响应的 `budget_mode`、`context_limit`、`fixed_input_tokens`、`available_recent_tokens`、`used_recent_tokens`、`used_messages`：可观察的预算结果。

## 4. 一条完整链路

未开启 automatic 且 `recent_token_budget` 不大于 0 时，服务走 `count`；未开启 automatic 且 `recent_token_budget > 0` 时走 `manual`；`auto_token_budget=true` 时走 `automatic`。automatic 读取模型 context、计算固定 prompt、严格选择历史，并向 Ollama `/api/chat` 传入 `options.num_predict=output_token_reserve`。

## 5. 实际怎么做

```bash
sh scripts/regression/lessons-11-12.sh
sh scripts/regression/lessons-11-12.sh --live
```

- 第一条是默认回归：依赖 Go、curl、tokenizer 资产、运行中的 Ollama 与目标模型；读取 `/api/tags`、`/api/show`，不请求 recent-chat，也不写 MySQL 业务数据，只写可重建 cache 和临时文件，可安全重复。
- 第二条是 live 回归：还依赖已启动的 recent-chat、其 MySQL 表和由 `config/recent-chat.env.example` 派生的私有本地配置；会调用 Ollama 生成并向 MySQL 写入一轮 user/assistant 消息。脚本每次使用新的 `regression-lesson-12-*` session，因此可重复运行但会持续新增业务记录；需要恢复时应先按该专用 session 精确检查、再由操作者清理对应记录。

## 6. 结果怎么看

automatic 响应必须满足 `fixed_input_tokens + output_token_reserve + available_recent_tokens = context_limit`，并且 `used_recent_tokens <= available_recent_tokens`。`used_messages` 是最终发给 Ollama 的历史条数。预算字段即使为 0 也会出现在 JSON，0 不能被理解为“没有这个字段”。

## 7. 算一遍或走一遍

假设返回 `context_limit=1000`、`fixed_input_tokens=120`、`output_token_reserve=200`，则 `available_recent_tokens=680`。若最终选中历史为 412 token，响应应有 `used_recent_tokens=412`，并满足 `412 <= 680`。automatic 会把同一个 200 传为 `num_predict`：只在公式里减 200 但不限制生成是不完整的，因为模型仍可能生成更多。

## 8. 常见误解

`auto_token_budget=true` 和正的 `recent_token_budget` 不能同时出现，服务会拒绝，避免两套历史额度互相覆盖。automatic 的 `recent_limit` 只是从 MySQL 读取候选消息的数量上限，不是最终 token 容量。也不能把 output reserve 仅当显示字段；它实际约束 `num_predict`。

## 9. 当前实现与生产边界

已保证：automatic 需要正的 `output_token_reserve`，使用 strict formatted window，并将 reserve 传给 Ollama 真实生成限制；历史选不进来时宁可为空。未保证：Ollama 必然恰好生成 reserve 个 token，`num_predict` 是最大值而非承诺长度；模型/网络失败时请求失败，服务不会凭字符数补一个预算。

## 10. 面试怎么说

我会把 API 的预算模式显式化：旧兼容的 count、手填上限的 manual、按模型上限自动计算的 automatic。自动模式不只算历史，还把同一个输出预留传给 `num_predict`，并在响应中暴露完整等式，便于线上检查。

## 11. 自检题与答案

问：`auto_token_budget=true` 加 `recent_token_budget=10` 会怎样？答：请求校验失败。问：automatic 的回答最大生成长度由什么限制？答：Ollama `options.num_predict`，值为 `output_token_reserve`。问：`--live` 默认无业务写入吗？答：不是，它会写一轮聊天记录。

## 12. 事实锚点

- 原课：[recent-chat-automatic-token-budget-sop.md](../recent-chat-automatic-token-budget-sop.md)
- Go 组件：`internal/recentchat/{types,service,ollama}.go`
- 回归：`scripts/regression/lessons-11-12.sh`
- 前置：[L07](L07-context-budget.md)、[L10](L10-template-aware-recent-window.md)、[L11](L11-automatic-history-budget.md)
