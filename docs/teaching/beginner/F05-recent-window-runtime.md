# F05：验证 recent-chat 是否真的记住最近对话

对应工程课：[Recent Window Runtime SOP](../recent-window-runtime-sop.md)。操作对象是 **18093** `recent-chat`；它依赖 MySQL + Ollama，不是 18092 mock gateway。

## 1. 本课一句话

用同一 session 的两次真实请求和一次只读 SQL，验证“第一轮写入、第二轮读回、窗口裁剪”这条链路。

## 2. 先用人话理解

不要只问“模型回答得像不像记得”。把它拆成可检查证据：数据库有没有记录、第二次请求到底带了哪些历史、限制变小时是否真的少带了消息。

## 3. 系统里有什么

- HTTP：`POST http://127.0.0.1:18093/chat`；返回 `used_messages` 与 `recent_window`。
- 表：`recent_chat_messages`，保存 `session_id`、`user_id`、`role`、`content`、时间。
- 服务链：`internal/recentchat/http.go` → `service.go` → `store_mysql.go` / `window_count.go` → Ollama。
- 外部依赖：已启动 MySQL、Ollama、可用 `qwen:7b`（或你的可用模型）、可读 tokenizer 资产以及已填写的本地配置。

## 4. 一条完整链路

第一轮：空窗口 → Ollama answer → 按两个写回 flag 分别 INSERT user/assistant。第二轮：MySQL 按 session/user 以 `created_at DESC, id DESC` 取最近记录，再由 Go 反转为正序送入模型 → 成功后再次按 flag 分别写入。Ollama 失败时两条都不写；user INSERT 成功而 assistant INSERT 失败会留下半轮。只读 SQL 独立证明写入事实，而 `recent_window` 证明模型输入事实。

## 5. 实际怎么做

在**同一个 shell** 生成本次唯一的 session/user；后续两次 curl 与 SQL 都复用它，避免固定示例键碰到旧历史：

```bash
# 请把整个条件块一次粘贴到当前 shell；它不会退出你的终端。
unset F05_SUFFIX F05_SESSION F05_USER
if ! command -v uuidgen >/dev/null 2>&1; then
  unset F05_SUFFIX F05_SESSION F05_USER
  printf '%s\n' 'uuidgen is unavailable: stop this setup block.'
  printf '%s\n' 'Before continuing, manually set F05_SUFFIX to one non-empty, brand-new UUID, then initialize F05_SESSION/F05_USER from it in this same shell.'
else
  F05_SUFFIX="$(uuidgen | tr '[:upper:]' '[:lower:]')"
  if test -z "$F05_SUFFIX"; then
    unset F05_SUFFIX F05_SESSION F05_USER
    printf '%s\n' 'uuidgen produced an empty UUID: stop this setup block and set a non-empty, brand-new F05_SUFFIX manually before continuing.'
  else
    F05_SESSION="f05-runtime-$F05_SUFFIX"
    F05_USER="f05-user-$F05_SUFFIX"
    printf 'session=%s user=%s\n' "$F05_SESSION" "$F05_USER"
  fi
fi
```

- 等级：纯本地；正常路径需要 `uuidgen`，只设置当前 shell 变量。缺失工具或空结果时，本段不会设置 session/user，读者必须先在同一 shell 手工设置一个非空、全新的 `F05_SUFFIX` 并从它初始化两个变量，才可继续后面的请求。正常路径每次执行会生成新的 UUID；同一轮两次 curl 与 SQL 必须复用这一次打印出的变量值。

第一轮请求使用正整数 `recent_limit=10`：

```bash
if test -z "${F05_SESSION:-}" || test -z "${F05_USER:-}"; then
  printf '%s\n' 'F05 session/user is not initialized; complete the UUID setup before sending this request.'
else
  curl -X POST http://127.0.0.1:18093/chat -H 'Content-Type: application/json' -d "{\"session_id\":\"$F05_SESSION\",\"user_id\":\"$F05_USER\",\"message\":\"我叫小黄，这个项目是 Go 写的。\",\"model\":\"qwen:7b\",\"recent_limit\":10,\"store_user_turn\":true,\"store_assistant_turn\":true}"
fi
```

- 等级：业务写入；调用 Ollama。仅当 Ollama 成功后，才会分别尝试向 MySQL `recent_chat_messages` 追加 user、assistant；因此可能是两条、零条，或 assistant INSERT 失败时的一条 user 记录。
- 依赖：F04 的 MySQL schema、18093 服务和模型已就绪。
- 重复执行：同一变量值重复 curl 会继续追加成功的 turn，窗口会改变，不能自动回滚；重新生成变量则使用新的隔离键。若需清理，只能由获授权人员在确认该专用键后按组织规则执行，不是本 SOP 的自动步骤。

第二轮将 `message` 改为“你记得我叫什么吗？”并在**同一个 shell**保持同一变量；同样属于业务写入：

```bash
if test -z "${F05_SESSION:-}" || test -z "${F05_USER:-}"; then
  printf '%s\n' 'F05 session/user is not initialized; complete the UUID setup before sending this request.'
else
  curl -X POST http://127.0.0.1:18093/chat -H 'Content-Type: application/json' -d "{\"session_id\":\"$F05_SESSION\",\"user_id\":\"$F05_USER\",\"message\":\"你记得我叫什么吗？\",\"model\":\"qwen:7b\",\"recent_limit\":10,\"store_user_turn\":true,\"store_assistant_turn\":true}"
fi
```

检查记录时执行：

```bash
if test -z "${F05_SESSION:-}" || test -z "${F05_USER:-}"; then
  printf '%s\n' 'F05 session/user is not initialized; complete the UUID setup before running this query.'
else
  mysql -u YOUR_USER -p YOUR_DATABASE -e "SELECT id, session_id, user_id, role, content, created_at FROM recent_chat_messages WHERE session_id = '$F05_SESSION' AND user_id = '$F05_USER' ORDER BY id ASC"
fi
```

- 等级：外部只读；不改变表，可安全重复执行。

双引号让当前 shell 展开前面生成的变量；`YOUR_USER`/数据库名仍须替换为自己的只读可查询连接。`ORDER BY id ASC` 只是便于人按写入顺序观察，不是服务的取数顺序。最后可把下一次请求的 `recent_limit` 改为正整数 `1`；该请求仍会写入，且只应把最近一条历史送入窗口。`0` 不是“不限量”，负数不受 count 服务路径支持。

## 6. 结果怎么看

第一轮预期 `used_messages=0`、`recent_window=[]`；若 Ollama 与两个 INSERT 都成功，SQL 会出现 user 和 assistant 两条。第二轮预期 `used_messages>0` 且窗口非空。`recent_limit=1` 时本次响应窗口最多一条历史。模型文本本身不是唯一证据，应优先看这些结构化字段和 SQL 行；若 INSERT 错误，应把可能存在的半轮与错误一起记录，而不是假定两条都落库。

## 7. 算一遍或走一遍

在 Ollama 成功且两个 INSERT 都成功的条件下，第一轮写入 `U1,A1` 两条。第二轮数据库先按最新顺序取回这两条，再由 Go 反转，所以常见示例为 `used_messages=2`；完成后共有四条 `U1,A1,U2,A2`。第三次若设 `recent_limit=1`，查询/窗口只选择最新历史 `A2`（再加当前新消息），不是把四条都发送。任何一次 Ollama 或 INSERT 失败都会改变这个算例。

## 8. 常见误解

“第二轮的 answer 提到名字就足以证明数据库读回”是错的，模型可能猜中。必须同时确认 `recent_window` 与 SQL。也不能以为 `recent_limit=1` 只影响显示：它改变实际送给模型的历史输入；`recent_limit=0` 也不是不限量，而会导致空历史。

## 9. 当前实现与生产边界

已保证：在各外部调用成功时的读写、双键过滤、按最新记录取回再恢复正序和条数窗口的运行证据。未保证：两次 INSERT 的原子性、每条消息的 token 成本、早期重要信息保留、业务删除工作流与完整审计。唯一测试键仍会写真实数据库，不能当作无副作用 demo。

## 10. 面试怎么说

30 秒版：我会在同一 shell 生成唯一 session/user，用两轮正整数 limit 的请求验证 recent memory：第一轮应是空窗口，成功后按 flag 分别写 user/assistant；第二轮必须返回非空窗口，再用只读 SQL 交叉核验。将 limit 设为 1 可以验证裁剪发生在模型调用前。展开点：为什么 answer 不是充分证据、为什么数据库先倒序取再由 Go 反转、为什么可能出现半轮。

## 11. 自检题与答案

问：第二轮为什么要保持同一 user 和 session？答：查询以二者共同过滤，改任一个都会看不到原窗口。问：Ollama 失败后会写 user/assistant 吗？答：不会，写入在 Ollama 成功之后。问：两个写回 flag 都为 true 是否绝对原子？答：不是，两次 INSERT 独立，可能留下半轮。问：`recent_limit=0` 是不限量吗？答：不是，真实查询为 `LIMIT 0`。问：SQL 查询会改变状态吗？答：不会；`SELECT` 是只读。

## 12. 事实锚点

- 原课：[recent-window-runtime-sop.md](../recent-window-runtime-sop.md)
- 入口：`cmd/recent-chat/main.go`、`internal/recentchat/http.go`
- Go 组件：`internal/recentchat/{service,store_mysql,window_count}.go`
- 配置/SQL：`config/recent-chat.env.example`、`sql/recentchat_messages.sql`
- 测试：`internal/recentchat/service_test.go`、`store_mysql_test.go`、`window_count_test.go`
