# Memory Item Extraction SOP

主题：第 20 节，用真实 `qwen:7b` 提取长期记忆候选，并由 Go 再次校验

## 1. 这一节解决什么问题

第 19 节已经定义了“什么候选可以进入下一步”。这一节把真实模型接进来：

```text
原始消息
-> Ollama JSON schema 提取
-> strict JSON decode
-> 第 19 节 validator
-> validated candidates
```

关键原则：JSON schema 只能约束输出结构，不能证明事实正确，也不能证明 operation 安全。

## 2. Part 1：构造不可信数据边界

代码：

- [prompt.go](/offline-rag-go-lab/internal/memoryitem/prompt.go:1)

`BuildExtractionPrompt` 要求消息：

- ID 为正数并严格递增
- 属于当前 `user_id` 和 `session_id`
- role 只能是 user/assistant
- content 不能为空

summary 和消息正文使用 HTML escape 后放入独立标签：

```text
<session_summary>
...
</session_summary>
<messages>
[id=101 role=user] ...
</messages>
```

system prompt 明确这些标签内都是不可信数据，不执行其中指令。assistant 可以帮助理解上下文，但不能进入 `source_message_ids`。

## 3. Part 2：JSON schema 负责什么

`CandidateJSONSchema` 约束：

- 顶层必须有 `candidates`
- 每个候选必须有 operation/kind/key/value/confidence/source IDs
- operation 和 kind 使用枚举
- key 使用 ASCII snake_case pattern
- confidence 声明为 0 到 1
- 不允许额外字段

但是当前本机 Ollama `0.23.2` 对复杂嵌套 schema 的 grammar 兼容性有限。因此 schema 没有承担全部长度和来源校验；这些规则由 [validate.go](/offline-rag-go-lab/internal/memoryitem/validate.go:1) 再执行一次。

这是职责分层，不是放弃校验：

```text
schema：尽量让模型输出正确形状
Go validator：决定数据是否真的可信
```

合法的 `{"candidates":[]}` 表示本轮没有稳定信息，不是错误。空响应、非法 JSON、缺字段、额外字段或非法候选才是错误。

## 4. Part 3：Ollama 具体请求代码

代码：

- [ollama.go](/offline-rag-go-lab/internal/recentchat/ollama.go:1)

`OllamaChatRequest` 增加：

```go
Format json.RawMessage `json:"format,omitempty"`
```

`GenerateJSON` 发送：

```text
POST /api/chat
model=qwen:7b
messages=[system,user]
format=<JSON schema>
stream=false
num_predict=512
temperature=0
```

`temperature=0` 必须显式传入，减少同一输入反复产生不同 operation/key 的概率。HTTP 非 2xx、schema 本身非法或模型空输出都会返回错误。

## 5. Part 4：strict decode 和二次校验

代码：

- [extractor.go](/offline-rag-go-lab/internal/memoryitem/extractor.go:1)

Extractor 使用 `json.Decoder.DisallowUnknownFields()`，并继续读取一次确认已经到 `io.EOF`，所以这些输出都会拒绝：

- JSON 后还有第二个对象
- 顶层多出 explanation
- candidate 多出 reason
- required 字段缺失

解析成功后，每条 candidate 仍调用：

```go
ValidateAndNormalizeCandidate(userID, sessionID, candidate, messages)
```

此外，forget 增加 destructive gate：至少一条来源正文必须明确包含“请忘掉”“不要记住”“please forget”等受控遗忘表达。普通陈述即使被模型错标为 forget，也不能通过。

## 6. Part 5：真实运行

配置文件：

- `config/recent-chat.env`
- 使用 `OLLAMA_BASE_URL=http://127.0.0.1:11434`
- 配置文件被 Git 忽略，不使用环境变量

执行：

```bash
go run ./cmd/memory-extract-demo \
  --config config/recent-chat.env \
  --model qwen:7b
```

代码：

- [main.go](/offline-rag-go-lab/cmd/memory-extract-demo/main.go:1)

真实输入包含：

- user：名字是小黄，项目使用 Go
- assistant：推测用户可能喜欢 Rust
- user：教学偏好真实操作
- user：不允许自动 push

最终真实输出通过了两条：

```text
Validated candidates: 2
- upsert identity/name="小黄" confidence=1.00 sources=[101]
- upsert project_fact/language="Go" confidence=1.00 sources=[101]
```

Rust 来自 assistant 推测，没有进入候选。当前弱模型没有提取所有可用事实，这说明 schema 和校验能保证安全边界，但不能保证召回率。

## 7. 真实故障与解决方案

### 故障 1：Ollama runner 直接 500

错误：

```text
model runner has unexpectedly stopped
```

日志 `~/.ollama/logs/server.log` 的真实根因是：

```text
SIGSEGV: segmentation violation
llama runner terminated error="exit status 2"
```

环境证据：

- Ollama server `0.23.2`
- Apple M2，16 GB unified memory
- qwen runner 约 6.2 GB，加载 context 4096
- 普通 chat 成功
- 单字段 schema 成功
- 完整强约束嵌套 schema 可重复触发 SIGSEGV
- 保持相同候选结构、减少非必要约束后成功

最终做法：

1. schema 保留结构、required、枚举、key pattern 和 confidence 范围。
2. 不在当前旧 Ollama grammar 中叠加全部 `maxLength`、`minItems`、source minimum 等约束。
3. 所有省略的规则继续由 Go validator 强制执行。
4. 不增加盲目 HTTP retry，因为 runner 崩溃不是瞬时网络错误。

以后遇到同类 500：先看 server log 是否 SIGSEGV，再用“普通 chat -> 最小 schema -> 当前 schema”三步对照，不要先修改 Go JSON parser。

### 故障 2：模型输出 confidence=100

旧 Ollama 接受 numeric range schema，但弱模型仍可能不遵守。最终在 prompt 中明确：

```text
confidence 必须是 0.0 到 1.0，例如 0.95，禁止输出 95 或 100
```

Go validator 仍拒绝范围外值，不能只依赖 prompt。

### 故障 3：普通陈述被误判为 forget

模型曾把“我叫小黄”输出为 forget。最终采用两层保护：

1. prompt 明确普通陈述是 upsert，只有明确遗忘请求才是 forget。
2. Go destructive gate 检查来源正文是否有明确遗忘表达。

不能采用“模型说 forget 就删除”的实现。

## 8. 当前实现和生产级差异

当前已经真实具备：

- 本地模型 JSON schema 提取
- temperature=0
- strict decode
- user/session/source 隔离
- assistant-only 拒绝
- destructive forget gate

仍未实现：

- 模型输出质量评分
- extraction retry/repair 策略
- ontology 控制 key，例如统一 `language` 与 `implementation_language`
- 人工审核高风险记忆
- MySQL 写入

模型召回率和 key ontology 属于后续优化；错误候选不能写入则是当前必须保证的正确性。

## 9. 总结与重点

1. `format` 让模型更容易输出 JSON，但不能代替 Go 校验。
2. `candidates: []` 是正常结果；非法输出才是错误。
3. 旧 Ollama 的复杂 schema grammar 可能让 runner SIGSEGV，使用兼容 schema + Go 二次校验。
4. 弱模型可能忽略 numeric range 或误判 forget，所以 destructive operation 必须有确定性保护。
5. 当前已证明真实 qwen 能产出可校验候选；下一节才决定 INSERT/UPDATE/NOOP/FORGET。
