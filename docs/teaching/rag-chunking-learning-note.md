# RAG Chunking 学习笔记

## 1. Chunking 为什么难

RAG 里的 chunking 不是简单地把文档切短。它真正要解决的是：

- 哪些内容应该放在同一个 chunk 里。
- 每个 chunk 是否能独立回答一类问题。
- chunk 是否足够小，可以进入 embedding 模型和最终 prompt。
- chunk 是否足够完整，不至于丢失上下文。

如果切得太粗，一个 chunk 里可能混入多个主题，embedding 后语义会变得模糊；如果切得太碎，单个 chunk 又可能缺少上下文，召回后也不好用。

所以 chunking 的目标不是“切得越小越好”，而是：

```text
尽量保持语义完整，同时控制在模型能处理的 token 范围内。
```

## 2. 为什么优先按标题分块

Markdown 标题是作者主动提供的结构信息。

例如：

```markdown
# 退款政策
## 申请条件
## 申请流程
## 不支持退款的情况
```

这些标题其实是在告诉系统：

```text
下面这些内容属于同一个语义主题。
```

所以标题不是装饰，而是人工标注过的结构边界。按标题分块，可以更自然地把接近的内容放在一起。

如果直接按行切，可能出现：

```text
chunk 1：申请条件第一半
chunk 2：申请条件第二半 + 申请流程开头
chunk 3：申请流程后半
```

这会让 chunk 语义混杂。按标题切更合理：

```text
chunk 1：申请条件
chunk 2：申请流程
chunk 3：不支持退款的情况
```

## 3. 什么叫关联性

这里的关联性不用理解得太玄。

一个实用定义是：

```text
一个 chunk 内的内容，应该共同回答同一类问题。
```

比如“申请条件”这个 chunk，应该能回答：

```text
什么情况下可以退款？
退款需要满足什么条件？
```

它不应该同时混入“发票开具”“头像设置”“账号注销”这些主题。

所以关联性可以理解为：

```text
chunk 里的句子是不是围绕同一个主题服务。
```

Markdown 标题能帮助系统找到这种主题边界。

## 4. 标题分块对 embedding 的价值

Embedding 会把整个 chunk 压缩成一个向量。

如果 chunk 的主题很纯：

```text
chunk = 退款申请条件
```

它的向量语义就更集中，更容易被“退款条件”这类问题召回。

如果 chunk 里混了很多主题：

```text
chunk = 退款 + 发票 + 头像设置 + 登录异常
```

它的向量会变成多个主题的平均表达。用户问退款时，它可能不够像退款；用户问发票时，它也不够像发票。

因此：

```text
chunk 越主题一致，embedding 越稳定，召回越准。
```

## 5. 标题不是唯一边界，size 也很重要

标题解决的是语义边界，size 解决的是模型容量边界。

理想顺序是：

```text
先按标题切
如果标题下内容仍然太长，再按小标题切
如果没有小标题，再按段落 / 列表 / 句子切
最后才按 size 硬切
```

所以不是“标题 vs size”，而是：

```text
标题优先，size 兜底。
```

如果只按 size 切，可能破坏一个完整主题；如果只按标题切，一个标题下面又可能有几千 token，后续无法进入模型上下文。

实战里更像“装箱”：

```text
把相关段落一个个放入 chunk
尽量接近目标 size
快超过上限时收口
再开下一个 chunk
```

## 6. size 是什么

这里的 size 通常指 token 数，不是字符数，也不是字数。

例如：

```text
chunk_size = 600
```

意思是：

```text
一个 chunk 目标最多约 600 tokens。
```

不同项目会选择不同大小：

```text
300 tokens：更细，检索更精准，但上下文可能不完整。
600 tokens：常见折中。
1000+ tokens：上下文更完整，但召回可能变粗，prompt 成本更高。
```

`chatinsight-index` 的默认配置是：

```text
size = 600
overlap = 20
```

## 7. 标题下内容超过 size 怎么办

假设有三个标题：

```text
标题1：700 tokens
标题2：500 tokens
标题3：400 tokens
size：600 tokens
```

标题2 和标题3 都没有超过 size，可以各自成为 chunk。

标题1 超过了 size，如果它下面还有小标题，就继续按小标题拆；如果没有小标题，就进入更细的切分逻辑：

```text
按段落切
段落太长则按列表 / 句子切
必要时才硬切
```

例如：

```text
# 标题1
段落A：200 tokens
段落B：250 tokens
段落C：250 tokens
```

可能变成：

```text
chunk 1：标题1 + 段落A + 段落B，约 450 tokens
chunk 2：标题1 meta + 段落C，约 250 tokens
```

如果某个段落本身就有 700 tokens，则会继续按句子拆。

重点是：即使标题1被拆成多个 chunk，也应该保留标题路径，例如：

```text
(meta: title=标题1)
正文...
```

这样每个拆出来的 chunk 仍然知道自己属于“标题1”这个主题。

## 8. overlap 是什么

overlap 是相邻 chunk 之间重复保留的一小段内容。

它不是为了让每个 chunk 都变成完整章节，而是为了给切割边界留一点缓冲。

例如原文是：

```text
1. 退款申请需要在购买后 7 天内提交。
2. 用户需要提供订单号。
3. 如果订单已经使用优惠券，退款金额会扣除优惠部分。
4. 审核通过后，款项会在 3 个工作日内原路退回。
5. 超过 7 天的订单一般不支持退款。
6. 特殊情况可以联系客服人工处理。
```

假设 size 只能放 3 句，overlap 是 1 句，可能切成：

```text
chunk 1:
1. 退款申请需要在购买后 7 天内提交。
2. 用户需要提供订单号。
3. 如果订单已经使用优惠券，退款金额会扣除优惠部分。

chunk 2:
3. 如果订单已经使用优惠券，退款金额会扣除优惠部分。
4. 审核通过后，款项会在 3 个工作日内原路退回。
5. 超过 7 天的订单一般不支持退款。

chunk 3:
5. 超过 7 天的订单一般不支持退款。
6. 特殊情况可以联系客服人工处理。
```

重复的句子就是 overlap。

它解决的是这种问题：

```text
上一块最后一句是原因
下一块第一句是结论
```

如果没有 overlap，两个 chunk 都可能缺上下文。

但 overlap 不能太大，否则会带来副作用：

```text
重复内容太多
向量库膨胀
检索结果重复
embedding 成本变高
```

可以记成一句话：

```text
结构边界决定 chunk 是否完整，overlap 只是切口处的保险带。
```

## 9. tiktoken 是什么

`tiktoken` 是 OpenAI 开源的 tokenizer 库，用来按 OpenAI 模型的分词规则计算 token。

`chatinsight-index` 中使用：

```python
tiktoken.encoding_for_model("gpt-3.5-turbo")
```

意思是：

```text
按 gpt-3.5-turbo 的 tokenizer 规则计算 token 数。
```

所以它不是按字符数估算，而是按 OpenAI 模型体系的 tokenizer 计算。

## 10. tiktoken 是否适用于所有模型

不适用。

不同模型可能使用不同 tokenizer：

```text
OpenAI GPT -> tiktoken
Qwen -> Qwen tokenizer
Llama -> Llama tokenizer
BGE -> BGE 对应 tokenizer
Claude -> Anthropic 自己的 token 规则
```

因此，同一段文本：

```text
用 tiktoken 算可能是 100 tokens
用 Qwen tokenizer 算可能是 90 或 120 tokens
用另一个模型可能又不同
```

差距不一定很大，但不能假设完全一致。

这很重要，因为 chunk size 绑定的是模型容量：

```text
embedding 模型一次能吃多长
生成模型最终上下文能塞多长
```

如果用错 tokenizer，可能出现：

```text
你以为没超
真实模型认为超了
请求失败或被截断
```

## 11. 为什么 chatinsight-index 用 tiktoken

因为 `chatinsight-index` 原本使用的是 OpenAI / Azure OpenAI embedding：

```text
OpenAIEmbeddings
text-embedding-ada-002 / ada-002
```

所以它用 `tiktoken` 来控制 chunk size 是合理的。

它的逻辑是：

```text
OpenAI embedding 模型
-> 用 OpenAI tokenizer 估算 token
-> 控制 chunk_size
```

这套是匹配的。

## 12. 如果我们用 bge-m3 / Qwen

如果我们后续用：

```text
embedding：bge-m3
生成模型：qwen:7b
```

就不应该盲目照搬 `tiktoken` 当绝对标准。

更严谨的方式是：

```text
分块给 bge-m3 embedding 用：参考 bge-m3 的输入限制和 tokenizer
最终 prompt 预算：用 Qwen tokenizer 控制
```

第一版 demo 可以先用近似方案，例如：

```text
tiktoken
简单字符估算
Qwen tokenizer.json
```

但要明确：

```text
这是近似 token counter，不是真实 bge-m3 / Qwen token counter。
```

生产里最好绑定目标模型对应的 tokenizer。

## 13. 对我们项目的启发

当前最值得吸收的是分块思路，而不是照搬 `.block` 文件或完整 Python 项目。

可以拆成三条原则：

```text
标题决定哪里切更合理。
size 决定每块多大能被模型吃下。
overlap 解决切口处上下文断裂。
```

我们自己的项目可以采用更清晰的接口：

```text
Go 主服务
-> 调 Python /split
-> Python 只负责 chunking
-> 返回 JSON chunks
-> Go 负责 embedding、Qdrant、MySQL
```

这样既能利用 Python 生态里的文档分块能力，又不把整个 RAG 主流程迁到 Python。

## 14. 最终记忆版

```text
按标题切，是利用作者已有结构，保证主题相关性。
按 size 控制，是为了不超过模型容量。
按段落 / 列表 / 句子切，是标题不够细时的降级策略。
overlap 是边界缓冲，不是主要分块手段。
token 计数必须尽量绑定真实模型，tiktoken 主要适合 OpenAI 模型。
```

再压缩成一句：

```text
好的 chunking 不是把文档切短，而是把同一主题、合适大小、上下文足够的内容放在一起。
```
