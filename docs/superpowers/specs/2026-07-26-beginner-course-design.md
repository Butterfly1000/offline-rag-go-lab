# AI 小白课程设计

## 目标

在不改写现有工程型教材的前提下，为 `docs/teaching/*.md` 直系目录中 42 篇非
`00-*` 课程或 SOP 各写一篇面向 AI 初学者的配套教材。

读者假设为：

- 会读基础 Go 代码，是初级程序员；
- 不理解 tokenizer、prompt、context、embedding、Qdrant、RAG、summary、
  memory、snapshot 和检索评估；
- 不要求学会编码，但需要知道系统里有什么、实际怎样操作、数据如何流动、结果怎样
  判断，并能在面试中准确复述。

现有 `00-*` 文件是课程协议、学习状态、运行日志、回归记录或 backlog，只作为备课和
事实核验材料，不另建小白课程。

## 方案选择

采用“保留原教材，新增一对一小白课程”的方案：

- 原文继续承担工程细节、代码说明和完整 SOP；
- 新文档放在 `docs/teaching/beginner/`，承担说人话、建立心智模型、解释操作链和面试
  复述；
- 第一批先在 `docs/teaching/beginner/00-course-map.md` 冻结 42 条来源 → 目标映射，
  后续机械检查只读这份 manifest，不递归扫描 `beginner/` 来发现来源；
- 每篇小白文档明确列出对应原课、实际命令入口、核心 Go 包和测试锚点；
- 不把不同原课合并成一篇，保证 42 对 42，可以机械检查是否漏课。

没有采用以下方案：

- 直接简化原文：会损失工程参考价值，也无法同时服务有经验读者；
- 每五课写一篇摘要：虽然短，但不满足“一节课对一节课”；
- 只覆盖连续 L01-L33：会漏掉 chat、ingest、早期 chunking、recent-window 和
  tokenizer 资产准备。

## 课程编号

连续实现主线保留现有的 L01-L33：

- L01-L12：Tokenizer、Chat Template 与真实 `/chat` 上下文预算；
- L13-L18：Session Summary；
- L19-L23：Long-term Memory Item；
- L24-L28：Dual Retrieval；
- L29-L33：Document Ingestion、Snapshot 与 Retrieval Evaluation。

主线之前的 9 篇材料使用 F01-F09，避免与 L01-L03 重号：

| 新编号 | 原课程 |
|---|---|
| F01 | `01-chat-behavior.md` |
| F02 | `02-ingest-behavior.md` |
| F03 | `03-chunking-behavior.md` |
| F04 | `recent-window-layer-01.md` |
| F05 | `recent-window-runtime-sop.md` |
| F06 | `recent-window-layer-02-count-distortion.md` |
| F07 | `tokenizer-demo-sop-qwen2.md` |
| F08 | `recent-window-layer-02b-token-budget.md` |
| F09 | `recent-window-layer-02c-session-summary.md` |

F01-F03 是早期 `internal/gateway` 最小演示；L12、L18、L28 使用更真实的
`recent-chat` 服务。每篇涉及二者的教材必须明确这是项目演进关系，不能说成同一个
服务的不同文件。

## 单课固定结构

每篇教材按同一个认知顺序编写：

1. 本课一句话：解决什么真实问题；
2. 先用人话理解：用业务或生活类比建立直觉；
3. 系统里有什么：文件、模型、服务、表、collection、配置与 Go 组件；
4. 一条完整链路：输入 → 处理 → 输出，注明数据最终落在哪里；
5. 实际怎么做：给出仓库已有命令或请求，不要求读者编写代码；
6. 结果怎么看：解释关键输出字段、成功证据和首要排错方向；
7. 算一遍或走一遍：至少一个带具体数字或具体状态变化的例子；
8. 常见误解：至少一个错误理解和删掉该机制后的具体后果；
9. 当前实现与生产边界：分别说明已保证、未保证和升级方向；
10. 面试怎么说：30 秒版本，以及可应对追问的 2-4 个展开点；
11. 自检题与答案：不要求编码，但能检验是否真的理解；
12. 事实锚点：原课、命令、Go 包、配置、SQL、测试。

“实际怎么做”还必须标明：

- 需要启动哪些服务和准备哪些资产；
- 命令属于纯本地、外部只读、业务写入、发布/回滚中的哪一级；
- 业务写入或发布命令会影响哪个 scope、session、collection、table 或 alias；
- 是否可以安全重复执行，以及重复执行时预期出现 no-op、覆盖还是新增记录。
- 哪些副作用不能自动恢复，能恢复时给出明确恢复方向。

数字必须区分三类：

- 公式或协议固定值；
- 当前仓库的配置值；
- 某次环境运行的示例值。

不能把 `32768`、`1024`、某个 token 数、某个 score 或某次命中数量写成所有模型和
环境都固定的结论。

## 三角色闭环

每批最多五课，严格按下面顺序交接：

1. 教师依据原课、实现和测试写初稿，并自查一对一映射；
2. 学生只从初级 Go 程序员、AI 小白视角试读：
   - 能否用一句人话说出问题；
   - 能否说出输入、处理、输出和数据落点；
   - 能否解释算例、输出与一个反例；
   - 能否完成 30 秒面试复述；
3. 学生有任何不理解，教师修订，学生重新确认；
4. 正确性审核者对照原课、Go 实现、测试和配置核验事实；
5. 审核发现重要错误或边界混淆，教师修订，学生复读，审核者复审；
6. 三个角色都确认后，运行机械检查并创建该批 commit。

“命令能运行”“测试通过”“学生看过”都不能单独代替三角色确认。

## 关键事实边界

所有课程共同遵守：

- token 不是字符数，必须由目标 tokenizer 对真实渲染文本编码；
- tokenizer 代码、`tokenizer.json` 资产、模型身份和 chat template 是不同对象；
- prompt 是 system、历史、summary、memory、document、当前问题和生成前缀等实际
  输入的组合，不只是用户正文；
- summary 是同一 session 的旧对话压缩，memory item 是带证据、可版本化、可跨
  session 使用的用户事实；
- 生成模型与 embedding 模型职责不同；
- `session_id`、`user_id`、`knowledge_scope` 不能混为一个隔离维度；
- 外部系统的 filter 不能替代返回后的 ownership 与 payload 重验；
- 基础设施错误可按设计降级，越权或数据完整性错误必须硬失败；
- 不同 collection 的 raw score 未校准时不能直接全局比较；
- 幂等不是“重复覆盖”，相同 ready 构建应不再 embedding/upsert；
- Qdrant alias 切换与 MySQL 激活没有跨系统事务，失败时可能需要 reconciliation；
- L25 的 document Qdrant 只用固定教学 fixture 验证 scope 检索，不负责真实文档解析、
  版本、幂等入库和发布；L29-L31 才补全“document point 从哪里来”的 ingestion
  生命周期；
- L25 与 L29 必须双向链接，并使用同一条关系链：
  `fixture 检索 → production-shaped ingestion → snapshot/alias 发布`；
- L25/L28 默认读取 `offline_rag_document_chunks_v1`，L29-L33 构建和发布
  `offline_rag_document_ingestion_lab_v1/v2` 与
  `offline_rag_document_ingestion_lab_active`；这是两套 collection 教学链，除非
  显式修改 recent-chat 配置，否则 L29-L33 的发布结果不会被真实 `/chat` 使用；
- L31 只产生 physical collection 中的 ready version 与 manifest；verified snapshot
  和 alias 发布从 L32 开始。当前 ingestion 不会自动删除同一物理 collection 中的旧
  points，失败重试只依靠稳定 point ID 覆盖同 ID；
- prompt 转义和“不可信数据”标记能降低注入风险，但不是权限边界；
- demo、fixture、一次真实运行和生产完备能力必须明确区分。

配置材料遵守秘密边界：

- 教材只展示 `config/recent-chat.env.example` 中的占位值；
- 不复制、引用或提交被 Git 忽略的真实 `config/recent-chat.env` 内容；
- 真实配置只允许核对键是否存在，任何输出都必须脱敏；
- 当前 `cmd/recent-chat` 由 `fileconfig.Load` 直接读取配置文件，shell 环境变量覆盖只
  适用于显式支持它的部分回归脚本，不能沿用旧教材中的笼统优先级说法。

## 验收与提交

机械验收至少包括：

- manifest 中冻结的 42 个原课程文件与 42 个小白课程文件一一映射；
- 来源只允许是 `docs/teaching/*.md` 直系非 `00-*` 文件，
  `docs/teaching/beginner/**` 永不作为来源；
- 课程地图无重复编号、无漏项、链接目标存在；
- 每篇都包含固定结构中的必要栏目；
- 命令、文件、包、配置字段和公式可在仓库中找到事实锚点；
- Markdown 相对链接检查通过；
- 写课前的 `go test ./...` 基线通过；
- 每批只暂存 manifest 指定的目标文件，`git diff --cached --check` 通过，并用
  `git diff --cached --name-only` 证明没有越界文件；
- 每批提交前的 `go test ./...` 通过，证明文档提交时工程仍处于健康状态；
- 空 module cache 的离线环境需预置依赖，不能把 Go module 下载能力当成课程保证。

提交按课程顺序每五篇一批：

- 批次 1：F01-F05；
- 批次 2：F06-F09、L01；
- 批次 3：L02-L06；
- 批次 4：L07-L11；
- 批次 5：L12-L16；
- 批次 6：L17-L21；
- 批次 7：L22-L26；
- 批次 8：L27-L31；
- 批次 9：L32-L33 和最终课程地图。

设计与计划文档单独提交；课程提交不执行 push。
