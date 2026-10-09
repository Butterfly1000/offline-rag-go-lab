# RAG Theory Learning Checkpoint

主题：最小 RAG 闭环的理论热身与续聊断点

更新时间：2026-08-18

## 1. 这条学习线的定位

当前选择的是“入口 1”：先从 `internal/gateway` 所表达的最小 RAG 闭环理解理论，再决定
是否进入正式课程。

这是一条理论热身线，不改变课程蓝图中的 L01-L78 状态，也不把讨论过的概念自动标记为
正式课程“已学习”。

教学偏好：

- 每次只讲一个小段。
- 以生产环境中的真实概念为主，mock 只用来说明接口和流程。
- 学习者可以随时打断提问；问题解决后再继续。
- 暂时不要求逐文件阅读代码或运行完整外部服务。

## 2. 已讨论内容

已经讨论：

1. RAG 的两个核心动作：Retrieval（检索）和 Generation（生成）。
2. `Augmented` 的含义：检索外部知识，为模型补充相关、可信且数量合适的上下文。
3. 模型内部参数提供通用语言、知识模式和推理能力；检索内容提供当前问题所需的外部事实。
4. 文档需要先按标题、段落、句子和长度切成可独立检索的 Chunk。
5. 关键词/Sparse 检索与语义向量/Dense 检索的基本区别，以及生产环境常见的混合检索。
6. 中文搜索分词的三类方式：词语切分、字符 N-gram、模型配套的子词 Tokenizer。
7. Chunk、Token ID 和 Embedding Vector 是三个不同层次：
   - Chunk 是人类可读的知识片段。
   - Token ID 是 Tokenizer 词表中的编号。
   - Vector 是 Embedding 模型生成的固定长度语义表示。
8. 文档由 Chunker 切块；短问题通常不切 Chunk。Chunk 和问题分别通过同一个 Embedding
   模型体系向量化。
9. Qdrant Point 通常同时保存不可直接解释的向量和人类可读的 Payload，如文本、标题、
   文档 ID、Chunk ID、版本和 knowledge scope。
10. 生成模型通常可以共享检索出的 Chunk 文本；已有向量则绑定 Embedding 模型、版本、
    维度、距离算法和预处理方式。更换 Embedding 模型通常需要重新向量化和建索引。

## 3. 需要在继续时先确认的概念

上一次在解释下列区别后转去讨论会话续接，尚未做学习者确认：

```text
文档 -> Chunker -> 可读 Chunk
                  -> Embedding Tokenizer
                  -> Token ID
                  -> Embedding 模型
                  -> Chunk Vector
                  -> Qdrant(Vector + Payload)

问题 -> 同一 Embedding 模型体系 -> Query Vector
     -> Qdrant 相似度搜索
     -> 返回可读 Chunk
     -> 生成模型回答
```

继续时先允许学习者对这条链路补问，不要假定已经完全理解。

## 4. 下一教学点

下一小段：Qdrant 如何判断 Query Vector 与 Chunk Vector 是否接近。

建议只覆盖：

1. 为什么单个向量维度（例如 `0.02`）没有独立业务含义。
2. 为什么要比较整个向量。
3. 余弦相似度的直觉：比较方向，而不是逐项寻找可读标签。
4. 相似度分数只是候选排序信号，不自动等于“答案正确概率”。

讲完停下来确认，再考虑点积、欧氏距离、Top-K、阈值和 Qdrant Point 的真实结构。

## 5. 新聊天续接提示词

```text
请读取 docs/teaching/00-rag-theory-learning-checkpoint.md，继续我的最小 RAG 闭环理论学习。
每次只讲一个小段，允许我随时提问。先确认 Chunk、Token ID、Embedding Vector 这条链路
还有没有问题，然后从 Qdrant 的向量相似度开始。
```
