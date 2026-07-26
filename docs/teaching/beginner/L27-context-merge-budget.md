# L27：确定性合并两路召回并守住子预算

对应工程课：[Retrieval 小节 27](../context-merge-budget-sop.md)。本课决定两路 hit 如何安全、可解释地进入 `retrieved_context`。

## 1. 本课一句话

memory/document 各自在本路排序并使用 quota，不跨 collection 比 raw score；memory 先入、精确内容去重后，再按完整渲染块逐候选精确计 token。

## 2. 先用人话理解

两份成绩单来自不同考试，不能把 0.8 分直接排成总名次。先各自选名额，再把个人记忆优先放入盒子；每放一张卡都称整个盒子的重量，太重就跳过这张，继续试后面更小的卡。

## 3. 系统里有什么

- `Merge`：memory 与 document 各自按 score 降序、ID 升序打破同分，再各取 quota。
- 去重：仅 `trim + 压缩连续空白 + lower` 后内容完全相同才去重。
- `RenderContext`：固定 `<retrieved_context>`、安全 instruction 与 memory/document 标签。
- `SelectWithinTokenBudget`：对每个 tentative 完整块调用 tokenizer。

## 4. 一条完整链路

L26 的分路 hits → 每路内部排序/quota → memory 先加入，document 与已选内容精确归一化相同则丢弃 → HTML escape 所有不可信字段/内容并渲染 → 每加入一个候选都重新渲染完整 tentative block、计 token → 超预算记录 dropped ID 但继续尝试后面候选 → 得到 context 子预算内的 hits。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/contextretrieval -run 'Test(Merge|SelectWithinTokenBudget|RenderContext)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/context-merge-demo --config /absolute/path/to/your/recent-chat.env --context-token-budget 160
```

- test 是纯本地 fake counter，不访问 MySQL/Ollama/Qdrant，不写业务数据；只写 build cache，可安全重复。
- demo 读取由 `config/recent-chat.env.example` 派生配置中的本地 tokenizer，只做本地计算，不写业务状态；相同资产/输入可重复。缺 tokenizer 或计数失败会失败，不以字符数估算。

## 6. 结果怎么看

输出应显示 duplicate 被移除、memory 在 document 前、`Used context tokens` 不超过 160。每个 hit 的 raw score 仍保留在各自来源，但没有一条“memory 0.7 大于 document 0.8”的跨库结论。

## 7. 算一遍或走一遍

设 memory A 内容为 `Go`，document B 内容为 ` go `，document C 较小且不同。A 先进入，B 归一化后重复而被删。若完整 block 加 A 后为 90 token，加 C 后为 170（预算 160），C 被 skip；若随后 document D 加入后为 140，D 仍可入选。因为每次计的是完整块，标签、instruction 和 escape 都占 token。

## 8. 常见误解

“所有 hit 按 score 全局混排”错误，raw score 未校准。“超预算就停止循环”错误，后面小 hit 仍可能放得下。“只 escape content”错误，ID/title/source_ref 也是不可信数据。“prompt instruction 能替代权限控制”也错误，代码 ownership 仍是边界。

## 9. 当前实现与生产边界

已保证：来源独立 quota、稳定排序、memory 优先精确去重、非变异复制、安全 HTML escape、每次候选加入后的完整渲染计数。未保证：语义去重、reranker、动态 quota、跨 collection score calibration 或 score threshold；这些需评估数据，本课不假装 raw score 可比较。

## 10. 面试怎么说

我不跨 collection 混排 score，而是每路独立排序和 quota，再以 memory 优先做保守的精确去重。召回内容按不可信数据 escape 渲染；预算不是数字段正文，而是每加一个候选后重数整个最终块，超大候选跳过但不阻挡后续小候选。

## 11. 自检题与答案

问：raw score 0.9 的 document 必然胜 0.8 的 memory 吗？答：不作这种跨库比较。问：超预算候选后还会试后面的候选吗？答：会。问：token 计数对象是什么？答：完整 rendered retrieved_context。

## 12. 事实锚点

- 原课：[context-merge-budget-sop.md](../context-merge-budget-sop.md)
- Go 组件：`internal/contextretrieval/{merge,prompt,budget}.go`
- 演示：`cmd/context-merge-demo/main.go`
- 前置：[L26](L26-dual-retrieval.md)，后续：[L28](L28-chat-dual-retrieval.md)
