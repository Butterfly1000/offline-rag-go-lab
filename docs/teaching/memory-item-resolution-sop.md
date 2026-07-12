# Memory Item Resolution SOP

主题：第 21 节，用确定性 Go 规则决定 INSERT、UPDATE、NOOP 和 FORGET

## 1. 这一节解决什么问题

第 20 节的模型只负责提出 candidate。数据库动作不能继续交给模型决定：

```text
validated candidate + current item
-> deterministic resolver
-> INSERT / UPDATE / NOOP / FORGET
```

同样的输入必须永远得到同样的 action 和 version。这一层不调用 Ollama、MySQL 或 Qdrant。

## 2. Part 1：四种 action

代码：

- [resolve.go](/offline-rag-go-lab/internal/memoryitem/resolve.go:1)

决策表：

| Current | Candidate | Action | Version |
| --- | --- | --- | --- |
| 不存在 | upsert | INSERT | 从 1 开始 |
| active 且值等价 | upsert | NOOP | 不变 |
| active 且值改变 | upsert | UPDATE | +1 |
| forgotten | upsert | UPDATE/恢复 | +1 |
| active | forget | FORGET | +1 |
| 不存在或已 forgotten | forget | NOOP | 不创建/不变 |

“本轮没有 candidate”不在表里，因为它什么也不做，绝不能推导出 FORGET。

## 3. Part 2：Resolve 具体行为

入口：

```go
func Resolve(current *Item, candidate Candidate) (Decision, error)
```

输出：

```go
type Decision struct {
    Action    Action
    Current   *Item
    Next      Item
    Candidate Candidate
    Reason    string
}
```

Resolver 会再次规范化 operation、kind、key 和 value。已有 item 必须满足：

- ID 为正数
- version 为正数
- status 为 active/forgotten
- kind/key 与 candidate 完全匹配

这些检查可以防止 store 把 A 事实的 candidate 错更新到 B item。

## 4. Part 3：值如何判断“等价”

当前规则会：

1. 去除首尾空白
2. 把连续空白压成一个空格
3. 比较时忽略大小写

例如：

```text
"Go language"
"  go   LANGUAGE "
```

结果是 NOOP，数据库不需要增加 version。但存储值仍保留原来的 `Go language`，不会因为格式噪音反复覆盖。

这只是确定性文本等价，不是语义等价。`Go` 与 `Golang` 当前仍可能是 UPDATE；跨表达方式的语义归一化属于 ontology 优化。

## 5. Part 4：FORGET 的状态语义

FORGET 把 status 改成 `forgotten`，增加 version，但当前保留 item value 作为审计状态。后续行为是：

- active 查询排除它
- Qdrant 删除对应 point
- 再次 forget 为 NOOP
- 新 upsert 可以恢复为 active

这里的“忘记”表示不再召回，不等于隐私法规中的物理擦除。真正的数据擦除还需要处理 evidence、消息原文、备份和审计策略，不在本节冒充完成。

## 6. Part 5：Batch 为什么要稳定排序

`ResolveBatch` 按 candidate 的最小 `source_message_id` 升序处理；来源相同时保持模型原始顺序。这样结果不依赖 Go map 的随机遍历顺序。

同一个 batch 里：

```text
source 20: implementation_language=Go
source 30: implementation_language=Rust
```

会得到 INSERT version 1，再 UPDATE version 2。

batch 在建立 identity 前先规范化 key，避免：

```text
Implementation Language
implementation_language
```

被错误处理为两个 INSERT。

## 7. Part 6：运行 demo

执行：

```bash
go run ./cmd/memory-resolve-demo
```

代码：

- [main.go](/offline-rag-go-lab/cmd/memory-resolve-demo/main.go:1)

预期输出：

```text
insert version=1 status=active value="Go" reason=new_memory_item
noop   version=1 status=active value="Go" reason=equivalent_active_value
update version=2 status=active value="Rust" reason=replace_changed_value
forget version=3 status=forgotten value="Rust" reason=explicit_forget_request
update version=4 status=active value="Go" reason=restore_forgotten_item
```

运行测试：

```bash
go test ./internal/memoryitem -run TestResolve
```

## 8. 当前实现和生产级差异

当前已经具备：

- 决策结果确定
- version 规则确定
- missing forget 安全 NOOP
- forgotten 恢复
- batch 稳定顺序
- 相同 key 的连续更新

下一节 MySQL store 还需要增加：

- transaction
- `SELECT ... FOR UPDATE`
- version conflict
- item 与 evidence 原子提交
- `(user_id, kind, key)` 唯一键

resolver 不负责数据库并发；它只把“当前状态 + 候选应该变成什么”讲清楚。

## 9. 总结与重点

1. 模型负责提出候选，Go resolver 负责数据库动作。
2. NOOP 不加 version，状态变化才加 version。
3. forget 缺失 item 时不创建 tombstone，已有 forgotten 再 forget 也是 NOOP。
4. FORGET 是停止召回，不等于物理擦除。
5. batch 必须先规范化 identity，再按来源稳定处理，不能依赖 map 顺序。
