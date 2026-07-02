# 01 Chat Behavior

主题：`/chat` 请求如何完成一次问答

这节课按“行为”来理解项目，不是先背所有文件，而是先抓住一次 `POST /chat` 请求进入系统后，到底发生了什么。

---

## Part 1：入口数据和 `App` 的职责

先看请求和响应长什么样，代码来自 [internal/gateway/level1_world/types.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level1_world/types.go:35)。

```go
// ChatRequest 是 POST /chat 的请求体。
// 也就是外部调用这个项目时，要传进来的数据结构。
type ChatRequest struct {
    SessionID    string `json:"session_id"`    // 会话 ID：把同一轮或同一段对话串起来
    UserID       string `json:"user_id"`       // 用户 ID：标记是谁在提问
    Question     string `json:"question"`      // 用户真正的问题
    Model        string `json:"model"`         // 可选：本次请求想记录成什么模型名
    UseKnowledge bool   `json:"use_knowledge"` // true=走知识库检索，false=不检索
}

// ChatResponse 是 POST /chat 的成功响应。
// 也就是系统处理完后，返回给调用方的数据结构。
type ChatResponse struct {
    Answer          string           `json:"answer"`           // 最终回答文本
    UsedKnowledge   bool             `json:"used_knowledge"`   // 这次回答有没有真的用到知识库片段
    RetrievedChunks []RetrievedChunk `json:"retrieved_chunks"` // 本次实际参与回答的命中片段摘要
    LatencyMS       int64            `json:"latency_ms"`       // 耗时，单位毫秒
}
```

这段代码说明：`/chat` 不只是“输入问题，输出答案”，它还关心会话、用户、是否启用知识库、命中了哪些知识、耗时多久。

再看核心编排者 `App`，代码来自 [internal/gateway/level2_hq/app.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level2_hq/app.go:19)。

```go
// App 是 RAG 主编排器。
// 重点不是“自己实现所有能力”，而是“把不同能力组织起来”。
type App struct {
    config        world.Config       // 全局配置，比如 topK、日志目录、prompt 限制
    store         KnowledgeStore     // 知识库存储：负责存和搜知识
    retriever     Retriever          // 检索器：负责根据问题找相关 chunk
    compressor    Compressor         // 压缩器：负责把命中的 chunk 做筛选、限量、裁剪
    promptBuilder PromptBuilder      // prompt 构造器：负责把问题和知识拼成 prompt
    generator     AnswerGenerator    // 回答生成器：负责产出最终 answer
    logger        ConversationLogger // 日志器：负责记录这次对话
}
```

`App` 的核心职责不是“亲自实现检索、生成、日志”，而是把这些组件按顺序串起来，组成一次完整的 chat 行为。

再看 `App` 是怎么拿到这些组件的：

```go
func NewAppWithDeps(cfg world.Config, deps AppDeps) *App {
    // 确保目录存在；内部会调用 os.MkdirAll 递归创建目录。
    shared.MustMkdirAll(cfg.LogDir)
    shared.MustMkdirAll(cfg.DocDir)

    // 如果外部没传真实实现，就用默认实现兜底。
    storeImpl := deps.Store
    if storeImpl == nil {
        storeImpl = store.NewMemoryKnowledgeStore()
    }

    retriever := deps.Retriever
    if retriever == nil {
        retriever = retrieval.NewRetrievalService(storeImpl, cfg.RetrievalTopK, cfg.ScoreThreshold)
    }

    promptBuilder := deps.PromptBuilder
    if promptBuilder == nil {
        promptBuilder = boss.StaticPromptBuilder{}
    }

    compressor := deps.Compressor
    if compressor == nil {
        compressor = compression.SimpleCompressor{}
    }

    generator := deps.Generator
    if generator == nil {
        generator = boss.MockAnswerGenerator{}
    }

    logger := deps.Logger
    if logger == nil {
        logger = boss.NewJSONLConversationLogger(cfg.LogDir, cfg.ChatModel, cfg.EmbeddingModel)
    }

    return &App{
        config:        cfg,
        store:         storeImpl,
        retriever:     retriever,
        compressor:    compressor,
        promptBuilder: promptBuilder,
        generator:     generator,
        logger:        logger,
    }
}
```

这里最重要的理解：

- `nil` 表示这个依赖没有传进来
- 没传就用默认实现
- 所以这个项目现在虽然是教学版，但未来可以把默认组件换成真实组件

### Part 1 总概述

这一部分先建立两个基础认识：

1. `/chat` 是一个完整行为，不只是字符串输入输出。
2. `App` 是主流程编排器，负责把“检索、压缩、生成、日志”这些组件串起来。

### Part 1 重点

- 先看 `ChatRequest` 和 `ChatResponse`，理解一次 chat 的输入输出。
- 再看 `App` 字段，理解系统能力被拆成了哪些组件。
- 再看 `NewAppWithDeps`，理解为什么这个项目现在能跑 mock，未来也方便接真实实现。

---

## Part 2：`Chat()` 主流程怎么串起来

代码来自 [internal/gateway/level2_hq/app.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level2_hq/app.go:191)。

```go
// Chat 完整问答链路：校验 -> 可选检索 -> 压缩 -> 生成 -> 写日志
func (a *App) Chat(req world.ChatRequest) (world.ChatResponse, error) {
    // strings.TrimSpace 会去掉首尾空格、换行、制表符。
    // 这里用于防止传入 "   " 这种看似非空、实际上无意义的值。
    if strings.TrimSpace(req.SessionID) == "" {
        return world.ChatResponse{}, errors.New("session_id is required")
    }
    if strings.TrimSpace(req.UserID) == "" {
        return world.ChatResponse{}, errors.New("user_id is required")
    }
    if strings.TrimSpace(req.Question) == "" {
        return world.ChatResponse{}, errors.New("question is required")
    }

    // 记录开始时间，后面用于计算本次 chat 耗时。
    start := time.Now()

    // 先准备一个空的命中列表。
    hits := []world.RetrievalHit{}

    // 只有启用知识库时才执行检索。
    if req.UseKnowledge {
        // Retrieve 返回结构体；这里主流程只取 Hits。
        hits = a.retriever.Retrieve(req.Question).Hits
    }

    // 检索结果不直接送去生成，要先压缩，控制数量和文本长度。
    selected := a.compressor.Compress(
        hits,
        a.config.PromptMaxChunks,
        a.config.PromptMaxChars,
    )

    // 先组装响应骨架。
    resp := world.ChatResponse{
        Answer: a.generator.Generate(req.Question, selected, a.config.PromptMaxChars),

        // len(selected) 是切片长度。
        // 只要最终进入生成阶段的 chunk 数量 > 0，就说明用到了知识。
        UsedKnowledge: len(selected) > 0,

        // make([]T, 0, n) 创建长度 0、容量 n 的切片。
        // 这样后面 append 时通常不会频繁扩容。
        RetrievedChunks: make([]world.RetrievedChunk, 0, len(selected)),

        // time.Since(start) 表示从 start 到现在经过了多久。
        // Milliseconds() 再把这个时长转成毫秒整数。
        LatencyMS: time.Since(start).Milliseconds(),
    }

    // 把内部命中结果转换成对外暴露的摘要结构。
    for _, hit := range selected {
        resp.RetrievedChunks = append(resp.RetrievedChunks, world.RetrievedChunk{
            DocumentID: hit.DocumentID,
            ChunkID:    hit.ChunkID,
            Title:      hit.Title,
            SourceRef:  hit.SourceRef,

            // Round4 是项目工具函数，用来把分数保留 4 位小数。
            Score: shared.Round4(hit.Score),
        })
    }

    // 最后写日志；如果日志失败，这次 Chat 整体也算失败。
    if err := a.logger.AppendLog(req, resp, selected); err != nil {
        return world.ChatResponse{}, err
    }

    return resp, nil
}
```

这段代码最值得记的不是细节，而是顺序。

一次 `/chat` 严格按这 5 步走：

1. 校验请求
2. 按需检索
3. 压缩结果
4. 生成回答
5. 写日志

你可以把它理解成一条流水线：

`请求进来 -> 先检查能不能处理 -> 找资料 -> 缩资料 -> 出答案 -> 记档案`

### Part 2 总概述

`Chat()` 是整个项目里一次问答行为的主骨架，顺序非常明确：

`校验 -> 检索 -> 压缩 -> 生成 -> 日志`

### Part 2 重点

- `strings.TrimSpace(...) == ""`：判断“看似有值、其实只是空白”的输入。
- `UseKnowledge`：决定这次是否启用知识检索。
- `Compress(...)`：说明“检索结果”不等于“最终送去生成的上下文”。
- `len(selected) > 0`：用于判断本次是否真的使用了知识。
- `time.Since(start).Milliseconds()`：记录本次请求耗时。

---

## Part 3：回答怎么生成，以及 `UsedKnowledge` 为什么不等于 `UseKnowledge`

先看生成器代码，来自 [internal/gateway/boss/generator_mock.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/boss/generator_mock.go:10)。

```go
// MockAnswerGenerator 模拟 LLM：有命中则拼接知识片段，无命中则返回固定兜底话术。
type MockAnswerGenerator struct{}

// Generate 实现 hq.AnswerGenerator；不接真实 Ollama 时用于跑通闭环。
func (g MockAnswerGenerator) Generate(question string, hits []world.RetrievalHit, maxChars int) string {
    // len(hits) == 0 表示这次没有任何知识片段进入生成阶段。
    if len(hits) == 0 {
        // 没命中知识库时，直接返回一个兜底回答。
        return "当前未命中知识库，我只能基于通用能力回答：" + question
    }

    // make([]string, 0, len(hits))：
    // 创建一个字符串切片，用来收集每个命中 chunk 的文本。
    parts := make([]string, 0, len(hits))

    for _, hit := range hits {
        // shared.Truncate 用来截断文本长度，避免单个 chunk 过长。
        parts = append(parts, shared.Truncate(hit.Text, maxChars))
    }

    // strings.Join(parts, " ")：
    // 用空格把多个字符串拼接成一个字符串。
    return "根据知识库，" + strings.Join(parts, " ")
}
```

这里要先抓住一个事实：

当前项目还没有接真实大模型，所以 `Generate(...)` 不是“模型推理”，而是一个教学版 mock 生成器。

它的行为很直接：

- 没命中知识：返回固定兜底句式
- 命中知识：把命中的 chunk 文本拼起来当作回答

再回到 `Chat()` 里这两行：

```go
resp := world.ChatResponse{
    Answer:        a.generator.Generate(req.Question, selected, a.config.PromptMaxChars),
    UsedKnowledge: len(selected) > 0,
}
```

这里最容易误解的一点是：`UsedKnowledge` 不是 `req.UseKnowledge`。

原因：

- `UseKnowledge` 表示：这次“要不要尝试”用知识库
- `UsedKnowledge` 表示：这次“最终有没有真的用上”知识

比如：

1. `UseKnowledge = false`
   不会检索，`UsedKnowledge = false`

2. `UseKnowledge = true`，但没检索到结果
   虽然尝试用了知识库，但没有任何 chunk 参与生成
   所以 `UsedKnowledge = false`

3. `UseKnowledge = true`，而且检索后压缩出了 `selected`
   这时才是 `UsedKnowledge = true`

所以：

```go
UsedKnowledge: len(selected) > 0
```

这个判断看的是“最终有没有知识片段真的进入回答生成阶段”，而不是“是否打开了知识库开关”。

### Part 3 总概述

- 当前项目的回答生成器是 mock 版，不是真实 LLM。
- 它的目标是先把 RAG 主链路跑通。
- `UsedKnowledge` 反映的是“实际是否用到知识”，不是“是否尝试启用知识”。

### Part 3 重点

- `Generate(...)` 当前是教学版拼接逻辑，不是推理模型。
- `len(hits) == 0` 时走兜底回答。
- `strings.Join(parts, " ")` 用于把多个命中片段拼成一个字符串。
- `UseKnowledge` 是“要不要尝试检索”。
- `UsedKnowledge` 是“最终有没有知识进入生成”。

---

## Part 4：为什么日志放在最后，以及这个顺序说明了什么

先看日志器代码，来自 [internal/gateway/boss/logging.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/boss/logging.go:13)。

```go
// JSONLConversationLogger 每次 Chat 往按日期命名的 .jsonl 文件追加一行 JSON。
type JSONLConversationLogger struct {
    logDir         string // 日志目录
    chatModel      string // 默认聊天模型名
    embeddingModel string // 默认 embedding 模型名
}

// AppendLog 写入一条对话记录；同一自然日共用一个文件。
func (l JSONLConversationLogger) AppendLog(
    req world.ChatRequest,
    resp world.ChatResponse,
    hits []world.RetrievalHit,
) error {
    // map[string]any 适合先动态组装一个 JSON 结构。
    record := map[string]any{
        "session_id":          req.SessionID,
        "user_id":             req.UserID,
        "question":            req.Question,
        "answer":              resp.Answer,
        "used_knowledge":      resp.UsedKnowledge,
        "retrieved_chunk_ids": shared.ChunkIDs(hits),

        // shared.ValueOrDefault(a, b)：a 非空就用 a，否则用 b。
        "chat_model":      shared.ValueOrDefault(req.Model, l.chatModel),
        "embedding_model": l.embeddingModel,

        // RFC3339 是一种常见的标准时间字符串格式。
        "created_at": time.Now().Format(time.RFC3339),
    }

    // json.Marshal 把 Go 数据转成 JSON 字节。
    raw, err := json.Marshal(record)
    if err != nil {
        return err
    }

    // Go 时间格式使用参考时间 2006-01-02 15:04:05。
    filename := time.Now().Format("2006-01-02") + ".jsonl"
    path := filepath.Join(l.logDir, filename)

    // O_CREATE：不存在就创建
    // O_WRONLY：只写
    // O_APPEND：追加到文件末尾
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
    if err != nil {
        return err
    }
    defer f.Close() // 函数退出前确保关闭文件

    // JSONL：每行一个 JSON 对象。
    _, err = f.Write(append(raw, '\n'))
    return err
}
```

日志记录的不是随便一句文本，而是一整次 chat 的关键结果：

- 谁问的
- 问了什么
- 回了什么
- 有没有用知识
- 用了哪些 chunk
- 用的什么模型
- 什么时候发生的

所以日志在这个项目里更像“行为留痕”，而不只是调试辅助。

再回到 `Chat()` 里的顺序：

```go
if err := a.logger.AppendLog(req, resp, selected); err != nil {
    return world.ChatResponse{}, err
}

return resp, nil
```

日志必须放在最后，因为它记录的是“这次真实发生了什么”，不是“准备要发生什么”。

只有前面的流程都完成了，日志里这些字段才完整：

- `resp.Answer`
- `resp.UsedKnowledge`
- `selected` 对应的 chunk id

这里还有一个工程上的选择：

如果日志写失败，这次 `Chat()` 整体也算失败。

这说明这个项目把“生成回答”和“成功留痕”视为一个完整行为，而不是两个彼此独立的小步骤。

### Part 4 总概述

- 日志记录的是一次 chat 的真实结果，不是意图。
- 所以日志必须放在回答生成之后。
- 当前实现把“回答成功”和“日志成功”绑定在一起，强调行为完整性和可追踪性。

### Part 4 重点

- `map[string]any`：适合动态组装日志结构。
- `json.Marshal(...)`：把 Go 数据转成 JSON。
- `os.OpenFile(..., os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)`：以追加方式写文件。
- `defer f.Close()`：确保函数退出前关闭文件。
- 日志放最后，是因为它依赖前面已经产出的真实结果。
- 日志失败会让整个 `Chat()` 失败，说明“留痕”被视为完整行为的一部分。
