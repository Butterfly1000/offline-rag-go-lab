# Context Hit Boundary SOP

主题：第 24 节，统一 Memory / Document 检索结果，同时保留数据隔离边界。

实现完成不代表用户已经学会。本节只有在用户教学确认后，才能更新为“已学会”。

## 1. 先实践效果

在项目根目录运行：

    go run ./cmd/context-hit-demo

预期输出：

    Valid memory: source=memory user_id=u-001
    Valid document: source=document knowledge_scope=offline-rag-course
    Rejected mixed ownership: memory hit must not carry knowledge_scope

这个 demo 不访问 MySQL、Ollama 或 Qdrant，也不会写入任何外部数据。

## 2. 这一节解决什么问题

长期记忆和知识文档最终都要进入 prompt，所以需要一个共同的 Hit 结构。

但“结构统一”不等于“来源消失”：

- memory 属于某个 user_id
- document 属于某个 knowledge_scope
- memory 不能伪装成 document
- document 不能携带个人 user_id

如果只保留 content 和 score，后续代码就无法证明结果属于谁，也无法正确展示来源。

## 3. 当前代码结构

核心类型：

- /offline-rag-go-lab/internal/contextretrieval/types.go
- /offline-rag-go-lab/internal/contextretrieval/validate.go
- /offline-rag-go-lab/internal/contextretrieval/errors.go

共同结构保留：

    Source
    ID
    Content
    Score
    UserID
    KnowledgeScope
    Kind
    Title
    SourceRef
    Metadata

Source 决定 ownership 规则。

Memory 必须满足：

    source = memory
    user_id 非空
    knowledge_scope 为空

Document 必须满足：

    source = document
    knowledge_scope 非空
    user_id 为空

## 4. 为什么 Qdrant 已过滤还要再校验

生产查询会在 Qdrant 请求里带 user_id 或 knowledge_scope filter。

返回后仍要执行 ValidateHit，原因包括：

- collection payload 可能被其他写入路径写错
- collection 或字段配置可能漂移
- 测试/替换实现可能忘记过滤
- 不能把外部系统返回值默认当成可信数据

因此安全边界是两层：

    请求前强制 filter
    -> 返回后重新验证 ownership

后面的第 25、26 节会把这两层接到真实 Qdrant。

## 5. Infrastructure 与 Integrity 的区别

Infrastructure failure 表示：

- Qdrant 暂时不可用
- HTTP 超时
- 网络连接失败

聊天服务可以记录 warning，并在没有这路上下文的情况下继续回答。

Integrity failure 表示：

- memory 返回了另一个 user_id
- document 返回了另一个 knowledge_scope
- payload 字段缺失或自相矛盾

这类错误不能静默降级，因为它说明数据隔离或索引契约已经失效。

代码用 SourceError 和 FailureKind 明确区分两种行为，而不是依靠错误字符串判断。

## 6. Metadata 为什么要复制

ValidateHit 会创建新的 Metadata map。

Go 的 map 是引用类型。如果直接返回原 map，调用方之后修改输入 map，会同时改变已经通过校验的结果。复制后，已验证结果不会被外部修改影响。

同时会裁剪 key/value 空白，并拒绝空 key 或规范化后重复的 key。

## 7. 测试

运行：

    go test ./internal/contextretrieval
    go test -race ./internal/contextretrieval

当前测试覆盖：

- 合法 memory/document
- 空 ID 和 content
- NaN/Inf score
- 未知 source
- 缺少或混合 ownership
- Metadata 深拷贝
- infrastructure/integrity 分类和 error unwrap

## 8. 当前实现与生产级差异

当前完成的是检索结果的领域边界，不是完整权限系统。

生产中 knowledge_scope 通常来自已经鉴权的租户、项目或知识库权限，不能完全信任客户端任意传值。后续接入 /chat 时仍要保证：

- user_id 来自可信身份
- knowledge_scope 已通过授权
- Qdrant filter 与返回校验同时存在
- 日志能够区分基础设施降级与数据隔离故障

## 9. 本节重点

1. 两类结果可以统一为 Hit，但必须保留 Source。
2. memory 用 user_id 隔离，document 用 knowledge_scope 隔离。
3. 外部检索结果返回后仍然要校验。
4. 基础设施错误可以降级，隔离错误必须停止。
5. 第 25 节会把 document ownership 接到真实 Qdrant。
