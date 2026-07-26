# L11：自动从模型和完整 prompt 算出历史额度

对应工程课：[Tokenizer 小节 11](../automatic-history-budget-sop.md)。本课把“模型容量从哪来”和“固定 prompt 到底多大”连成一条可失败、不可猜测的数据流。

## 1. 本课一句话

自动规划器先经 `ContextProvider` 读取模型 context，再经 `ConversationCounter` 计算完整固定 prompt，最后用 L07 的公式得出历史额度；任一步失败都返回错误，绝不估算容量。

## 2. 先用人话理解

这像装车前先读车辆载重牌、再称已经装上的货。牌子读不到或秤坏了时，不能凭感觉说“应该还能装一点”，因为那样可能超载。

## 3. 系统里有什么

- `ContextProvider.ContextLength(model)`：按模型名取得 context limit；当前 demo 的实现通过 Ollama client 读取模型信息。
- `ConversationCounter.Count(messages, true)`：渲染固定对话并带 assistant prefix 后计数。
- `AutomaticPlanner`：把上述两个接口和 `promptbudget.Plan` 编排为 `AutomaticPlan`。
- 输出：context、fixed input、output reserve、available history 和 rendered fixed prompt。

## 4. 一条完整链路

`ContextProvider` → Ollama `/api/show` 读取模型 context length；固定 system + 当前 user → `ConversationCounter` 完整渲染并一次计数（带 assistant prefix）→ `Plan(context, fixed, reserve)` → 得到可用历史额度和 rendered 固定 prompt。这里没有调用 `/api/chat`。

## 5. 实际怎么做

```bash
sh scripts/regression/lessons-11-12.sh
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/automatic-budget-demo --base-url http://127.0.0.1:11434 --model qwen:7b --system '你是 Go 助手。' --prompt '解释 recent window。' --output-reserve 2048 --tokenizer assets/tokenizers/qwen2/tokenizer.json
```

- 默认回归脚本与 demo 都会外部只读访问本地 Ollama（`/api/tags` 或 `/api/show`），并读取本地 tokenizer；需要 Go、curl、资产、运行中的 Ollama 和目标模型。它们不调生成接口、不写 MySQL 或聊天业务数据，只会建立可重建 build cache 和临时回归文件，可安全重复。
- 脚本的 `--live` 选项未在此处使用：它会请求 recent-chat `/chat` 并可能写入 user/assistant 业务记录，属于 L12 的 live 验证，不是本课默认的无业务写入检查。

## 6. 结果怎么看

成功时应满足 `fixed input tokens + output reserve tokens + available recent history tokens = context limit`。输出的 rendered fixed prompt 便于核对“fixed”到底包含哪些消息和 assistant prefix；它不是模型已经生成的回答。

## 7. 算一遍或走一遍

假设 provider 返回 context `1000`，counter 对固定 prompt 返回 `120`，调用者选择 reserve `200`，自动计划得到历史 `680`。若 Ollama 读取失败、tokenizer 加载/编码失败、格式化 role 非法，或 `120+200` 超过 context，`AutomaticPlanner.Plan` 直接返回错误；它不会改用默认 context，也不会把 token 数猜成字符数。

## 8. 常见误解

“自动”不表示后台生成模型回答；本课只读 metadata 并本地计数。“拿到 context 后随便估正文 token”也不对，固定输入必须是完整 conversation 的真实格式化计数。

## 9. 当前实现与生产边界

已保证：读取 context 的错误被包装为 `read model context length`，计数错误被包装为 `count fixed prompt tokens`，预算超限照样失败返回；Ollama/context/tokenizer 任一失败都不估算或降级。未保证：网络服务长期可用、模型 metadata 一定正确，或自动规划器自行压缩提示词；这些需要调用方重试、换模型或调整输入。

## 10. 面试怎么说

自动 history budget 由两条可信输入组成：模型自己声明的 context 和同一 tokenizer 对完整固定 prompt 的计数。把它们减去输出预留后才交给窗口；任何元数据或计数失败都 fail closed，而不是猜一个容量继续请求。

## 11. 自检题与答案

问：自动预算会调用 `/api/chat` 吗？答：不会，只读取模型信息并本地计数。问：tokenizer 失败时可否按字符数估算？答：不可，必须返回错误。问：默认脚本会写聊天业务数据吗？答：不会；只有显式 `--live` 才进入 L12 的写入路径。

## 12. 事实锚点

- 原课：[automatic-history-budget-sop.md](../automatic-history-budget-sop.md)
- Go 组件：`internal/promptbudget/automatic.go`、`internal/promptbudget/budget.go`
- demo：`cmd/automatic-budget-demo/main.go`
- 回归：`scripts/regression/lessons-11-12.sh`
- 前置：[L07](L07-context-budget.md)、[L09](L09-conversation-token-count.md)
