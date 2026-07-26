# L19：长期记忆候选先过确定性校验

对应工程课：[Memory 小节 19](../memory-item-validation-sop.md)。本课在模型和数据库之间建立一道不可绕过的证据与归属边界。

## 1. 本课一句话

模型提出的 candidate 只有同时通过 operation/kind、key/value、来源归属和 forget 的有限短语 gate，才有资格进入长期 memory 的下一步。

## 2. 先用人话理解

session summary 是同一会话的压缩笔记；memory item 是以后跨会话也可能继续使用的结构化事实。因此模型的一句猜测不能直接变成档案，必须能回指到当前用户、当前会话中用户本人说过的话。

## 3. 系统里有什么

- operation 仅两种：`upsert`（新增/修正）与 `forget`（申请遗忘，另需有限短语 gate）。
- kind 仅五种：`identity`、`preference`、`project_fact`、`goal`、`constraint`。
- `Candidate`：不可信模型建议；`SourceMessage`：原始证据；`Item`：后续可持久化的当前事实。
- `ValidateAndNormalizeCandidate`：Go 侧的唯一准入边界。

## 4. 一条完整链路

模型 candidate → 规范化 operation/kind/key/value → 校验 confidence 在 0–1 且有限 → 找到所有 source IDs → 每条 source 必须属于当前 user、当前 session，且 `role=user`、正文非空 → forget 还须命中 `validate.go` 中有限的中英文前缀或 substring 短语启发式 → 产出 validated candidate。assistant 可提供上下文，但不能单独成为证据。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-item-validate-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/memoryitem -run TestValidate -count=1
```

- 两条命令都是纯本地：依赖 Go 与项目依赖，不访问 Ollama/MySQL/Qdrant 或配置，不写业务数据；只可能写 build cache，可安全重复。
- demo 会显示 `Implementation Language` 被规范为 `implementation_language`，并拒绝把 assistant ID 102 当成唯一证据。

## 6. 结果怎么看

示例的 user 消息 ID 101 “这个项目使用 Go。”可形成 `project_fact/implementation_language=Go`。assistant ID 102 的“你可能也喜欢 Rust。”即使看起来合理也被拒绝，因为来源 role 不是 user。来源 ID 重复会去重但保留第一次顺序，未知或跨 user/session 的 ID 则失败。

## 7. 算一遍或走一遍

对 `(user=u-1, session=s-9)`，candidate `upsert/project_fact/Implementation Language= Go `、confidence `0.95`、sources `[101,101]` 被规范为 key `implementation_language`、value `Go`、sources `[101]`。同一 candidate 若 101 属于 session `s-8`，即使 user 相同也失败。forget `project_fact/implementation_language` 若来源只是“项目使用 Rust”，通常不会命中 gate；“请忘掉项目语言”会命中当前短语 gate，但这只是有限启发式，不是对用户意图的语义证明。

## 8. 常见误解

“模型输出 JSON 就已可信”错误，L20 的结构化 JSON 仍需本课校验。“assistant 的确认可以证明用户偏好”错误。也不能因为本轮没再提到某事实就 forget；遗忘有破坏性 gate，但 gate 只检查有限短语，引用、教学示例或含有相同短语的文本也可能误报，换一种表达也可能漏报。summary 的压缩文本不能替代 source user message。

## 9. 当前实现与生产边界

已保证：未知 operation/kind、非法 key、空 upsert value、NaN/Inf confidence、跨 user/session、assistant-only 和未命中 forget 短语 gate 均拒绝。未保证：来源合法并不证明 candidate 的 kind/key/value 真由该来源语义蕴含；也不保证 forget gate 完整识别真实遗忘意图，或跨 key 的语义去重，例如 `coding_language` 和 `implementation_language` 是否同义。这些需要更强的语义/人工治理，不应由宽松校验猜测。

## 10. 面试怎么说

长期记忆先把模型建议当作 untrusted candidate。Go 只接受五类事实和两种操作，要求每个来源是当前 user/session 的非空 user 消息，并对 forget 加有限短语门槛；它保证 provenance 和基础结构，不声称已经证明 kind/key/value 的语义真实性，这比把聊天摘要直接写入 memory 更安全。

## 11. 自检题与答案

问：当前 kind 有几类？答：五类。问：assistant 消息能否单独当 source？答：不能。问：forget 只要模型选了该 operation 就能执行吗？答：不能，至少要命中有限短语 gate；它仍可能对引用/教学文本误报或遗漏别的表达。问：合法来源能证明 candidate 的 value 一定为真吗？答：不能。

## 12. 事实锚点

- 原课：[memory-item-validation-sop.md](../memory-item-validation-sop.md)
- Go 组件：`internal/memoryitem/{types,validate}.go`
- 演示：`cmd/memory-item-validate-demo/main.go`
- 后续：[L20](L20-memory-extraction.md)、[L21](L21-memory-resolution.md)
