# Optimization Backlog

主题：不阻塞当前教学主线、但值得后续继续研究和落地的内容

这份文档用于避免两个极端：

1. 为了快速推进而忘记真实生产差距
2. 为了追求一次性完美而长期停在某个细节

记录到这里不代表降低质量，而是明确：

- 当前已经做到什么
- 哪些问题暂时不影响理解和主线运行
- 什么时候值得回来继续实现

---

## 使用规则

后续教学遇到优化项时：

1. 如果影响当前行为正确性、安全性或数据完整性，必须在当前小节解决
2. 如果不影响当前框架成立，但属于生产增强，记录到本文件
3. 每项必须写明“为什么需要”和“何时再做”
4. 不因为 backlog 很长而阻塞下一章

---

## Tokenizer 优化项

### 1. 官方来源与版本绑定

为什么需要：当前 tokenizer 文件来自 `~/Dolphin/hf_model/tokenizer.json`，尚未证明和 Ollama 的 `qwen:7b` 同源。

何时再做：项目准备使用 token 预算做严格生产限制，或更换正式模型资产时。

目标结果：记录模型上游仓库、revision、下载地址和 tokenizer SHA256。

### 2. token IDs 黄金样例对照

为什么需要：文件能被 Go 库加载，不等于编码结果和模型官方 tokenizer 完全一致。

何时再做：取得明确的官方 tokenizer 资产后。

目标结果：选取中文、英文、代码和特殊字符样例，对比官方实现与 Go 实现的 token IDs。

### 3. 完整 chat template 计数

当前进度：项目已经按 Qwen ChatML 计算 role、消息边界、完整固定 prompt 和 recent history，并接入自动预算。

为什么仍需要：尚未用 Ollama 实际 `prompt_eval_count` 对多轮黄金样例做严格误差对照。

何时再做：开始统一分配 system、history、retrieval、user input 和 output reserve 预算时。

目标结果：请求前计算结果与 Ollama 返回的实际 prompt token 数在黄金样例中一致。

### 4. 多模型 tokenizer 注册表

为什么需要：生产系统可能同时服务多个模型，每个模型需要绑定自己的 tokenizer 和上下文上限。

何时再做：项目引入第二个真实模型时。

目标结果：通过配置文件按模型名选择 tokenizer 资产、SHA256、chat template 和 context limit。

### 5. 性能与内存优化

为什么需要：当前检查器一次读取完整 JSON，tokenizer 在启动时加载一次；对单模型已经足够，但多模型和高并发时需要测量。

何时再做：真实压测发现启动时间、内存或并发吞吐成为瓶颈时。

目标结果：有 benchmark 数据后再决定流式解析、缓存、对象池或并发控制，不提前优化。

### 6. SHA256 参数格式校验

为什么需要：当前实际 SHA256 一定是 64 位十六进制，但命令对人工输入只做一致性比较，没有单独提示长度或字符格式错误。

何时再做：该参数进入正式配置文件，或实际出现难以定位的人工配置错误时。

目标结果：在比较前验证 64 位十六进制格式，并返回更明确的配置错误。

### 7. 模型上限与运行时 context size 对照

为什么需要：`/api/show` 的 `context_length` 是模型元数据上限，Ollama 某次实际运行可能配置更小的 `num_ctx`。

何时再做：开始实现统一生产预算，或发现长上下文在达到模型上限前就被截断时。

目标结果：同时展示模型能力上限、运行时有效 context size 和项目预算，并使用三者中的安全值。

### 8. `/api/chat` 多轮消息的精确模板渲染对照

为什么需要：当前模型模板使用 `.Prompt`，Ollama 如何把多轮 `messages` 整理成该值属于运行时模板处理细节。

何时再做：需要让本地计算结果与 Ollama 实际 prompt token 数严格一致时。

目标结果：准备多轮黄金样例，对照 Ollama 实际渲染结果和本地模板计数结果。

### 9. 复杂 Ollama template 函数兼容

为什么需要：当前本机 Qwen template 只使用 Go template 内置的 `if` 和字段；其他模型可能依赖 Ollama 提供的额外模板函数。

何时再做：切换模型后，`promptbudget.Render` 因未知函数解析失败时。

目标结果：只补实际模型所需的函数映射和测试，不提前复制 Ollama 的完整模板运行时。

### 10. Model metadata 缓存

为什么需要：automatic 模式当前每次请求都会调用一次 Ollama `/api/show`。

何时再做：真实并发或多模型服务开始运行时。

目标结果：按模型缓存 context length，并在模型版本或配置变化时安全刷新。

### 11. 历史消息成对裁剪

为什么需要：当前严格窗口按单条 message 选择，极端预算下可能只保留 assistant 而丢掉对应 user 问题。

何时再做：session summary 和上下文质量课程开始时。

目标结果：定义 turn 边界，在容量允许时优先保留完整 user/assistant turn，并明确 tool message 的归属。

### 12. 预算与 Ollama usage 对照监控

为什么需要：当前 `output_token_reserve` 已绑定 `num_predict`，但 API 尚未保存 Ollama 的 `prompt_eval_count` 和 `eval_count`。

何时再做：需要评估预算利用率、调整默认回答预留或验证本地 token 误差时。

目标结果：记录计划值和 Ollama 实际 usage，形成误差与利用率指标。

## Session Summary 优化项

### 1. 触发阈值的运行数据校准

为什么需要：当前默认 `8 messages / 2048 tokens` 是可运行起点，不是经过真实会话成本和摘要质量验证的最佳值。

何时再做：积累真实 summary 触发、输入长度、输出质量和模型耗时数据后。

目标结果：按模型和业务场景配置阈值，并能解释成本、延迟和信息保留率之间的取舍。

### 2. 并发触发去重与冷却

为什么需要：同一 session 的并发请求可能同时观察到相同水位并重复生成摘要。

何时再做：实现 summary MySQL upsert 和接入 `/chat` 时。

目标结果：使用 version 乐观锁、session 级锁或任务幂等键，确保同一消息区间只被成功提交一次。

### 3. 消息删除后的水位语义

为什么需要：`last_message_id` 只表示已覆盖的最大 ID，不能通过 `latest_id - last_message_id` 直接推导真实消息数量。

何时再做：实现 MySQL 未摘要消息查询时。

目标结果：始终查询实际消息集合和计数，明确清理、归档和数据迁移对 summary watermark 的影响。

### 4. 摘要结构化输出与质量评估

为什么需要：当前 qwen 可能添加 wrapper、引导语或建议，prompt 只能降低概率，不能严格保证形态和事实质量。

何时再做：摘要闭环跑通并积累真实样例后。

目标结果：评估 Ollama JSON schema/结构化输出，建立事实保留、编造和冗余样例；只做安全 wrapper 解析，不用关键词规则误删正文。

### 5. 未摘要消息查询索引

为什么需要：滚动摘要按 `(session_id, user_id, id > watermark)` 查询并按 `id` 排序，现有教学表索引不是这个完整顺序。

何时再做：消息量进入压测或生产迁移设计时。

目标结果：用 `EXPLAIN` 验证查询计划，再增加匹配访问路径的复合索引；通过独立 migration 修改已有表，不把 `CREATE TABLE IF NOT EXISTS` 误当成索引迁移。

---

## 后续主题如何追加

以后 chunking、retrieval、summary、memory 等课程遇到非主线优化，也继续追加到本文档，并保持：

```text
优化项
-> 为什么需要
-> 何时再做
-> 目标结果
```
