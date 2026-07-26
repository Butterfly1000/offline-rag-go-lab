# F07：准备 tokenizer 资产，而不是猜 token

对应工程课：[Tokenizer Demo SOP](../tokenizer-demo-sop-qwen2.md)。本课只建立本地资产与代码库的边界，不从网络下载，也不引用任何机器的历史目录或真实配置。

## 1. 本课一句话

tokenizer 代码负责执行规则，`tokenizer.json` 是模型相关规则资产；两者都齐全，才能得到可重复的 token 计数。

## 2. 先用人话理解

代码像读卡器，`tokenizer.json` 像一张具体门禁卡。读卡器存在不代表你已经有正确的卡；拿到卡也不等于它适合当前模型。

## 3. 系统里有什么

- 目标资产路径：`assets/tokenizers/qwen2/tokenizer.json`。
- 复制工具：`scripts/bootstrap/tokenizer-asset.sh SOURCE [DESTINATION]`；它只复制，不下载、不验证模型身份。首参缺失时才会读取 `RECENT_CHAT_TOKENIZER_SOURCE` 作为 source。
- Go 代码库：根 `go.mod` 将 `github.com/sugarme/tokenizer` replace 到 `third_party/github.com/sugarme/tokenizer`。
- 使用者：`cmd/tokenizer-demo` 与 `internal/tokenizerdemo`；资产来源必须由用户按模型家族/授权渠道提供。

## 4. 一条完整链路

用户提供的合法本地 `tokenizer.json` → bootstrap 脚本复制到约定路径 → Go 的本地 replace 库读取资产 → demo/服务加载并编码。文件复制成功只证明文件到位，不证明它与某个 Ollama 模型完全匹配。

## 5. 实际怎么做

先由用户取得有权使用且已确认模型家族的本地资产，再执行：

```bash
sh scripts/bootstrap/tokenizer-asset.sh /path/provided-by-user/tokenizer.json
go list -m -json github.com/sugarme/tokenizer
```

- bootstrap：纯本地写入；依赖用户提供的源文件。它的完整形状是 `SOURCE [DESTINATION]`，不传首参才可用 `RECENT_CHAT_TOKENIZER_SOURCE`。外部 source 复制到不同 destination 时可重复但不是 no-op：会覆盖目标资产，旧资产不会自动恢复，应先自行备份。source 与 destination 指向同一个文件会使 `cp` 失败，不能把这种情况称作可重复初始化。
- `go list`：纯本地只读；应显示 Replace 指向仓库的 `third_party/github.com/sugarme/tokenizer`，不应把网络 module cache 当作本项目实际实现。

## 6. 结果怎么看

脚本打印 `Tokenizer asset initialized:` 表示复制完成。没有首参也没有 `RECENT_CHAT_TOKENIZER_SOURCE` 时会打印 usage；源不存在或 source=destination 时会失败。若 `go list` 没有本地 Replace，先检查根 `go.mod` 与完整仓库，而不是先换模型或重新下载。

## 7. 算一遍或走一遍

同一段“我”可能是 1 token，也可能不是，取决于具体资产与执行库。文件大小、目录名 `qwen2`、甚至能成功复制，都不能替代“用该资产实际编码”的验证。

## 8. 常见误解

“bootstrap 会帮我下载官方 tokenizer”是错的，它只 `cp` 用户给出的文件。也不能把 `third_party` 的 Go 库和 `tokenizer.json` 混成同一个东西：前者是执行器，后者是数据资产。

## 9. 当前实现与生产边界

已保证：本地复制路径和本地 replace 边界。未保证：资产来源可信度、上游 revision、资产与运行模型/chat template 的严格匹配；这些需记录来源、指纹并在后续课验证。

## 10. 面试怎么说

30 秒版：token 计数依赖“实现库 + 模型资产”两个对象。项目把 tokenizer 库固定到本地 replace，bootstrap 只把用户提供的资产放到约定位置，因此复制成功不是模型兼容证明。展开点：为什么不自动下载、为什么要记录来源和 SHA、replace 的作用。

## 11. 自检题与答案

问：脚本能联网下载 tokenizer 吗？答：不能，只复制。问：不传 SOURCE 一定会失败吗？答：若已设置 `RECENT_CHAT_TOKENIZER_SOURCE`，它会使用该变量；否则打印 usage。问：`go list` 看到本地 Replace 能证明资产匹配模型吗？答：不能，只证明代码库来源。

## 12. 事实锚点

- 原课：[tokenizer-demo-sop-qwen2.md](../tokenizer-demo-sop-qwen2.md)
- 工具：`scripts/bootstrap/tokenizer-asset.sh`、根 `go.mod`
- Go 组件：`cmd/tokenizer-demo/main.go`、`internal/tokenizerdemo/tokenizer.go`
- 初始化说明：[AI_INITIALIZATION.md](../../../AI_INITIALIZATION.md)
