# Session Summary Five Lessons Design

日期：2026-07-11

## 目标

连续实现第 14 到第 18 节，把第 13 节的 summary 数据结构和触发策略推进为真实聊天闭环：

```text
summary watermark
-> 选择已驱逐的连续旧消息
-> 真实 tokenizer 统计
-> Ollama 增量摘要
-> MySQL version 安全更新
-> summary + recent + current user
-> 自动预算 /chat
```

完成第 18 节后，Session Summary 主线应具备真实可运行能力；下一阶段才进入 long-term memory item，不在本批次实现 Qdrant 或长期记忆检索。

## 方案选择

采用“连续前缀水位 + 小接口组合”的方案：

1. `internal/sessionsummary` 不依赖 `internal/recentchat`。
2. 两个包共享的数据通过 `SourceMessage`、生成器、store 和 updater 接口传递。
3. 只摘要 watermark 之后、recent window 之前的连续消息前缀。
4. summary 成功保存后才推进 watermark。
5. 第 18 节由 recent-chat 组合 summary 和 recent window。

不采用“对消息 ID 做任意集合差”方案。任意跳过中间消息后再推进最大 ID，会让被跳过消息永久落到 watermark 之前。

## 第 14 节：驱逐前缀选择与 token 统计

### 输入

- 按 ID 严格递增的 session 消息
- `last_message_id`
- 当前 recent window 最早消息 ID `recent_start_id`

### 选择规则

1. 过滤 `id <= last_message_id` 的已摘要消息。
2. `recent_start_id > 0` 时，它必须存在于未摘要消息中。
3. 选择 `recent_start_id` 之前的连续未摘要前缀作为 `Evicted`。
4. recent window 为空时，全部未摘要消息都属于 `Evicted`。
5. `NextWatermark` 只等于 `Evicted` 最后一条 ID；没有驱逐消息时保持原水位。

消息 ID 必须为正数且严格递增。ID 可以有空洞，不要求连续数字。

### Token 统计

使用 Qwen message formatter 和当前本地 tokenizer，分别统计：

- 所有未摘要消息 token
- 已驱逐前缀 token

输出可直接构造第 13 节 `TriggerInput`。

## 第 15 节：真实 Ollama 增量摘要生成

### 输入

- 旧 summary，可为空
- 第 14 节选出的 `Evicted` 消息
- 模型名和最大输出 token

### Prompt 结构

system 指令要求模型：

- 只输出新的滚动摘要
- 保留用户偏好、目标、约束、事实和已确认决定
- 合并旧 summary 和新增消息
- 删除临时闲聊与重复信息
- 不编造缺失事实

user prompt 显式区分：

```text
<previous_summary>
...
</previous_summary>
<new_messages>
[id=21 role=user] ...
...</new_messages>
```

消息内容需要做边界转义，避免正文中的 XML-like 标签破坏结构。生成结果去除首尾空白，空结果视为错误。

### 模型接口

`sessionsummary.Generator` 依赖小型 `TextGenerator` 接口。现有 `HTTPOllamaClient` 增加通用文本生成方法并传入 `num_predict`，避免 sessionsummary 反向依赖 recentchat。

## 第 16 节：MySQL Summary Store

### Store 行为

```text
Get(session_id, user_id)
Save(next_summary, expected_version)
```

`Get` 返回 summary 和是否存在。

`Save` 使用乐观版本：

- `expected_version = 0`：插入第一版，version 写 `1`
- `expected_version > 0`：只更新匹配旧 version 的记录，并把 version 加一
- 更新影响行数为 `0`：返回 `ErrVersionConflict`

`last_message_id` 只能前进，不能回退。新 content 不能为空。

### 测试边界

业务 store 依赖 `SummaryQueries` 小接口，单元测试使用 fake queries 验证首次插入、读取、更新、冲突和错误传播。MySQL adapter 集中 SQL，不把 SQL 分散到 service。

### 外部状态

本节不会静默执行 `sql/session_summaries.sql`。真实数据库验证前必须请求用户许可。

## 第 17 节：滚动摘要更新服务

### 依赖

- `SummaryStore`
- `MessageSource`
- 第 14 节 Selector/Analyzer
- 第 13 节 TriggerPolicy
- 第 15 节 Generator

### 数据流

```text
Get old summary
-> List messages after watermark
-> Select evicted prefix
-> Count unsummarized/evicted tokens
-> TriggerPolicy.Decide
-> false: 返回未更新原因
-> true: Generator.Update
-> Save with expected version
-> 返回新 summary/version/watermark
```

如果生成或保存失败，watermark 不推进。version conflict 返回明确错误，由调用方重试整个读取-生成流程；本节不静默覆盖并发结果。

## 第 18 节：接入 `/chat`

### API

`ChatRequest` 新增显式开关：

```json
"use_session_summary": true
```

summary 模式只允许与 `auto_token_budget=true` 一起使用。服务配置提供：

- `summary_input_reserve`：为送入主对话的完整 summary 消息预留 token
- `summary_output_limit`：增量摘要生成的 `num_predict` 上限

必须满足 `summary_output_limit < summary_input_reserve`，为 role、消息边界和格式开销留出容量。

`ChatResponse` 新增：

- `session_summary_used`
- `session_summary_updated`
- `session_summary_version`
- `session_summary_watermark`
- `session_summary_trigger_reason`

旧请求不传开关时行为不变。

### 请求顺序

1. recent-chat 先读取并选择 current recent window。
2. 自动预算先计算不含 summary 的历史容量，再扣除固定 `summary_input_reserve`。
3. 使用这个保守容量确定 recent window 和它的最早 ID。
4. 调用 updater 处理 recent window 之前的连续旧消息。
5. 再读取当前 summary，并用 tokenizer 验证完整 summary 消息不超过 reserve。
6. 将现有 system prompt 与 summary 组合为单个 system message。
7. 以真实 combined system 重新规划；实际可用历史必须不小于保守容量。
8. 保持第 3 步选出的 recent window，不因为剩余容量变多而把已摘要原文重新加回。
9. 发送 `combined summary + selected recent + current user` 给 Ollama。

固定 summary reserve 打破了“summary 长度影响 recent 起点，recent 起点又影响 summary 输入”的循环。实际 summary 超过 reserve 时请求失败，不能继续挤出一批尚未进入摘要的消息。

### 真实验证

需要：

- 用户明确执行/批准 summary schema
- 本机 MySQL、Ollama、tokenizer 可用
- curl 使用独立 session，验证 summary 创建、后续请求使用 summary 和版本推进

若外部 schema 未获批准，不能把单元测试冒充真实 curl 闭环，目标保持未完成。

## 错误处理

- 消息无序、重复或非法 ID：选择失败
- recent start 不在未摘要消息中：选择失败
- tokenizer、Ollama 或 MySQL 失败：向上传递并附加操作上下文
- summary 空输出：拒绝保存
- summary 完整输入 token 超过 reserve：拒绝进入主对话
- summary 模式未开启自动 token budget：请求校验失败
- watermark 回退：拒绝保存
- version conflict：拒绝覆盖
- `use_session_summary=true` 但依赖未装配：请求失败，不静默降级

## 生产边界

本批次明确不实现：

1. 异步 summary worker 和消息队列
2. 多实例分布式 session 锁
3. summary 质量自动评分
4. long-term memory item 抽取
5. Qdrant memory retrieval

version conflict、触发 reason 和清晰接口为后续异步化保留扩展点，但不提前实现分布式系统。

## 每节验收

每节必须满足：

1. RED/GREEN 测试证据
2. 可执行命令、测试或 curl
3. 独立教学 SOP
4. 提交前 review
5. 独立实现 commit
6. 不 push

## 整批验收

1. 第 14-18 节恰好有五个实现 commit。
2. 设计/计划文档与学习进度完整。
3. `go test ./...` 通过。
4. 相关 package race 测试通过。
5. `go vet ./...` 和 `go build ./cmd/...` 通过。
6. `git diff --check` 通过。
7. 真实 curl 证明 summary 创建、使用和可观测字段。
8. 工作区干净，只有本地 commits，没有 push。
