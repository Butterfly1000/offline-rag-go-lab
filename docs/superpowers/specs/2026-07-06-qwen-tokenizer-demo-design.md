# Qwen Tokenizer Demo Design

日期：2026-07-06

主题：为 `qwen:7b` 增加一个本地 tokenizer 计数 demo，并提供可执行 SOP。

---

## 1. 目标

这次实现不直接改 `recent-chat` 主链路，而是先完成一个最小、可运行的本地 tokenizer demo。

完成后应满足：

1. 能基于 `qwen:7b` 对应 tokenizer 资产，在本地计算一段中文文本的 token 数
2. 能演示一组简单 chat messages 的 token 计数
3. 有一份可执行 SOP，说明 tokenizer 资产从哪里获取、放哪里、如何运行 demo

---

## 2. 范围

### 本次 in scope

- 新增一个独立命令，例如 `cmd/tokenizer-demo`
- 在 Go 里本地加载 tokenizer 资产
- 支持：
  - 单段文本 token 计数
  - 多条 messages token 计数演示
- 新增一份教学/SOP 文档

### 本次 out of scope

- 不直接改 `internal/recentchat` 的主流程
- 不在这一步实现 token-budget recent window
- 不在这一步处理 session summary
- 不追求一上来就完美覆盖所有 chat template 细节

---

## 3. 方案

采用 A1 方案：

- 在 Go 里直接加载与 `qwen:7b` 匹配的 tokenizer 资产
- 不依赖外部 Python 脚本或旁路服务

推荐原因：

1. 更符合“本地精确 token 计数”的学习目标
2. 后续最容易复用到 `recent-chat`
3. 更适合作为教学示例

---

## 4. 预计产物

### 代码

- `cmd/tokenizer-demo/main.go`
- 可能新增一个小型 tokenizer 封装目录，例如：
  - `internal/tokenizerdemo`
  - 或 `internal/tokenizer`

### 文档

- `docs/teaching/` 下新增一份 tokenizer demo/SOP 文档

### 资产约定

- tokenizer 文件不直接提交大文件
- 通过 SOP 指导用户自行下载并放到本地约定目录

---

## 5. 运行方式

第一版 demo 采用“写死输入”的方式，减少变量：

1. 一段固定中文文本
2. 一组固定 messages

运行 demo 后，输出至少包括：

1. 文本原文
2. token 总数
3. messages 的逐条 token 计数
4. messages 总 token 数

---

## 6. 验证

完成后用下面证据判断是否达成：

1. `go run ./cmd/tokenizer-demo` 能成功运行
2. demo 能输出文本 token 数和 messages token 数
3. SOP 能指导用户完成 tokenizer 资产放置与 demo 运行

---

## 7. 下一步

A 完成后，再进入 B：

- 把同一个 tokenizer 组件接到 `recent-chat`
- 再实现 token-budget recent window
