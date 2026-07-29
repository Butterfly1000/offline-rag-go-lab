# Production Offline RAG Course Blueprint

主题：从当前教学实践走到生产级离线 RAG 的完整课程蓝图

## 1. 如何使用这份蓝图

这份文档是课程编号、顺序、状态和阶段门禁的唯一权威来源。

配套文档各自只负责一件事：

- [世界地图](../../world/game-world-map.md)：快速理解整个系统和主线阶段。
- [学习状态](00-learning-status.md)：记录当前学到、实现到和下一次从哪里继续。
- [教学协议](00-teaching-protocol.md)：规定如何讲、如何实践和何时改变状态。
- [优化 Backlog](00-optimization-backlog.md)：保存证据尚不足、暂时不能成为正式课程的候选项。

蓝图是一条经过完整推演的当前主航线，不是永远不能变化的承诺。后续调整必须由真实
数据、故障、安全要求或部署约束驱动，不能因为某项技术流行就临时改变方向。

## 2. 当前终局

当前课程终点是 L78：在干净 Linux 单机或小型内网环境中，交付一套可离线部署、可评估、
可隔离、可观测、可恢复、可升级回滚的 RAG 系统。

L78 通过时，系统必须能够：

1. 在断网环境准备、校验并安装应用、依赖和模型资产。
2. 导入、更新、删除、重建并追溯真实文档。
3. 用版本化数据集评估检索、引用和回答质量。
4. 让 recent window、session summary、long-term memory 和 document retrieval 共同工作。
5. 保证用户、记忆和知识域之间的强隔离。
6. 在 MySQL、Qdrant、Ollama 或 worker 故障时表现出确定的失败、降级和恢复行为。
7. 完成备份恢复、版本升级、数据迁移、模型替换和失败回滚。
8. 由另一位工程师根据运行手册重复完成最终验收。

当前终局明确不包含：

- 公网 SaaS、计费和复杂企业审批。
- Kubernetes、多地域和互联网规模分布式架构。
- 复杂管理前端或终端用户 UI。
- 基础模型训练、微调平台和 GPU 集群调度。
- 没有真实样本与门禁的“支持所有格式、模型和中间件”。

## 3. 状态模型

每节课只允许使用四种状态：

- `已学习`：实践完成、机器验证通过，并且用户已确认理解。
- `已实现待学习`：实践和机器验证已完成，但用户尚未逐节确认理解。
- `已规划`：目标、依赖、产物和门禁已确定，尚未开始实现。
- `阻塞`：已经开始，但缺少不可替代的外部条件，并留有阻塞证据。

状态变化遵守：

```text
已规划
  -> 实践实现并通过机器验证
已实现待学习
  -> 用户逐节确认理解
已学习
```

提交代码或写完 SOP 不能直接变成 `已学习`。单课测试通过也不能代替阶段门禁。

## 4. 当前坐标

- 已学习：L01-L23，共 23 节。
- 已实现待学习：L24-L33，共 10 节。
- 已规划：L34-L78，共 45 节。
- 下一次教学：从 L24 开始。
- 下一次新实践实现：从 L34 开始。
- 当前实现边界：L33。
- 当前蓝图终点：L78。

## 5. 完整课程路线

## 阶段一：Tokenizer、Prompt 与 Token Budget（L01-L12）

- 目标：让系统能够按真实模型格式准确计量上下文，并自动分配 Chat 历史预算。
- 依赖：前置 gateway、recent window 和 tokenizer 资产准备课程。
- 阶段实践产物：Tokenizer 工具、Prompt 渲染、预算计算、严格窗口和真实 `/chat` 接入。
- 阶段验收门禁：模型容量、固定输入、历史预算和输出预留加法闭合；超限输入硬失败。
- 非目标：本阶段不解决摘要、长期记忆和知识文档检索。

### L01 · Tokenizer Load Once

- 状态：`已学习`
- 依赖：Qwen2 `tokenizer.json` 资产已准备。
- 实践产物：`internal/tokenizerdemo` 加载器与 [运行 SOP](tokenizer-load-once-sop.md)。
- 验收：启动时只加载一次，重复计数稳定，缺失或非法文件立即报错。

### L02 · Tokenizer Components

- 状态：`已学习`
- 依赖：L01。
- 实践产物：Tokenizer 结构检查器与 [组件 SOP](tokenizer-inspect-sop.md)。
- 验收：能够解释 model、normalizer、pre-tokenizer、post-processor、added tokens 和词表规模。

### L03 · Tokenizer Fingerprint

- 状态：`已学习`
- 依赖：L01-L02。
- 实践产物：SHA256 校验与 [资产指纹 SOP](tokenizer-fingerprint-sop.md)。
- 验收：正确指纹通过，错误指纹在执行任何 token 计算前失败。

### L04 · Ollama Model Metadata

- 状态：`已学习`
- 依赖：L03、可用的本地 Ollama 模型。
- 实践产物：`/api/show` 检查器与 [模型检查 SOP](ollama-model-inspect-sop.md)。
- 验收：读取模型 context length 和 prompt template，并区分模型上限与运行时配置。

### L05 · Render Prompt Template

- 状态：`已学习`
- 依赖：L04。
- 实践产物：Go `text/template` 渲染器与 [模板渲染 SOP](prompt-template-render-sop.md)。
- 验收：相同 system/user 输入稳定生成当前模型预期的 prompt 文本。

### L06 · Prompt Token Overhead

- 状态：`已学习`
- 依赖：L01、L05。
- 实践产物：正文、渲染结果和模板开销对照与 [计数 SOP](prompt-template-token-overhead-sop.md)。
- 验收：能够用真实 Tokenizer 解释正文 token 与实际 prompt token 的差值。

### L07 · Context Budget

- 状态：`已学习`
- 依赖：L04、L06。
- 实践产物：上下文预算计划器与 [预算 SOP](context-budget-plan-sop.md)。
- 验收：`context = fixed + history + output reserve` 恒等式闭合，不可能的预算被拒绝。

### L08 · Qwen Message Format

- 状态：`已学习`
- 依赖：L05-L07。
- 实践产物：Qwen ChatML 消息格式化与 [消息格式 SOP](qwen-message-format-sop.md)。
- 验收：role、消息边界和结束标记正确，非法 role 被拒绝。

### L09 · Conversation Token Count

- 状态：`已学习`
- 依赖：L08。
- 实践产物：完整 conversation 一次性计数与 [对话计数 SOP](conversation-token-count-sop.md)。
- 验收：system、history、current user 和 assistant prefix 共同参与真实计数。

### L10 · Template-aware Recent Window

- 状态：`已学习`
- 依赖：L08-L09。
- 实践产物：严格 token recent window 与 [窗口 SOP](recent-window-template-token-sop.md)。
- 验收：只保留预算内最新消息，最新消息自身超限时返回空窗口而不突破预算。

### L11 · Automatic History Budget

- 状态：`已学习`
- 依赖：L07、L09-L10。
- 实践产物：自动历史预算服务与 [自动预算 SOP](automatic-history-budget-sop.md)。
- 验收：从模型容量、固定输入和回答预留自动计算可用历史额度。

### L12 · Chat Automatic Budget

- 状态：`已学习`
- 依赖：L11、recent-chat 真实 MySQL/Ollama 链路。
- 实践产物：真实 `/chat` 自动预算接入与 [Chat SOP](recent-chat-automatic-token-budget-sop.md)。
- 验收：API 返回预算明细，`num_predict` 与 output reserve 一致，手工/自动预算冲突被拒绝。

## 阶段二：Session Summary（L13-L18）

- 目标：把被 recent window 驱逐的连续历史压缩成可滚动、可并发保护的会话摘要。
- 依赖：L01-L12。
- 阶段实践产物：触发、选择、生成、MySQL 保存、滚动更新和真实 Chat 接入。
- 阶段验收门禁：summary content、version、watermark 和 token reserve 在真实链路中可观察。
- 非目标：本阶段不把会话摘要当成跨会话长期记忆。

### L13 · Summary Trigger

- 状态：`已学习`
- 依赖：L12。
- 实践产物：Summary record、watermark 与 [触发策略 SOP](session-summary-trigger-sop.md)。
- 验收：只有发生驱逐且消息数或 token 阈值满足时触发，非法统计被拒绝。

### L14 · Summary Message Selection

- 状态：`已学习`
- 依赖：L13。
- 实践产物：连续旧消息前缀选择器与 [选择 SOP](session-summary-selection-sop.md)。
- 验收：按 watermark 和 recent 起点选择真实存在的连续前缀，ID 有空洞时仍正确推进。

### L15 · Summary Generation

- 状态：`已学习`
- 依赖：L14、本地 Ollama。
- 实践产物：滚动摘要生成器与 [生成 SOP](session-summary-generation-sop.md)。
- 验收：旧摘要与新增驱逐消息生成非空新摘要，历史内容被视为不可信数据而非指令。

### L16 · Summary Store

- 状态：`已学习`
- 依赖：L13-L15、MySQL。
- 实践产物：version 乐观锁 MySQL Store 与 [持久化 SOP](session-summary-store-sop.md)。
- 验收：content/watermark 原子保存，版本冲突拒绝覆盖，事务失败不留下部分状态。

### L17 · Summary Update

- 状态：`已学习`
- 依赖：L13-L16。
- 实践产物：message source、selector、trigger、generator、store 编排与 [更新 SOP](session-summary-update-sop.md)。
- 验收：无触发为确定性 no-op，成功后只推进到实际摘要覆盖的最后消息。

### L18 · Chat Session Summary

- 状态：`已学习`
- 依赖：L12、L17。
- 实践产物：Summary 与 recent window 的真实 `/chat` 接入和 [集成 SOP](recent-chat-session-summary-sop.md)。
- 验收：真实两轮请求证明 summary 创建、更新和使用，summary reserve 进入总预算。

## 阶段三：Long-term Memory（L19-L23）

- 目标：把跨会话稳定事实提取为可追溯、可更新、可遗忘和可语义检索的 memory item。
- 依赖：L01-L18。
- 阶段实践产物：候选校验、模型提取、生命周期决策、MySQL 事实库和 Qdrant 索引。
- 阶段验收门禁：用户隔离为硬边界，MySQL item/evidence 与 Qdrant point 行为可重复验证。
- 非目标：本阶段不把模型输出直接当事实，也不把 Qdrant 当事实源。

### L19 · Memory Candidate Validation

- 状态：`已学习`
- 依赖：L18。
- 实践产物：五类 memory item、candidate 和来源边界与 [校验 SOP](memory-item-validation-sop.md)。
- 验收：user/session/source 匹配，kind/key/value 边界明确，越权或畸形候选硬失败。

### L20 · Memory Extraction

- 状态：`已学习`
- 依赖：L19、本地 Ollama。
- 实践产物：结构化候选提取与 [提取 SOP](memory-item-extraction-sop.md)。
- 验收：模型响应 strict decode 后再经 Go validator；越界 confidence 和无证据事实被拒绝。

### L21 · Memory Resolution

- 状态：`已学习`
- 依赖：L19-L20。
- 实践产物：确定性 INSERT、UPDATE、NOOP、FORGET、恢复决策与 [决策 SOP](memory-item-resolution-sop.md)。
- 验收：相同输入重复运行不产生无意义更新，遗忘和恢复具有明确 version 变化。

### L22 · Memory Store

- 状态：`已学习`
- 依赖：L21、MySQL。
- 实践产物：Item/evidence 事务 Store 与 [存储 SOP](memory-item-store-sop.md)。
- 验收：item 与 evidence 原子提交，冲突可识别，回滚不留下孤儿 evidence。

### L23 · Memory Qdrant

- 状态：`已学习`
- 依赖：L22、bge-m3、Qdrant。
- 实践产物：1024/Cosine collection、payload index 与 [向量检索 SOP](memory-item-qdrant-sop.md)。
- 验收：语义相近的两个测试用户只能命中自身 memory，forgotten point 被删除。

## 阶段四：Dual Retrieval（L24-L28）

- 目标：把用户记忆与知识文档安全、可降级、可预算地接入同一轮真实 Chat。
- 依赖：L01-L23。
- 阶段实践产物：统一 Hit、文档索引、并行检索、确定性合并和 `/chat` 集成。
- 阶段验收门禁：ownership 错误硬失败，基础设施错误按策略降级，retrieval context 不超预算。
- 非目标：本阶段不假设两路 raw score 可直接比较，也不提前引入 reranker。

### L24 · Context Hit Boundary

- 状态：`已实现待学习`
- 依赖：L23。
- 实践产物：统一 Hit 与 memory/document ownership 边界及 [边界 SOP](context-hit-boundary-sop.md)。
- 验收：用户记忆要求 user ownership，知识文档要求 scope ownership，畸形或越界结果硬失败。

### L25 · Document Qdrant

- 状态：`已实现待学习`
- 依赖：L24、Qdrant、bge-m3。
- 实践产物：独立文档 collection、payload index 与 [文档检索 SOP](document-qdrant-sop.md)。
- 验收：按 `knowledge_scope` 服务端过滤并返回后重验，不存在 scope 时返回零文档。

### L26 · Dual Retrieval

- 状态：`已实现待学习`
- 依赖：L23-L25。
- 实践产物：一次 embedding、并行 memory/document retrieval 和 [双路检索 SOP](dual-retrieval-sop.md)。
- 验收：基础设施失败可按来源 warning 降级，ownership 与畸形数据始终硬失败。

### L27 · Context Merge Budget

- 状态：`已实现待学习`
- 依赖：L26、L09。
- 实践产物：独立排序、固定 quota、去重、安全渲染和 [合并预算 SOP](context-merge-budget-sop.md)。
- 验收：确定性结果不超过 retrieval token 子预算，截断不会生成不完整边界。

### L28 · Chat Dual Retrieval

- 状态：`已实现待学习`
- 依赖：L18、L27。
- 实践产物：Dual Retrieval、Summary、recent window、Ollama、MySQL 的 [真实 Chat SOP](recent-chat-dual-retrieval-sop.md)。
- 验收：真实请求同时观察 memory/document 命中、预算、warning 和持久化消息。

## 阶段五：Production Document Ingestion（L29-L33）

- 目标：让文档从稳定身份、结构化切块到幂等构建、原子发布和检索评估形成闭环。
- 依赖：L01-L28。
- 阶段实践产物：文档版本模型、chunker、ingestion、snapshot alias 和 Golden Evaluation。
- 阶段验收门禁：构建可重试、发布可验证可回滚、scope 隔离和 forbidden hit 门禁通过。
- 非目标：本阶段只支持教学所需 Markdown/Go，不假装覆盖所有生产文档格式。

### L29 · Document Identity and Version

- 状态：`已实现待学习`
- 依赖：L25、L28。
- 实践产物：逻辑文档、版本、稳定 chunk ID、状态机和 [身份 SOP](document-identity-version-sop.md)。
- 验收：未变化 chunk 跨版本保持 ID，内容/结构变化产生可解释的新身份，非法状态转换失败。

### L30 · Structured Document Chunking

- 状态：`已实现待学习`
- 依赖：L29、L01。
- 实践产物：Markdown heading/fence 与 Go AST chunker 和 [切块 SOP](structured-document-chunking-sop.md)。
- 验收：结构路径保留，所有 chunk 由真实 Tokenizer 强制上限，不产生 overlap-only chunk。

### L31 · Idempotent Document Ingestion

- 状态：`已实现待学习`
- 依赖：L29-L30、MySQL、Qdrant、bge-m3。
- 实践产物：批量 embedding、稳定 point、manifest、失败重试和 [幂等入库 SOP](idempotent-document-ingestion-sop.md)。
- 验收：相同 build 第二次运行 embedding/upsert 均为零，失败不把不完整版本标记 ready。

### L32 · Snapshot and Alias Publication

- 状态：`已实现待学习`
- 依赖：L31。
- 实践产物：完整语料 snapshot、逐 point verification、alias 切换/回滚和 [发布 SOP](document-snapshot-alias-sop.md)。
- 验收：未验证 snapshot 不能发布；alias 原子切换；失败回滚不依赖删除旧 collection。

### L33 · Retrieval Evaluation

- 状态：`已实现待学习`
- 依赖：L32、L25-L28。
- 实践产物：12 个 Golden Cases、Recall@3、MRR@3、隔离和 [评估 SOP](document-retrieval-evaluation-sop.md)。
- 验收：教学 fixture 达到 Recall@3=1、MRR@3=1、scope isolation=100%、forbidden hit=0。

## 阶段六：检索质量工程（L34-L38）

- 目标：用生产数据证明检索策略的质量、代价和适用边界。
- 依赖：L24-L33。
- 阶段实践产物：版本化数据集、Sparse/Hybrid 检索、Reranker 和可解释检索决策策略。
- 阶段验收门禁：所有改动对比固定基线；forbidden hit 为零；质量、延迟和资源成本同报。
- 非目标：没有标注数据时，不引入仅凭主观感觉配置的复杂模型。

### L34 · Production Golden Dataset

- 状态：`已规划`
- 依赖：L33。
- 实践产物：版本化 query、正例、负例、forbidden hit、问题分类和基线报告。
- 验收：同一 dataset/version 可重复运行，并能把失败归类为解析、切块、召回、排序或隔离问题。

### L35 · Field-aware Sparse Retrieval

- 状态：`已规划`
- 依赖：L34、L29-L30。
- 实践产物：标题、路径、正文、代码字段感知的 BM25/关键词检索和独立评估报告。
- 验收：精确术语、标识符和错误码查询对比 Dense 基线，scope 过滤仍为硬门禁。

### L36 · Hybrid Candidate Fusion

- 状态：`已规划`
- 依赖：L35、现有 Dense Retrieval。
- 实践产物：Dense + Sparse 候选和确定性 RRF 等融合策略。
- 验收：一次查询的两路候选可追踪，融合结果稳定，并在 L34 数据集上对比单路基线。

### L37 · Reranker and Diversity

- 状态：`已规划`
- 依赖：L36。
- 实践产物：统一候选 Reranker、章节/文档多样性约束和无 Reranker fallback。
- 验收：报告 Recall/NDCG、回答前延迟和资源成本；Reranker 故障不会破坏 ownership。

### L38 · Retrieval Decision Policy

- 状态：`已规划`
- 依赖：L34-L37。
- 实践产物：Score Calibration、动态 quota、阈值、决策原因和检索回归门禁。
- 验收：按问题类型记录策略选择；校准和 quota 调整有数据支持；退化时回到确定性基线。

## 阶段七：有依据的回答（L39-L43）

- 目标：让系统从“检索到内容”升级为“引用证据、验证引用并在无证据时拒答”。
- 依赖：L34-L38。
- 阶段实践产物：Evidence/Citation 契约、Context Packing、回答验证和质量评估。
- 阶段验收门禁：引用可反查到 source/chunk/version，不可回答问题不能通过编造补齐。
- 非目标：不访问公网补证据，也不建设通用事实核查搜索引擎。

### L39 · Evidence and Citation Contract

- 状态：`已规划`
- 依赖：L29、L38。
- 实践产物：稳定 Evidence、Citation、SourceRef 结构和 Chat API 引用输出。
- 验收：每个引用能反查到真实 scope、document、version、chunk 和显示位置。

### L40 · Evidence-aware Context Packing

- 状态：`已规划`
- 依赖：L39、L27。
- 实践产物：去重、文档/章节多样性、位置感知和引用边界完整的上下文打包。
- 验收：打包不超 token 子预算，不拆坏引用边界，并记录每个证据被保留或丢弃的原因。

### L41 · Citation-aware Prompt Boundary

- 状态：`已规划`
- 依赖：L40、L15 的不可信历史边界。
- 实践产物：引用感知回答 Prompt 和不可信检索内容隔离。
- 验收：文档内指令不能改变 system、ownership、引用和拒答规则，恶意样例进入回归集。

### L42 · Grounded Answer Validation

- 状态：`已规划`
- 依赖：L39-L41。
- 实践产物：引用核对、unsupported claim 检查、证据不足拒答和安全 fallback。
- 验收：引用不存在、越界或不能支持声明时拒绝把回答标记为 grounded。

### L43 · Answer Quality Evaluation

- 状态：`已规划`
- 依赖：L34、L42。
- 实践产物：Answerability、Faithfulness、Citation Correctness 数据集和回归报告。
- 验收：可回答/不可回答样例、引用正确率和编造失败样例均进入版本化发布门禁。

## 阶段八：生产级长期记忆（L44-L48）

- 目标：让长期记忆具备可评估的提取质量、异步执行、一致性修复和可靠遗忘。
- 依赖：L19-L28、L43。
- 阶段实践产物：Ontology、提取数据集、worker/outbox、漂移修复和行为评估。
- 阶段验收门禁：MySQL 始终是事实源；重复任务幂等；forgotten/stale 事实不能参与回答。
- 非目标：不让 embedding 相似度自动覆盖事实，不把模型自报 confidence 当成概率。

### L44 · Memory Ontology and Schema Evolution

- 状态：`已规划`
- 依赖：L19-L23。
- 实践产物：受控 kind/key、alias、Schema version 和兼容迁移规则。
- 验收：同义 key 有确定归一化路径；旧记录迁移可预览、可重复并保留 evidence。

### L45 · Memory Extraction Evaluation

- 状态：`已规划`
- 依赖：L44、L20。
- 实践产物：提取 Golden Dataset、召回/误提取指标和 Confidence 校准报告。
- 验收：候选接受、拒绝和人工难例均可复现；confidence 不经校准不能驱动自动写入。

### L46 · Asynchronous Memory Extraction

- 状态：`已规划`
- 依赖：L21-L22、L45。
- 实践产物：异步任务、幂等键、有限重试、并发冲突处理和任务审计。
- 验收：重复投递不重复制造 evidence；同一 identity 并发更新不丢 version。

### L47 · Memory Outbox and Rebuild

- 状态：`已规划`
- 依赖：L23、L46。
- 实践产物：MySQL Outbox、Qdrant worker、漂移扫描、repair 和全量 rebuild。
- 验收：Qdrant 丢失或落后可从 MySQL 检测并重建；失败不回滚或覆盖 MySQL 事实。

### L48 · Memory Behavior Evaluation

- 状态：`已规划`
- 依赖：L43-L47。
- 实践产物：旧事实、冲突事实、遗忘和个性化回答端到端数据集。
- 验收：正确事实提高相关回答质量，过期/forgotten/其他用户事实不能进入上下文。

## 阶段九：生产文档管道（L49-L53）

- 目标：把教学格式 ingestion 扩展成可承载真实文件、坏文件和持续增量同步的生产管道。
- 依赖：L29-L38。
- 阶段实践产物：Source Registry、常见解析器、OCR、异步 worker 和数据血缘。
- 阶段验收门禁：坏文件隔离；更新/删除/重试/重建可重复；解析失败不能伪装成功。
- 非目标：没有样本和质量门禁时，不承诺支持所有文件格式。

### L49 · Source Registry and Connector Contract

- 状态：`已规划`
- 依赖：L29、L31。
- 实践产物：Source Registry、connector 接口、文件身份、扫描游标和权限元数据。
- 验收：同一来源重复扫描幂等，移动/修改/删除有确定事件，不绕过 knowledge scope。

### L50 · Common Document Parsers

- 状态：`已规划`
- 依赖：L49、L30。
- 实践产物：PDF、HTML、Office、纯文本解析器、格式 fixture 和资源限制。
- 验收：解析保留来源与结构；加密、损坏、超大或恶意文件进入明确失败/隔离路径。

### L51 · OCR and Layout-aware Extraction

- 状态：`已规划`
- 依赖：L50。
- 实践产物：扫描文档 OCR、页码/版面/表格边界和质量标记。
- 验收：低置信内容可识别，页码与引用可追溯，OCR 失败不生成看似可靠的空文本。

### L52 · Asynchronous Ingestion Worker

- 状态：`已规划`
- 依赖：L31-L32、L49-L51。
- 实践产物：Job 状态机、重试、背压、quarantine、重放和 worker 指标。
- 验收：单个坏文件不阻塞队列；重复任务复用 build identity；失败有可操作原因。

### L53 · Corpus Lineage and Rebuild

- 状态：`已规划`
- 依赖：L32-L33、L52。
- 实践产物：增量同步、删除传播、source-to-point 血缘和全语料重建验收。
- 验收：任一 point 可追溯到源文件与构建；删除不留可检索残影；全量重建结果可比较。

## 阶段十：离线模型与资产供应链（L54-L58）

- 目标：让模型、Tokenizer、依赖和硬件容量在断网环境中可识别、可重复、可回滚。
- 依赖：L01-L12、L23、L38。
- 阶段实践产物：Model Manifest、能力注册表、离线资产包、黄金对照和容量档位。
- 阶段验收门禁：不匹配时启动失败；新机器不临时联网；模型替换通过黄金集并可回滚。
- 非目标：不建立模型训练或 GPU 集群调度平台。

### L54 · Model Asset Manifest

- 状态：`已规划`
- 依赖：L03-L04、L23。
- 实践产物：生成模型、Embedding、Tokenizer 的来源、revision、license、大小和 SHA256 清单。
- 验收：缺失、损坏或未经批准的资产在启动/安装阶段被拒绝。

### L55 · Model Capability Registry

- 状态：`已规划`
- 依赖：L54、L04-L07。
- 实践产物：按模型绑定 tokenizer、template、context、embedding dimension 和能力标记的注册表。
- 验收：模型选择不依赖散落默认值，未知模型和维度冲突硬失败。

### L56 · Offline Bootstrap Bundle

- 状态：`已规划`
- 依赖：L54-L55。
- 实践产物：Go module、应用、模型、配置模板和服务依赖的离线初始化包。
- 验收：断网干净环境可验证资产并完成初始化，不在运行时偷偷下载。

### L57 · Cross-runtime Golden Compatibility

- 状态：`已规划`
- 依赖：L55-L56、L09。
- 实践产物：官方/参考 runtime 与 Go 实现的 token ID、prompt、embedding 黄金对照矩阵。
- 验收：中文、英文、代码、added token 和完整 Chat 样例结果受版本控制；升级先过黄金集。

### L58 · Model Capacity and Replacement

- 状态：`已规划`
- 依赖：L55-L57。
- 实践产物：预热、CPU/GPU/内存档位、并发限制、模型切换和回滚报告。
- 验收：每个硬件档位有稳定容量边界；新模型质量/资源门禁失败可恢复旧模型。

## 阶段十一：运行时可靠性（L59-L63）

- 目标：让 API、并发、重试、资源和性能在真实压力与故障下保持确定行为。
- 依赖：L43、L48、L53、L58。
- 阶段实践产物：版本化 API、并发保护、故障矩阵、资源治理和 SLO 报告。
- 阶段验收门禁：重试不重复写事实；故障符合矩阵；SLO 记录硬件与统计方法。
- 非目标：不追求互联网规模吞吐和跨地域高可用。

### L59 · Stable API and Idempotency

- 状态：`已规划`
- 依赖：L28、L53。
- 实践产物：Chat/Ingest/Admin API 契约、版本策略、请求 ID 和幂等结果。
- 验收：相同幂等键不重复写消息、版本或任务；不兼容请求得到明确错误。

### L60 · Session and Worker Concurrency

- 状态：`已规划`
- 依赖：L17、L46、L52、L59。
- 实践产物：session 锁、水位并发、重复任务治理和真实并发集成测试。
- 验收：并发摘要、记忆和 ingestion 不丢更新、不重复提交、不越过 watermark。

### L61 · Timeout, Retry and Degradation

- 状态：`已规划`
- 依赖：L26、L47、L52、L60。
- 实践产物：MySQL/Qdrant/Ollama/worker 超时、有限重试、circuit breaker 和降级矩阵。
- 验收：每个失败是硬失败、warning 降级或异步补偿之一；安全错误永不降级。

### L62 · Resource Governance

- 状态：`已规划`
- 依赖：L58、L61。
- 实践产物：批处理、缓存、并发信号量、队列背压、用户/任务资源配额。
- 验收：Chat、OCR、Embedding 和 rebuild 不会无界争抢资源；缓存不绕过身份和版本。

### L63 · Performance and Resilience SLO

- 状态：`已规划`
- 依赖：L59-L62。
- 实践产物：参考硬件性能基线、压力/长稳/故障注入测试和 SLO 报告。
- 验收：报告 p50/p95/p99、吞吐、错误率和资源曲线；连续运行及依赖故障符合目标行为。

## 阶段十二：安全、权限与隐私（L64-L68）

- 目标：把 ownership 从代码约定提升为认证、授权、恶意输入和数据生命周期的发布门禁。
- 依赖：L43、L48、L53、L59-L63。
- 阶段实践产物：威胁模型、权限矩阵、攻击回归、审计和删除/导出流程。
- 阶段验收门禁：越权、恶意语料执行和删除后残留均阻断发布，不允许 warning 降级。
- 非目标：不实现公网身份平台、计费和复杂企业审批。

### L64 · Threat Model and Data Classification

- 状态：`已规划`
- 依赖：L43、L48、L53。
- 实践产物：资产、攻击者、入口、信任边界、数据等级和风险处置清单。
- 验收：Chat、Upload、Model、Store、Backup 和 Admin 边界都有明确 owner 与威胁。

### L65 · Authentication and Authorization

- 状态：`已规划`
- 依赖：L59、L64。
- 实践产物：principal、认证、角色/能力授权和集中 ownership/scope 校验。
- 验收：伪造 user/scope、横向读取和管理员接口越权全部被拒绝并进入审计。

### L66 · Prompt and Document Attack Defense

- 状态：`已规划`
- 依赖：L41、L50-L51、L64-L65。
- 实践产物：Prompt Injection、恶意文档、压缩炸弹、超大输入和解析器攻击回归集。
- 验收：不可信内容不能改变系统/权限规则；资源攻击被限制；失败不泄露内部敏感信息。

### L67 · Secrets, Encryption and Audit

- 状态：`已规划`
- 依赖：L64-L66。
- 实践产物：Secret 管理、传输/静态加密策略、敏感字段处理、审计事件和轮换 SOP。
- 验收：日志和错误不泄露 secret；敏感操作可追踪；密钥轮换不破坏已有数据恢复。

### L68 · Retention, Export and Verified Deletion

- 状态：`已规划`
- 依赖：L47-L48、L53、L67。
- 实践产物：保留策略、用户数据导出、MySQL/Qdrant/summary/cache/backup 删除流程和安全门禁。
- 验收：删除后在线检索不到，重建不复活，备份恢复路径按策略阻止过期数据重新出现。

## 阶段十三：可观测与可恢复运维（L69-L73）

- 目标：让故障能够被发现、定位、修复和从经过验证的备份恢复。
- 依赖：L59-L68。
- 阶段实践产物：日志关联、指标追踪、健康告警、备份恢复和事故 runbook。
- 阶段验收门禁：关键故障可定位；备份经过恢复演练；repair 可预览、幂等、可审计。
- 非目标：不自研通用监控平台，优先复用成熟标准与组件。

### L69 · Structured Logging and Correlation

- 状态：`已规划`
- 依赖：L59、L67。
- 实践产物：request/session/job/correlation ID、结构化事件和审计事件分流。
- 验收：一次请求可关联 Chat、Retrieval、Model、Store 和 worker，且日志不含 secret。

### L70 · Metrics and Tracing

- 状态：`已规划`
- 依赖：L63、L69。
- 实践产物：请求、token、检索、模型、队列、错误和资源 SLI 及 trace。
- 验收：能区分检索慢、模型慢、数据库慢和排队慢；指标标签不制造用户级高基数泄漏。

### L71 · Health, Readiness and Alerting

- 状态：`已规划`
- 依赖：L61、L70。
- 实践产物：进程存活、服务就绪、依赖诊断、降级状态和可操作告警。
- 验收：依赖异常时 readiness/告警符合故障矩阵，不用单一 HTTP 200 假装健康。

### L72 · Backup and Restore

- 状态：`已规划`
- 依赖：L53、L68、L71。
- 实践产物：MySQL、Qdrant、源文件、配置、manifest 的一致备份和干净环境恢复报告。
- 验收：恢复后数据版本、alias、ownership、删除状态和检索质量通过对照。

### L73 · Incident Runbook and Repair

- 状态：`已规划`
- 依赖：L47、L53、L69-L72。
- 实践产物：常见事故手册、对账、dry-run repair、恢复演练和审计记录。
- 验收：另一位工程师能按 runbook 定位并修复索引漂移、失败发布或依赖中断。

## 阶段十四：交付、升级与最终验收（L74-L78）

- 目标：把全部能力包装成可重复安装、升级、回滚并由他人验收的离线发布物。
- 依赖：L01-L73。
- 阶段实践产物：Migration、离线发布包、Linux 部署、升级回滚和最终验收报告。
- 阶段验收门禁：干净环境完成安装、数据、质量、并发、故障、恢复和升级全流程。
- 非目标：不把进程启动或单次请求成功当成生产完成。

### L74 · Configuration and Schema Migration

- 状态：`已规划`
- 依赖：L55-L56、L59、L72。
- 实践产物：环境配置分层、配置校验、MySQL/Qdrant Migration 和兼容顺序。
- 验收：全新安装和从上一版本升级均可重复；失败 migration 可恢复且不产生半版本状态。

### L75 · Reproducible Offline Release

- 状态：`已规划`
- 依赖：L54-L58、L67、L74。
- 实践产物：可重复构建、SBOM、license、checksum、版本说明和签名离线发布包。
- 验收：发布包内容可审计和验证，缺件/篡改失败，安装不访问未声明网络。

### L76 · Linux Single-node Deployment

- 状态：`已规划`
- 依赖：L71、L75。
- 实践产物：Linux 单机/小型内网拓扑、服务管理、持久卷、权限和冷启动 SOP。
- 验收：干净参考环境按 SOP 启动所有服务，通过 readiness 并完成第一份语料导入。

### L77 · Upgrade and Rollback Drill

- 状态：`已规划`
- 依赖：L72、L74-L76。
- 实践产物：应用、Schema、数据和模型升级顺序、兼容窗口、失败注入与回滚报告。
- 验收：升级成功路径和中途失败路径都保持数据可恢复，旧版本能按策略重新服务。

### L78 · Production Acceptance Boss

- 状态：`已规划`
- 依赖：L01-L77。
- 实践产物：最终验收脚本/清单、证据包、运行手册和签字报告。
- 验收：完成第 6 节定义的八项 Boss 战，且另一位工程师能够在干净环境复现。

## 6. 最终生产验收

L78 必须依次留下以下证据：

1. 离线冷安装：记录环境、发布包校验、服务版本和 readiness。
2. 数据生命周期：导入、更新、删除并重建一套真实语料。
3. 质量门禁：运行固定版本的检索、回答、引用、记忆和隔离数据集。
4. 并发与隔离：执行多用户并发 Chat，证明 user/scope forbidden hit 为零。
5. 故障演练：分别模拟 Ollama、Qdrant、MySQL 和 worker 故障及恢复。
6. 备份恢复：在干净环境恢复数据并重新通过身份、删除和质量检查。
7. 升级回滚：执行一次版本升级、注入一次中途失败并恢复到可服务状态。
8. 他人复现：由未参与实现的人按运行手册重复执行，并记录差异与结论。

任何安全隔离失败、数据丢失、删除后复活、不可恢复升级或无依据回答门禁失败，都阻断
“生产级离线 RAG”验收。

## 7. 蓝图变更规则

允许改变课程内容或顺序的证据：

- 真实 Golden Dataset 结果。
- 可复现的正确性、性能、容量或恢复故障。
- 新的安全、隐私、合规或部署要求。
- 实践证明原依赖关系无法实施。

不允许作为变更理由：

- 某项技术流行。
- 只想增加课程数量。
- 没有样本却预设新模型或中间件一定更好。
- 为了让状态好看而降低门禁。

调整时必须：

1. 更新本蓝图并记录原因。
2. 同步 [世界地图](../../world/game-world-map.md) 和 [学习状态](00-learning-status.md)。
3. 已被教学记录引用的编号原则上不重排；必须重排时提供旧编号到新编号的迁移表。
4. 新优化先进入 [Backlog](00-optimization-backlog.md)，达到证据条件后再成为正式课程。
