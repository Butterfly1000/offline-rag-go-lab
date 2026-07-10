# Batch Operation And Impact Log

主题：2026-07-11 连续三个 tokenizer/context 教学小节的低风险操作记录

基线 commit：`39ea7ff`

## 授权边界

本批次允许：

- 读取和修改当前仓库文件
- 运行格式化、单元测试、vet、build 和教学命令
- 只读调用本机 Ollama `/api/show`
- 在当前仓库创建独立 commit

本批次禁止：

- `git push`
- 修改数据库、Qdrant 或 Ollama 模型文件
- 写入其他项目
- 破坏性 Git 或文件操作
- 使用、修改或输出凭据

## 小节 05：真实模板渲染

### 执行操作

1. 新增 `internal/promptbudget` 的模板渲染测试和实现
2. 新增 `cmd/prompt-budget-demo`
3. 运行目标测试和全量测试
4. 只读调用本机 Ollama `/api/show` 获取 `qwen:7b` 模板
5. 运行真实模板渲染命令
6. 提交前 review，并创建独立 commit

### 状态影响

- 仓库：新增教学代码、测试、SOP、设计与计划文档
- Ollama：仅读取模型元数据，没有发起生成，没有修改模型
- 数据库/Qdrant：没有访问
- Git：只创建本地 commit，没有 push
- 外部系统：没有访问

### 风险分析

- `text/template` 只执行 Ollama 返回的本地模板，不执行 shell 命令
- 本节没有把渲染结果接入 `/chat`，不会改变现有聊天行为
- 当前实现验证本机简单 Qwen 模板；复杂自定义模板函数后续按需优化

### Review 与验证证据

- RED：`Render` 不存在时三个目标测试编译失败
- GREEN：`go test ./internal/promptbudget` 通过
- 回归：`go test ./...` 通过
- 静态检查：`go vet ./internal/promptbudget ./cmd/prompt-budget-demo` 通过
- 构建：`go build ./cmd/prompt-budget-demo` 通过
- 实践：真实读取 `qwen:7b` 并渲染 system/user/assistant prompt 成功
- Review：无 Critical/Important 问题

### 临时缓存说明

本小节早期测试沿用了 `/private/tmp/offline-rag-go-lab-gocache` 作为 Go 编译缓存。它只包含可删除的本项目编译产物，不包含业务数据，也没有修改其他项目。后续统一改用仓库内被忽略的 `.cache/go-build`。

## 小节 06：模板 token 开销

### 执行操作

1. 新增 token 对比测试和 `CompareTokens`
2. 扩展 `prompt-budget-demo`，加载项目内 tokenizer
3. 运行真实命令，计算正文与 rendered prompt
4. 运行目标测试、全量测试、vet 和全部命令构建
5. 提交前 review，并创建独立 commit

### 状态影响

- 仓库：新增 token 对比代码、测试和 SOP
- Ollama：仅读取 `/api/show`，没有生成或模型修改
- tokenizer：只读项目内 JSON 资产
- 数据库/Qdrant：没有访问
- Git：只创建本地 commit，没有 push

### 实际结果

- system 正文：`24` tokens
- user 正文：`15` tokens
- content-only：`39` tokens
- rendered prompt：`88` tokens
- template overhead：`49` tokens

### 问题与影响分析

第一次真实运行发现命令入口先调用 `CompareTokens`、后创建 `rendered`，导致编译失败。根因是代码插入顺序不符合数据依赖，而库测试没有编译 `cmd`。

修复只调整执行顺序，并新增批次门禁：

```bash
go build ./cmd/...
```

该问题只发生在未提交的教学命令中，没有进入 commit，没有影响现有服务或数据。

### Review 与验证证据

- RED：`CompareTokens` 和 `TokenComparison` 不存在时目标测试编译失败
- GREEN：`go test ./internal/promptbudget` 通过
- 回归：`go test ./...` 通过
- 并发检查：`go test -race ./internal/promptbudget` 通过
- 静态检查：`go vet ./internal/promptbudget ./cmd/prompt-budget-demo` 通过
- 命令构建：`go build ./cmd/...` 通过
- 实践：真实输出 `39` content-only、`88` rendered、`49` overhead
- Review：无 Critical/Important 问题
