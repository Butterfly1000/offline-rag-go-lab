# Memory Item Store SOP

主题：第 22 节，用 MySQL 事务保存 Memory Item 与来源证据

## 1. 这一节解决什么问题

第 21 节只得到了确定性的 `Decision`，还没有真正落库。本节把行为接成：

```text
validated candidate
-> SELECT current item FOR UPDATE
-> Resolve
-> INSERT / UPDATE / NOOP / FORGET
-> INSERT evidence
-> COMMIT
```

核心边界：

- MySQL 是 memory item 的事实源
- item 保存某个事实的当前状态
- evidence 保存这个状态由哪些原始用户消息支持
- item 和 evidence 必须在同一事务成功或一起失败
- 每条查询都必须显式携带 `user_id`

## 2. Part 1：为什么需要两张表

Schema：

- [memory_items.sql](/offline-rag-go-lab/sql/memory_items.sql:1)

`memory_items` 保存当前事实：

```text
(user_id, kind, memory_key) -> value, status, version
```

唯一键是：

```sql
UNIQUE KEY uk_memory_items_identity (user_id, kind, memory_key)
```

因此两个用户都能拥有 `project_fact/implementation_language`，但同一用户只能有一个当前版本。

`memory_item_evidence` 保存来源：

```text
memory_item_id
+ user_id
+ source_session_id
+ source_message_id
+ operation
+ evidence_text
```

它不是另一份当前事实，而是审计链。唯一键：

```sql
(memory_item_id, source_session_id, source_message_id, operation)
```

让同一来源、同一操作重复执行时返回 0 行新增，而不是重复插入证据。

## 3. Part 2：Store 对外只暴露三种行为

代码：

- [store.go](/offline-rag-go-lab/internal/memoryitem/store.go:1)

```go
type MemoryStore interface {
    // Get 读取一个用户的指定 memory identity。
    Get(ctx context.Context, userID string, kind Kind, key string) (Item, bool, error)

    // Apply 校验 candidate，并在一个事务里更新 item 和 evidence。
    Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)

    // ListActive 只返回当前用户仍允许召回的 active item。
    ListActive(ctx context.Context, userID string) ([]Item, error)
}
```

这里的 `context.Context` 用来传播超时和取消。调用方超时后，MySQL 查询不会继续无限等待。

`Get` 的 `bool` 用来区分：

```text
found=false, err=nil  -> 合法地不存在
found=false, err!=nil -> 查询失败
found=true            -> 找到 item
```

不能只返回空 `Item`，否则“不存在”和“数据库返回了零值”会混在一起。

## 4. Part 3：Apply 的事务顺序

简化后的关键代码：

```go
func (s *Store) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
    // 模型输出必须先通过确定性校验，非法候选不应开启事务。
    normalized, err := ValidateAndNormalizeCandidate(
        req.UserID, req.SessionID, req.Candidate, req.SourceMessages,
    )
    if err != nil {
        return ApplyResult{}, err
    }

    // Begin 开启数据库事务；后续 item 和 evidence 共用同一个 tx。
    tx, err := s.transactions.Begin(ctx)
    if err != nil {
        return ApplyResult{}, err
    }

    committed := false
    defer func() {
        // defer 会在函数返回时执行；未成功 Commit 就尝试 Rollback。
        if !committed {
            _ = tx.Rollback()
        }
    }()

    // FOR UPDATE 锁住当前 identity，避免并发请求同时基于旧版本决策。
    current, err := tx.FindForUpdate(
        ctx, req.UserID, normalized.Kind, normalized.Key,
    )

    decision, err := Resolve(currentPtr, normalized)
    next, err := s.persistDecision(ctx, tx, req.UserID, decision)

    // 每条 source_message_id 都写成一条可追溯 evidence。
    for _, sourceID := range normalized.SourceMessageIDs {
        _, err = tx.InsertEvidence(ctx, Evidence{/* ... */})
    }

    // Commit 成功后才能向调用方返回成功结果。
    if err := tx.Commit(); err != nil {
        return ApplyResult{}, err
    }
    committed = true
    return result, nil
}
```

顺序不能改成“先提交 item，再单独插 evidence”。否则第二步失败后，会留下没有来源的事实。

## 5. Part 4：SELECT FOR UPDATE 和 version 各解决什么

MySQL adapter：

- [store_mysql.go](/offline-rag-go-lab/internal/memoryitem/store_mysql.go:1)

读取当前值：

```sql
SELECT id, user_id, kind, memory_key, value, status, version,
       created_at, updated_at
FROM memory_items
WHERE user_id = ? AND kind = ? AND memory_key = ?
FOR UPDATE;
```

`?` 是参数占位符，值通过 driver 参数传入，不用字符串拼 SQL。

更新时再检查旧 version：

```sql
UPDATE memory_items
SET value = ?, status = ?, version = ?, updated_at = ?
WHERE id = ?
  AND user_id = ?
  AND kind = ?
  AND memory_key = ?
  AND version = ?;
```

两层职责不同：

- `FOR UPDATE` 在当前事务期间串行化同一 identity 的决策
- `version = ?` 是最后一道乐观锁保护，影响行数不是 1 就返回 conflict
- 唯一键处理两个事务都观察到“尚不存在”后同时 INSERT 的竞争

Go 通过 `RowsAffected()` 取得 UPDATE 实际影响行数；INSERT 通过 `LastInsertId()` 取得新 item ID。

## 6. Part 5：NOOP 为什么仍然写 evidence

假设已有：

```text
project_fact/implementation_language=Go, version=1
```

另一条用户消息再次明确“项目仍使用 Go”：

- item 值没变，所以 action 是 NOOP
- version 仍为 1
- 新消息仍是有效来源，所以新增一条 evidence

这能区分：

```text
事实状态是否变化 -> 看 item version
有多少原始依据   -> 看 evidence
```

重复执行完全相同的来源时，MySQL 唯一键错误 `1062` 被 adapter 转成 `affected=0, err=nil`，表示幂等命中，不是业务失败。

## 7. Part 6：本地配置和真实运行

本地配置文件：

```text
/offline-rag-go-lab/config/recent-chat.env
```

该文件已被 `.gitignore` 排除。需要包含：

```dotenv
RECENT_CHAT_MYSQL_DSN=你的用户:你的密码@tcp(127.0.0.1:3306)/offline_rag?parseTime=true
MEMORY_STORE_SCHEMA_PATH=sql/memory_items.sql
```

入口：

- [main.go](/offline-rag-go-lab/cmd/memory-store-demo/main.go:1)

执行：

```bash
go run ./cmd/memory-store-demo \
  --config config/recent-chat.env \
  --apply-schema
```

`go run` 会先编译 `./cmd/memory-store-demo` 包，再立即运行临时二进制。`--config` 指定文件配置；`--apply-schema` 会读取并幂等执行两条 `CREATE TABLE IF NOT EXISTS`。

demo 固定使用：

```text
user_id    = memory-store-demo-user-20260712-a
session_id = memory-store-demo-20260712-a
```

入口不提供覆盖这两个值的 flag，避免误把六条 fixture 消息写进真实用户。已有六条消息还必须逐条满足预期 role 和正文，否则立即停止。

## 8. Part 7：真实首次运行结果

本机 MySQL 首次执行结果：

```text
Applied schema: sql/memory_items.sql
Seeded source messages: 6
insert project_fact/implementation_language version=1 status=active evidence_inserted=1
noop   project_fact/implementation_language version=1 status=active evidence_inserted=1
update project_fact/implementation_language version=2 status=active evidence_inserted=1
update project_fact/implementation_language version=3 status=active evidence_inserted=1
insert preference/temporary_tool version=1 status=active evidence_inserted=1
forget preference/temporary_tool version=2 status=forgotten evidence_inserted=1
Active items: 1
Evidence rows: 6
```

这里验证了：

1. 同一事实从 INSERT v1 到 NOOP v1，再 UPDATE v2/v3
2. NOOP 没增加 version，但新增了来源 evidence
3. `temporary_tool` 从 active v1 变为 forgotten v2
4. active 查询只剩 `implementation_language=Go`
5. 六条原始用户消息都有 evidence

## 9. Part 8：真实重复运行结果

review 发现原实现重跑会重新执行 Go -> Rust -> Go，使 version 不必要增长。修复后，入口会先验证完整终态；只有两张 item 都不存在且 evidence 为 0 才执行 fixture。

第二次运行：

```text
Applied schema: sql/memory_items.sql
Demo state already complete; no writes applied.
Active items: 1
Evidence rows: 6
```

这次没有调用 `Apply`，所以主事实仍是 v3、临时工具仍是 forgotten v2，evidence 仍是 6。

如果发现部分数据或版本不符合终态，demo 会报错停止，不会猜测如何修复或覆盖。

## 10. Part 9：测试和人工查询

运行本节测试：

```bash
go test ./internal/memoryitem ./cmd/memory-store-demo
```

事务测试覆盖：

- insert item + evidence 后 commit
- evidence 失败时 rollback
- NOOP 不增 version 但可加 evidence
- duplicate evidence 幂等
- update、forget 和 forgotten 恢复
- version conflict 与 duplicate insert conflict
- reader/locked item 越过 user 边界时拒绝
- commit 失败时不返回伪成功

在 MySQL 客户端中可只读确认专用数据：

```sql
SELECT id, user_id, kind, memory_key, value, status, version
FROM memory_items
WHERE user_id = 'memory-store-demo-user-20260712-a'
ORDER BY id;

SELECT memory_item_id, source_message_id, operation, evidence_text
FROM memory_item_evidence
WHERE user_id = 'memory-store-demo-user-20260712-a'
ORDER BY id;
```

## 11. 当前实现和生产级差异

当前已经形成真实的 MySQL item/evidence 事务闭环，但生产还应根据运行数据继续补：

- version/duplicate conflict 的有限次数重试和指标
- 独立 migration，而不是把 `CREATE TABLE IF NOT EXISTS` 当升级工具
- 用复合外键在数据库层进一步约束 evidence user 与父 item user 一致
- outbox/worker 把 MySQL 变化可靠同步到 Qdrant
- 定期检查 MySQL 与 Qdrant 索引漂移

并发重试和复合外键迁移已进入优化清单，不在本节没有真实需求时扩展成迁移框架。

## 12. 总结与重点

1. MySQL item 是当前事实，evidence 是原始来源，两者职责不能混用。
2. candidate 先校验，再开启事务；item 与 evidence 同事务提交。
3. `FOR UPDATE`、version 条件和唯一键共同处理并发，不是三选一。
4. NOOP 不增加 version，但新的可靠来源仍可增加 evidence。
5. 所有查询都带 `user_id`，Store 还会检查数据库返回对象没有越过用户边界。
6. demo 固定专用身份，并对重复运行只读验证，不能污染真实用户或反复增加版本。
7. 下一节以 MySQL active item 为事实源，用 `bge-m3` 生成 1024 维向量，再写入新的 Qdrant collection。
