# 02 Ingest Behavior

主题：`/ingest` 请求如何把一篇文档导入系统

这节课的目标不是先讲所有细节，而是先抓住 ingest 这个行为的主干：一篇文档进入系统后，到底发生了什么。

---

## Part 1：`/ingest` 到底做了哪 3 件事

先看主函数，代码来自 [internal/gateway/level2_hq/app.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level2_hq/app.go:133)。

```go
// IngestText 导入知识：切块 -> 写入 store -> 原文落盘
func (a *App) IngestText(req world.IngestRequest) (world.IngestResponse, error) {
    // 第 1 步：把整篇文档切成多个 chunk。
    chunks, err := chunking.BuildChunks(req)
    if err != nil {
        return world.IngestResponse{}, err
    }

    // 第 2 步：把切好的 chunk 写进知识库。
    // 当前默认是内存 store。
    a.store.Upsert(chunks)

    // 第 3 步：把原始全文保存到磁盘。
    docPath := filepath.Join(a.config.DocDir, req.DocumentID+".txt")
    if err := os.WriteFile(docPath, []byte(req.Text), 0o644); err != nil {
        return world.IngestResponse{}, err
    }

    // 最后返回导入结果摘要。
    return world.IngestResponse{
        DocumentID:     req.DocumentID,
        ChunkCount:     len(chunks),
        EmbeddingModel: a.config.EmbeddingModel,
        Status:         "ok",
    }, nil
}
```

这段代码最值得记住的一句话：

**`/ingest` = 切块 + 入库 + 保存原文**

也就是说，这个项目定义的“导入知识”最小闭环是：

1. 收到文档
2. 把文档拆成知识块
3. 让这些知识块可被后续检索
4. 保留原始全文，方便追溯

再看请求结构，代码来自 [internal/gateway/level1_world/types.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level1_world/types.go:18)。

```go
type IngestRequest struct {
    DocumentID string   `json:"document_id"` // 文档唯一 ID
    Title      string   `json:"title"`       // 文档标题
    SourceRef  string   `json:"source_ref"`  // 来源引用
    Text       string   `json:"text"`        // 原始正文
    Tags       []string `json:"tags"`        // 标签
}
```

可以先把它理解成：

- `DocumentID`：这篇文档在系统里的主键
- `Text`：真正要被切块的正文
- `Title` / `SourceRef` / `Tags`：附加信息，后面会跟着 chunk 一起流动

### Part 1 总概述

`/ingest` 的核心行为不是“神秘建库”，而是把一篇文档转成可检索的 chunk，并保留原文。

### Part 1 重点

- `BuildChunks(req)`：把整篇文档拆成多个 chunk。
- `a.store.Upsert(chunks)`：把 chunk 放进知识库。
- `os.WriteFile(...)`：把原始全文单独落盘。
- `len(chunks)`：返回这次一共切出了多少块。

---

## Part 2：为什么要同时保存 chunk 和原文

先看 ingest 里的两行关键代码：

```go
a.store.Upsert(chunks)

docPath := filepath.Join(a.config.DocDir, req.DocumentID+".txt")
if err := os.WriteFile(docPath, []byte(req.Text), 0o644); err != nil {
    return world.IngestResponse{}, err
}
```

这两份数据看起来像重复保存，但用途其实完全不同。

第一份：chunk 进 store

```go
a.store.Upsert(chunks)
```

这份数据的用途是：

- 给后续检索用
- 给 `/chat` 用
- 给调试接口 `/debug/retrieval` 和 `/debug/prompt` 用

也就是说，chunk 是面向“被搜索、被命中、被拿来回答”的数据形态。

第二份：原文落盘

```go
os.WriteFile(docPath, []byte(req.Text), 0o644)
```

这份数据的用途是：

- 追溯导入时的原始内容
- 方便以后重新切块
- 方便对照“为什么切成了这些 chunk”
- 出问题时回看源文本，而不是只看被处理过的 chunk

所以这不是重复存储，而是两种不同职责：

- `store` 里的 chunk：为了后续“使用”
- 磁盘里的原文：为了后续“追溯”

再看这行路径拼接：

```go
docPath := filepath.Join(a.config.DocDir, req.DocumentID+".txt")
```

- `filepath.Join(...)`：按当前操作系统规则拼接路径，比手写字符串更稳
- `req.DocumentID + ".txt"`：当前项目是一篇文档对应一个原文文件

再看：

```go
[]byte(req.Text)
```

这里是把字符串转成字节切片，因为 `os.WriteFile` 写入的是字节。

权限参数：

```go
0o644
```

表示创建出来的文件默认是“拥有者可读可写，其他人只读”。

### Part 2 总概述

ingest 同时保存 chunk 和原文，不是重复，而是分别服务于“后续使用”和“后续追溯”。

### Part 2 重点

- `a.store.Upsert(chunks)`：保存“处理后的知识形态”。
- `os.WriteFile(..., []byte(req.Text), 0o644)`：保存“原始全文形态”。
- chunk 主要为了检索和回答。
- 原文主要为了追溯、复盘、重切。
- `filepath.Join(...)`：安全拼接路径。
- `[]byte(...)`：把字符串转成可写入文件的字节。
