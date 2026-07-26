# L29：区分文档、构建版本和 chunk 身份

对应工程课：[Ingestion 小节 29](../document-identity-version-sop.md)。本课先固定生产 ingestion 的身份规则，再谈真实写入。

## 1. 本课一句话

逻辑 document、不可变 build version 和 stable chunk 是三种身份：build hash 绑定内容/解析/策略/目标 collection，chunk ID 不含 version 或全局行号。

## 2. 先用人话理解

一本书、这次印刷版和书中的一个段落不是同一件事。换纸张或排版策略会产生新印刷版；段落在不影响它身份的改动下应保持可识别，不能因为前面插一行字就全部改名。

## 3. 系统里有什么

- document identity：`knowledge_scope + document_id`；`source_ref` 是仓库相对定位，不是主键。
- build identity：`content_hash + parser_version + chunk_policy_hash + target_collection`。
- policy hash：format、parser version、max tokens、overlap 和 embedding model。
- chunk identity：scope、document、structure kind、heading path、规范化内容 hash、duplicate ordinal。

## 4. 一条完整链路

NormalizeDocument 统一换行/行尾空白 → 算 content hash → policy（含 embedding model）算 hash → MySQL 用 build identity 找/建 version → L30 产生 chunk → StableChunkID 用结构/内容/重复序号命名 → L31 写 manifest 和 vectors。version 与全局行号故意不进 chunk ID；内容、结构路径或重复位置变化才改变 chunk 身份。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-identity-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test -race ./internal/documentingest
```

- 都是纯本地：不需 MySQL/Ollama/Qdrant/私有配置，不写业务数据，只写 build cache，可重复。
- demo 对同一内容、改内容、移动 heading、duplicate ordinal 打印不同/相同 ID；测试还覆盖状态转移与 portable text。

## 6. 结果怎么看

相同 chunk 输入 ID 相同；文本改动、heading 移动或 duplicate ordinal 改变则 ID 改变。状态机只允许 `pending→building→ready→active`，`building→failed`、`failed→building`；`active→building` 被拒绝，发布版本不能原地重建。

## 7. 算一遍或走一遍

若 document 为 `(scope=course, id=intro)`，内容 hash 为 H1，policy hash为 P1，collection 为 C1，构建 identity 是 `(intro,H1,parser,P1,C1)`。仅把 embedding model 从 bge-m3 改为另一模型会改变 P1，必须新 build。一个正文相同但从 `A/Intro` 移到 `B/Intro` 的 chunk 因 heading path 改变 ID；在文首新增无关行不因“全局行号”改变它的 ID。

## 8. 常见误解

“版本号就是 chunk ID 的一部分”错误，会让无关重建全部变点。“绝对路径适合 document 主键”错误，机器根目录不稳定。“SHA256 是相似度”错误，它用于确定身份。“ready 就等于对外发布”错误，alias 发布在后续课程。

## 9. 当前实现与生产边界

MySQL schema 有 `document_sources`、`document_versions`、`document_chunk_manifests`，约束逻辑文档、build 和 manifest。当前本课不建表、不 embed、不写 Qdrant；它只固定身份/状态规则。它与 [L25](L25-document-qdrant.md) 双向处于同一 fixture → identity/version → ingestion → alias 关系链：L25 固定 fixture 验 scope，L29 反向说明生产 identity/version。

## 10. 面试怎么说

我把文档、版本、chunk 分开建模。build identity 还包含 embedding model 的 policy hash 与物理 collection，避免复用不兼容向量；stable chunk ID 只包含真正的结构/内容身份，不含 version 和全局行号，保证可重试而不被无关插入扰动。

## 11. 自检题与答案

问：改 embedding model 会复用旧 build 吗？答：不会，policy hash 改变。问：chunk ID 包含 version 吗？答：不包含。问：L25 和 L29 是孤立课程吗？答：不是，二者双向连接同一关系链。

## 12. 事实锚点

- 原课：[document-identity-version-sop.md](../document-identity-version-sop.md)
- Go 组件：`internal/documentingest/{types,identity,state}.go`
- schema：`sql/document_ingestion.sql`
- 演示：`cmd/document-identity-demo/main.go`
- 双向关系链：[L25](L25-document-qdrant.md) → L29 → [L30](L30-structured-chunking.md) → [L31](L31-idempotent-ingestion.md)
