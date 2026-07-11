# Session Summary Batch Operation And Impact Log

主题：第 14 到第 18 节 Session Summary 真实落地记录

设计/计划基线：`da5cc54`

## 授权边界

- 允许修改当前仓库、运行测试和教学命令、创建本地 commit
- 禁止 push、Qdrant/long-term memory 实现和破坏性操作
- 数据库 schema、凭据和外部状态变更前停止询问

## 第 14 节：驱逐前缀选择与 token 统计

### RED

- selector 测试因 `SourceMessage`、`SelectPrefix` 不存在而编译失败
- formatted counter 测试因 `NewFormattedMessageCounter` 不存在而编译失败

### 状态影响

- 新增纯 Go selector/counter、测试、命令和 SOP
- tokenizer 实践只读本地资产
- 未访问 MySQL、Ollama、Qdrant
- 未修改 `/chat`

### 验证与 Review

- 标准实践：未摘要 IDs `21..26`，驱逐 `21..24`，token `129/86`，水位 `24`
- ID 空洞实践：驱逐 `21,23`，token `43`，水位 `23`
- Review 发现 raw tokenizer 为 nil 时会 panic
- 修复证据：新增测试先复现 panic，再改为明确 `text token counter is required` 错误
- `go test ./...` 通过
- `go test -race ./internal/sessionsummary` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现其他 Critical 或 Important 问题
