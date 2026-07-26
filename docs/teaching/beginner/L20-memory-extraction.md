# L20：模型提取候选，Go 决定能否接受

对应工程课：[Memory 小节 20](../memory-item-extraction-sop.md)。本课让 qwen 生成 JSON candidate；确定性代码只守住结构、来源归属和有限 destructive gate，候选事实正确性仍不保证。

## 1. 本课一句话

Qwen 生成的 candidate 永远不可信；JSON schema 只帮助约束形状，严格 JSON decode 与 L19 Go 校验才决定哪些候选在结构、来源归属和有限 forget 门槛上被接受。

## 2. 先用人话理解

模型像一名把谈话整理成表格的速记员，表格格式正确不等于内容一定真实。系统可以要求它按固定列填写，但规则只检查证据归属和是否命中当前有限的 forget gate，不证明候选事实或真实遗忘意图。

## 3. 系统里有什么

- `BuildExtractionPrompt`：把 session summary 和消息放进不可信数据区。
- `CandidateJSONSchema`：要求顶层 `candidates` 和候选字段/枚举/基础范围。
- `GenerateJSON`：Ollama `/api/chat` 的 `format` schema、`temperature=0` 和输出上限。
- `Extractor`：strict decode，随后逐条调用 L19 validator。

## 4. 一条完整链路

当前 user/session 的消息与可选 summary → HTML 转义后写入 `<session_summary>`/`<messages>` → system 指令声明其中内容不可执行 → qwen 按 schema 生成 JSON → `DisallowUnknownFields` 解码且确认没有第二个 JSON 值 → 必填字段检查 → 每条进入 L19 的来源归属、role/非空和有限 forget gate 校验 → 输出 validated candidates 或错误。该过程不验证 candidate 的 kind/key/value 是否被来源语义支持。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/memoryitem -run 'Test(Extractor|BuildExtractionPrompt|CandidateJSONSchema)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-extract-demo --config /absolute/path/to/your/recent-chat.env --model qwen:7b --max-output-tokens 512
```

- test 是纯本地 fake generator 检查，不访问 Ollama/MySQL，不写业务数据；只写可重建 build cache，可安全重复。
- demo 依赖运行中的 Ollama、qwen 模型和由 `config/recent-chat.env.example` 派生的私有配置；它调用 `/api/chat` 进行生成，但不连接 MySQL/Qdrant、也不写 memory。可重复调用，但模型输出和候选数量/措辞不能承诺逐字相同。

## 6. 结果怎么看

`{"candidates":[]}` 是合法结果，表示这轮没有稳定事实；空响应、非法 JSON、额外字段、缺少必填字段、尾随第二个 JSON 值或任一候选未通过 L19 都是错误。输出中的 `Raw JSON` 只是诊断材料，只有 `Validated candidates` 才能进入 resolver。

## 7. 算一遍或走一遍

消息 ID 101 是 user 的“我叫小黄，这个项目使用 Go。”，ID 102 是 assistant 的 Rust 猜测。模型可能产生 name/项目语言候选；引用 102 的候选会因 role 失败，而引用 101 的候选可通过来源校验，但这不证明例如把 value 错写成 Rust 具有语义依据。若模型输出 `confidence=100`，schema/prompt 不足以保证安全，Go 拒绝范围外值；若输出 `{"candidates":[],"reason":"none"}`，strict decode 因额外字段拒绝。

## 8. 常见误解

“schema 保证事实正确”错误，它主要约束 JSON 形状。“通过 Go 校验就证明 kind/key/value 正确”也错误：当前 Go 校验结构、来源归属、role/非空与有限 forget gate，不做来源语义蕴含验证。“temperature=0 就完全确定”也错误，qwen/运行环境仍可能产生不同候选或遗漏事实。“标签包起来就没有 prompt injection”错误：还需要转义、system 中声明数据不可信、以及 L19 的边界检查。

## 9. 当前实现与生产边界

常见故障包括模型返回非 JSON/多余解释、弱模型不遵守 confidence 或误选 forget、以及复杂 schema 在某些 Ollama runner 上触发 5xx/崩溃。当前做法是兼容的 shape schema + strict decode + Go validator；Go validator 覆盖结构、来源归属、role/非空和有限 forget gate，却不判断 kind/key/value 是否由来源语义支持。遇到 runner 5xx 应先检查服务日志和最小 schema 对照，不盲目把 HTTP 重试当修复。未保证候选召回率、key ontology 或模型事实正确性。

## 10. 面试怎么说

结构化输出是引导模型，不是信任边界。我让 qwen 输出 schema JSON，再用 strict decoder 拒绝额外/尾随数据，并用确定性 validator 验证结构、user/session/source、role/非空与有限 forget gate；它不证明 kind/key/value 的事实语义，模型只提建议，不能直接写库。

## 11. 自检题与答案

问：空 `candidates` 数组是错误吗？答：不是。问：引用合法 user source 的错误 value 能否仍通过当前 Go 校验？答：可能，当前不验证语义蕴含。问：模型响应后又带一段解释能接受吗？答：不能，strict decode 拒绝尾随 JSON/未知字段。问：真实 qwen 输出可作为 golden 文本吗？答：不能，生成非确定。

## 12. 事实锚点

- 原课：[memory-item-extraction-sop.md](../memory-item-extraction-sop.md)
- Go 组件：`internal/memoryitem/{prompt,extractor}.go`
- Ollama 边界：`internal/recentchat/ollama.go`
- 演示：`cmd/memory-extract-demo/main.go`
- 前置：[L19](L19-memory-candidate-validation.md)
