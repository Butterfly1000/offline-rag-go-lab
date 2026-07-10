# Three Token Context Lessons Design

日期：2026-07-11

## 目标

连续实现三个教学小节，把“模型模板存在”推进到“能渲染、能计数、能做预算”，但不修改现有 `/chat` 行为。

## 三个小节

### 小节 05：真实模板渲染

- 从 Ollama `/api/show` 读取当前模型 template
- 使用 Go `text/template` 填入 `.System` 和 `.Prompt`
- 打印渲染后的真实 prompt

### 小节 06：模板 token 开销

- 使用现有本地 tokenizer 计算 system/user 正文 token
- 计算渲染后 prompt token
- 输出模板包装增加的 token

### 小节 07：上下文预算规划

- 使用 Ollama 返回的 `context_length`
- 从总容量中扣除当前渲染 prompt 和输出预留
- 输出可分配给 recent history 的 token 预算

## 边界

- 不修改 MySQL、Qdrant、Ollama 模型文件或 `/chat` 现有行为
- 不实现 Ollama 多轮 `.Prompt` 的严格内部对照
- 不证明当前本地 tokenizer 与 Ollama 模型完全匹配
- 不 push
- 仓库外写入、破坏性操作、凭据和数据库变更必须停止询问

## 验证

每小节必须满足：

1. 新行为有 RED/GREEN 单元测试
2. 有一条真实可运行命令
3. 有独立 SOP
4. 提交前完成 review
5. 创建一个独立 commit

批次完成还必须满足：

- `go test ./...` 通过
- 从基线 `39ea7ff` 起恰好新增 3 个相关 commit
- `git status --short` 为空
- 没有执行 `git push`
