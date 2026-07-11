# Session Summary Trigger Design

日期：2026-07-11

## 目标

实现第 13 节：定义 session summary 保存什么，以及系统在什么条件下应该生成或更新摘要。

本节形成下面的可运行行为：

```text
未摘要消息状态
+ recent window 驱逐状态
+ 可配置双阈值
-> 是否触发 summary
+ 可解释原因
```

本节不调用 Ollama、不执行 MySQL 建表、不修改 `/chat`。下一节才把被驱逐消息交给 Ollama 生成摘要并写入 summary store。

## 方案选择

采用“生产可复用触发核心 + 可执行 SQL + 独立实践命令”的方案。

不采用：

- 直接生成并持久化摘要：单节同时混入触发、模型 prompt、MySQL upsert 和请求编排，无法逐层验证。
- 只创建 summary 表：只能回答“存哪里”，不能回答“什么时候更新”。
- 固定每 N 轮无条件摘要：没有消息离开 recent window 时也会调用模型，增加成本并重复压缩仍在窗口内的原文。

## Summary 数据结构

Go 类型 `SessionSummary` 包含：

- `SessionID`：会话标识
- `UserID`：用户标识
- `Content`：当前滚动摘要正文
- `LastMessageID`：摘要已经覆盖到的最大 message ID
- `Version`：摘要更新版本
- `CreatedAt` / `UpdatedAt`：创建和更新时间

`LastMessageID` 是关键水位。下一次生成摘要时，只处理：

```text
message.id > last_message_id
```

避免每次把整个会话从头重复摘要。

## MySQL Schema

新增 `sql/session_summaries.sql`，表主键为：

```text
(session_id, user_id)
```

每个用户的每个 session 只保存一条当前滚动摘要。字段包含 content、last_message_id、version 和时间戳。

本节只提交 SQL 文件，不执行它。原因是 schema 修改属于需要用户按 SOP 明确操作的外部状态变化。

## 触发输入

`TriggerInput` 包含：

- `UnsummarizedMessages`：`last_message_id` 之后尚未进入摘要的消息数
- `UnsummarizedTokens`：这些消息按当前 tokenizer 计算的 token 数
- `EvictedMessages`：尚未摘要且已经离开本轮 recent window 的消息数

约束：

- 三个数字不能为负数
- `EvictedMessages` 不能大于 `UnsummarizedMessages`

## 触发策略

`TriggerPolicy` 接受两个正整数配置：

- `MinMessages`
- `MinTokens`

决策顺序：

```text
evicted_messages == 0
-> 不触发

否则 unsummarized_messages >= min_messages
-> 触发，reason = message_threshold

否则 unsummarized_tokens >= min_tokens
-> 触发，reason = token_threshold

否则
-> 不触发，reason = below_threshold
```

如果两个阈值同时达到，返回 `both_thresholds`，便于监控实际触发原因。

默认教学配置：

```text
min_messages = 8
min_tokens = 2048
```

这些是可运行默认值，不声明为所有生产系统的固定最佳值。真实系统应根据平均消息长度、模型成本和摘要质量调整。

## 代码边界

### `internal/sessionsummary/types.go`

只定义 summary 状态、触发输入、触发结果和原因常量。

### `internal/sessionsummary/trigger.go`

只负责参数校验和确定性决策，不访问 tokenizer、数据库、Ollama 或 HTTP。

### `cmd/summary-trigger-demo/main.go`

通过 flags 输入观测值和阈值，打印是否触发及原因。

### `sql/session_summaries.sql`

提供用户可手动执行的 MySQL 建表语句。

## 错误处理

- 阈值小于等于 0：创建 policy 失败
- 输入统计小于 0：决策失败
- 驱逐消息大于未摘要消息：决策失败
- 合法但未达到条件：不是 error，返回 `ShouldSummarize=false`

## 测试与实践

单元测试覆盖：

1. 没有消息被驱逐时不触发
2. message 阈值触发
3. token 阈值触发
4. 两个阈值同时触发
5. 有驱逐但低于阈值时不触发
6. 非法 policy 和非法输入失败

实践命令至少运行四组场景，并在 SOP 中解释输出。

## 生产边界

本节完成“状态与时机”，但明确不包含：

- 从 MySQL 计算 unsummarized/evicted 消息集合
- 调用 tokenizer 汇总 unsummarized tokens
- 调用 Ollama 生成增量摘要
- summary MySQL Get/Upsert
- summary + recent window 合并进 `/chat`

这些按后续小节逐步实现，不把尚未落地的能力写成已完成。

## 验收标准

1. 测试先因触发 API 不存在而失败，再通过。
2. `summary-trigger-demo` 能运行 message、token、no-eviction 和 below-threshold 场景。
3. SQL 文件可由用户手动执行，但本节不执行。
4. SOP 讲清楚水位、三种统计值、决策顺序和生产边界。
5. `go test ./...`、相关 race、`go vet ./...`、`go build ./cmd/...`、`git diff --check` 通过。
6. 提交前 review，无未解决 Critical/Important 问题。
7. 创建独立实现 commit，不执行 `git push`。
