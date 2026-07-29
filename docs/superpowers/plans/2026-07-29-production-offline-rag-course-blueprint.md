# Production Offline RAG Course Blueprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立 L01-L78 连续、可验收的生产级离线 RAG 课程蓝图，并让世界地图、学习状态、接手入口和优化 backlog 与它保持一致。

**Architecture:** `docs/teaching/00-course-blueprint.md` 是课程定义的唯一真相源；`world/game-world-map.md` 只提供快速导航；`00-learning-status.md` 只维护动态坐标；teaching protocol、handoff 和 backlog 分别维护协作规则、接手入口和证据不足的候选优化。所有文档使用可迁移的仓库相对链接，最终用一次性只读命令验证编号、状态、链接、事实和修改范围。

**Tech Stack:** Markdown、POSIX shell、Git、`rg`、`sed`、`awk`

## Global Constraints

- 当前课程终点是 L78。
- L01-L23 状态为 `已学习`。
- L24-L33 状态为 `已实现待学习`。
- L34-L78 状态为 `已规划`。
- L34 之后每五节组成一个可独立验收的阶段。
- 终局面向 Linux 单机或小型内网部署。
- 不扩展到公网 SaaS、Kubernetes、多地域、复杂前端或模型训练。
- 只修改相关 Markdown 文档，不修改 Go、SQL、Shell、测试和 `docs/teaching/beginner/`。
- 保留用户现有的三个未跟踪 AirDroid Business 文档，不读取、不暂存、不修改。
- 不 push。
- 实施依据是 `docs/superpowers/specs/2026-07-29-production-offline-rag-course-blueprint-design.md`。

---

## File Structure

- Create: `docs/teaching/00-course-blueprint.md`
  - 唯一权威的 L01-L78 逐课清单、阶段定义、状态模型和终局门禁。
- Modify: `world/game-world-map.md`
  - 面向快速理解的世界入口、当前开放区域、阶段地图和导航。
- Modify: `docs/teaching/00-learning-status.md`
  - 当前实现/学习坐标、剩余课程数量和下一次教学入口。
- Modify: `docs/teaching/00-teaching-protocol.md`
  - 明确蓝图优先级、课程晋级状态和蓝图变更证据。
- Modify: `docs/teaching/00-handoff-guide.md`
  - 给新模型的最小读取顺序和教学/实现接手提示词。
- Modify: `docs/teaching/00-optimization-backlog.md`
  - 给已进入 L34-L78 的优化项添加正式课程映射，保留未成熟候选。

---

### Task 1: 建立权威课程蓝图

**Files:**
- Create: `docs/teaching/00-course-blueprint.md`
- Reference: `docs/superpowers/specs/2026-07-29-production-offline-rag-course-blueprint-design.md`
- Reference: `docs/teaching/00-learning-status.md`
- Reference: `docs/teaching/beginner/00-course-map.md`

**Interfaces:**
- Consumes: 设计规格中的终局、状态模型、十四个阶段和 L34-L78 课程定义。
- Produces: 后续世界地图、学习状态、handoff 和 backlog 引用的唯一权威蓝图路径。

- [ ] **Step 1: 写入蓝图的治理和终局部分**

创建文档并包含：

```markdown
# Production Offline RAG Course Blueprint

## 1. 如何使用这份蓝图
## 2. 当前终局
## 3. 状态模型
## 4. 当前坐标
## 5. 完整课程路线
## 6. 最终生产验收
## 7. 蓝图变更规则
```

明确蓝图是课程定义唯一真相源；世界地图和学习状态只能引用，不能复制出另一份课程定义。

- [ ] **Step 2: 写入 L01-L33 的逐课事实**

逐课列出 L01-L33，每课至少包含：

```markdown
### L01 · Tokenizer Load Once

- 状态：`已学习`
- 依赖：无
- 实践产物：`internal/tokenizerdemo` 与对应 SOP
- 验收：Tokenizer 启动加载一次、重复计数稳定、缺失文件硬失败
```

课程名称和证据路径以现有 SOP、实现计划和 beginner course manifest 为准。L01-L23 使用
`已学习`，L24-L33 使用 `已实现待学习`，不得把机器验证等同于用户学习确认。

- [ ] **Step 3: 写入 L34-L78 的逐课定义**

每课至少包含：

```markdown
### L34 · Production Golden Dataset

- 状态：`已规划`
- 依赖：L33
- 实践产物：版本化 query/case 数据集、失败分类和基线报告
- 验收：数据集可重复运行，记录正例、负例、forbidden hit 与版本
```

严格采用设计规格的九个未来阶段和 45 个课名。逐课依赖使用最小必要前置，不全部笼统
写成“L01-L33”。

- [ ] **Step 4: 写入每阶段目标、产物、门禁和非目标**

十四个阶段都必须具有：

```markdown
目标：
依赖：
阶段实践产物：
阶段验收门禁：
非目标：
```

阶段一至五描述当前已取得的能力；阶段六至十四使用设计规格中的生产门禁。

- [ ] **Step 5: 验证权威逐课清单**

Run:

```bash
sed -nE 's/^### L([0-9]{2}) ·.*/\1/p' docs/teaching/00-course-blueprint.md |
awk '{count[$1]++} END {
  for (i=1;i<=78;i++) {
    key=sprintf("%02d",i)
    if (count[key] != 1) {
      printf "L%s count=%d\n", key, count[key]
      bad=1
    }
  }
  if (!bad) print "PASS: L01-L78 unique and continuous"
  exit bad
}'
```

Expected: `PASS: L01-L78 unique and continuous`

Run:

```bash
awk '
/^### L[0-9][0-9] ·/ {lesson=substr($2,2,2)+0}
/- 状态：`已学习`/ {if (lesson < 1 || lesson > 23) bad=1; learned++}
/- 状态：`已实现待学习`/ {if (lesson < 24 || lesson > 33) bad=1; implemented++}
/- 状态：`已规划`/ {if (lesson < 34 || lesson > 78) bad=1; planned++}
END {
  printf "learned=%d implemented=%d planned=%d\n", learned, implemented, planned
  if (bad || learned != 23 || implemented != 10 || planned != 45) exit 1
}' docs/teaching/00-course-blueprint.md
```

Expected: `learned=23 implemented=10 planned=45`

- [ ] **Step 6: Commit**

```bash
git add docs/teaching/00-course-blueprint.md
git diff --cached --check
git commit -m "docs: add production RAG course blueprint"
```

---

### Task 2: 重建当前世界地图

**Files:**
- Modify: `world/game-world-map.md`
- Reference: `docs/teaching/00-course-blueprint.md`
- Reference: `docs/teaching/00-learning-status.md`

**Interfaces:**
- Consumes: 权威蓝图的十四阶段、当前坐标和终局。
- Produces: 新模型或读者进入仓库时的简洁入口，不再包含过时项目路径和能力判断。

- [ ] **Step 1: 删除过时事实**

删除或改写：

- “真实 Qdrant、Ollama、embedding、memory_items、summary 尚未开放”。
- 只包含旧 gateway 六关的主线。
- `/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/...` 绝对路径。
- “如果未来接真实依赖”的假设语气。

- [ ] **Step 2: 写入当前世界状态**

地图必须区分：

- 已学习区域：L01-L23。
- 已实现待学习区域：L24-L33。
- 已规划区域：L34-L78。
- 当前教学坐标：L24。
- 当前实现边界：L33。
- 当前终点：L78。

- [ ] **Step 3: 写入十四阶段地图**

每阶段只保留一行目标和状态，详细内容链接：

```markdown
[完整课程蓝图](../docs/teaching/00-course-blueprint.md)
[当前学习状态](../docs/teaching/00-learning-status.md)
```

- [ ] **Step 4: 验证地图不含旧事实**

Run:

```bash
if rg -n '/Users/huangyanyu/go/src/chat-api|真实 `Qdrant` store|真实 `Ollama` generator|还没有完全开放' world/game-world-map.md; then
  exit 1
fi
rg -n 'L01-L23|L24-L33|L34-L78|L78|00-course-blueprint.md' world/game-world-map.md
```

Expected: 第一条无输出；第二条命中所有当前坐标和蓝图入口。

- [ ] **Step 5: Commit**

```bash
git add world/game-world-map.md
git diff --cached --check
git commit -m "docs: update RAG world map"
```

---

### Task 3: 对齐学习状态和接手入口

**Files:**
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-teaching-protocol.md`
- Modify: `docs/teaching/00-handoff-guide.md`

**Interfaces:**
- Consumes: `00-course-blueprint.md` 的状态模型和当前坐标。
- Produces: 后续教学和实现都能稳定找到 L24，不重复实现 L24-L33，也不提前实施 L34。

- [ ] **Step 1: 将学习状态收敛为动态快照**

重写 `00-learning-status.md`，保留：

- 总目标和权威蓝图入口。
- `已学习 23 / 已实现待学习 10 / 已规划 45`。
- L01-L23 已学习的五阶段摘要。
- L24-L33 已实现待学习的两个阶段摘要。
- 下一次教学从 L24 开始。
- 下一次新实现从 L34 开始，但必须先完成该批课程的设计、计划和证据基线。
- 学习确认与实现完成的状态转换规则。

删除重复的长 SOP 列表、已经实现却仍写作“下一章”的矛盾描述，以及“最后做真正升级”
但未指向具体课程的表达。

- [ ] **Step 2: 在教学协议中加入蓝图治理**

补充规则：

- 开课前读取权威蓝图和学习状态。
- 只有用户确认后才从 `已实现待学习` 改为 `已学习`。
- 新实践必须按蓝图顺序；调整顺序需要真实证据并更新蓝图。
- Backlog 不是正式课程，除非被蓝图吸收。
- 每五节完成阶段验收，不用单课测试冒充阶段完成。

- [ ] **Step 3: 重写 handoff 的最小读取顺序**

新模型最少读取：

1. `AI_INITIALIZATION.md`
2. `world/game-world-map.md`
3. `docs/teaching/00-course-blueprint.md`
4. `docs/teaching/00-learning-status.md`
5. `docs/teaching/00-teaching-protocol.md`

继续 L24-L28 时再读五份 Dual Retrieval SOP 和 batch log；继续 L29-L33 时再读五份
Document Ingestion SOP 和 batch log。提示词必须明确教学从 L24 开始，新实现从 L34
开始，不能重复实现已经机器验证的课程。

- [ ] **Step 4: 验证三份文档当前坐标一致**

Run:

```bash
rg -n '00-course-blueprint.md|L24|L33|L34|L78|已实现待学习' \
  docs/teaching/00-learning-status.md \
  docs/teaching/00-teaching-protocol.md \
  docs/teaching/00-handoff-guide.md
```

Expected: 三份文档均存在蓝图入口；状态与用途不矛盾。

- [ ] **Step 5: Commit**

```bash
git add \
  docs/teaching/00-learning-status.md \
  docs/teaching/00-teaching-protocol.md \
  docs/teaching/00-handoff-guide.md
git diff --cached --check
git commit -m "docs: align teaching navigation with blueprint"
```

---

### Task 4: 将优化 Backlog 映射到正式课程

**Files:**
- Modify: `docs/teaching/00-optimization-backlog.md`
- Reference: `docs/teaching/00-course-blueprint.md`

**Interfaces:**
- Consumes: L34-L78 的正式课程定义。
- Produces: 每个已被蓝图吸收的优化项都有正式去向，未被吸收的项目仍保持条件性候选。

- [ ] **Step 1: 添加 backlog 使用状态**

在使用规则中定义：

- `已纳入 Lxx`：已经成为正式课程的一部分。
- `保留候选`：尚未达到开课条件。
- `已由 Lxx 基线覆盖`：原始问题已经在现有课程解决，条目只保留历史背景。

- [ ] **Step 2: 映射 Tokenizer 与 Document Ingestion 项**

至少映射：

- 资产来源、版本绑定、多模型注册、runtime 对照、容量：L54-L58。
- 完整模板计数和历史预算监控：L57、L63。
- Golden Dataset、更多 parser、oversized 性能：L34、L50-L53、L63。

已经由 L01-L12 或 L29-L33 覆盖的基线必须标注“已覆盖”，不能仍写成尚未实现。

- [ ] **Step 3: 映射 Summary、Memory 与 Dual Retrieval 项**

至少映射：

- Summary 并发、异步和质量评估：L43、L60-L63。
- Memory ontology、confidence、并发、outbox、漂移：L44-L48。
- Reranker、score calibration、动态 quota：L34-L38。
- 数据保留、删除、隐私和恢复：L64-L73。

保留原始“为什么需要”和“目标结果”，只修正已经过期的“何时再做”。

- [ ] **Step 4: 验证正式映射存在**

Run:

```bash
rg -n '已纳入 L(3[4-9]|4[0-9]|5[0-9]|6[0-9]|7[0-8])|已由 L(0[1-9]|[12][0-9]|3[0-3])' \
  docs/teaching/00-optimization-backlog.md
```

Expected: Tokenizer、Document、Summary、Memory 和 Dual Retrieval 五类均有映射；不得把
所有条目粗暴映射到同一节。

- [ ] **Step 5: Commit**

```bash
git add docs/teaching/00-optimization-backlog.md
git diff --cached --check
git commit -m "docs: map optimization backlog to course blueprint"
```

---

### Task 5: 完成全局一致性验证

**Files:**
- Verify: `docs/teaching/00-course-blueprint.md`
- Verify: `world/game-world-map.md`
- Verify: `docs/teaching/00-learning-status.md`
- Verify: `docs/teaching/00-teaching-protocol.md`
- Verify: `docs/teaching/00-handoff-guide.md`
- Verify: `docs/teaching/00-optimization-backlog.md`

**Interfaces:**
- Consumes: Tasks 1-4 的所有文档。
- Produces: 支撑“完整蓝图已建立”的编号、状态、链接、范围和人工自审证据。

- [ ] **Step 1: 重新运行编号和状态检查**

重复 Task 1 Step 5 的两条命令。

Expected:

```text
PASS: L01-L78 unique and continuous
learned=23 implemented=10 planned=45
```

- [ ] **Step 2: 检查本次文档中的本地 Markdown 链接**

使用只读 shell 循环提取六份文档中的相对 `.md` 链接，去掉锚点和行号后，相对当前文档
目录解析；任何目标不存在都输出 `BROKEN` 并返回非零。

Expected: `PASS: local Markdown links resolve`

- [ ] **Step 3: 检查过时事实和占位符**

Run:

```bash
if rg -n 'TODO|TBD|/Users/huangyanyu/go/src/chat-api|还没有完全开放这些区域|真实 `Qdrant` store|真实 `Ollama` generator' \
  docs/teaching/00-course-blueprint.md \
  world/game-world-map.md \
  docs/teaching/00-learning-status.md \
  docs/teaching/00-teaching-protocol.md \
  docs/teaching/00-handoff-guide.md \
  docs/teaching/00-optimization-backlog.md; then
  exit 1
fi
```

Expected: 无输出。

- [ ] **Step 4: 检查修改范围**

Run:

```bash
git status --short
git diff --name-only dbf4600..HEAD
```

Expected: 除用户原有三个未跟踪 AirDroid Business Markdown 外，只出现设计、计划和本计划
列出的相关 Markdown；不出现 Go、SQL、Shell、测试或 `docs/teaching/beginner/`。

- [ ] **Step 5: 人工自审**

逐项确认：

- 世界地图、蓝图、学习状态都写明当前教学 L24、实现 L33、终点 L78。
- 每个未来阶段都有目标、依赖、实践产物、门禁和非目标。
- L34-L78 每课都有状态、最小依赖、产物和验收。
- Backlog 不再与正式课程争夺真相源。
- “生产级”没有扩展到已排除范围。
- 所有完成状态都有现有代码、SOP、operation log 或学习记录支持。

- [ ] **Step 6: 如果验证产生修订，单独提交**

```bash
git add \
  docs/teaching/00-course-blueprint.md \
  world/game-world-map.md \
  docs/teaching/00-learning-status.md \
  docs/teaching/00-teaching-protocol.md \
  docs/teaching/00-handoff-guide.md \
  docs/teaching/00-optimization-backlog.md
git diff --cached --check
git commit -m "docs: verify production RAG course blueprint"
```

只有存在实际修订才创建此提交；无修订时不创建空提交。
