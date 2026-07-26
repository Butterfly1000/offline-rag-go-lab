# L04：从 Ollama 读取模型容量和模板

对应工程课：[Tokenizer 小节 04](../ollama-model-inspect-sop.md)。本课从 Ollama 的只读 `/api/show` 获取模型元数据，不把容量或包装写死在项目里。

## 1. 本课一句话

`/api/show` 同时给出模型的架构、上下文长度和 prompt template，让预算与包装依据实际模型元数据。

## 2. 先用人话理解

像向设备厂商索取说明书：你先知道它的容量和插头形状，之后才能正确分配空间和接线，而不是照搬另一台设备的参数。

## 3. 系统里有什么

- 外部只读接口：`POST /api/show`，JSON 请求体为 `{"model":"qwen:7b"}`。
- 摘要入口：`cmd/ollama-model-inspect --base-url ... --model ...`。
- 客户端：`internal/recentchat/ollama.go` 的 `Show`；读取 `general.architecture` 后拼出 `{architecture}.context_length`，并读取 `template`。
- 输出：family、architecture、parameter size、quantization、context length、capabilities、parameters、template。

## 4. 一条完整链路

模型名 → POST body → Ollama show 响应 → 解析架构（如 qwen2）→ 查 `qwen2.context_length` → 返回 context 与模板摘要。缺失/非正 context 元数据会报错，不会静默当 0。

## 5. 实际怎么做

```bash
curl -sS http://127.0.0.1:11434/api/show -H 'Content-Type: application/json' -d '{"model":"qwen:7b"}'
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/ollama-model-inspect --base-url http://127.0.0.1:11434 --model qwen:7b
```

- 两条都是外部只读；依赖 Ollama 运行且目标模型存在，不生成回答、不修改模型/数据库，只会让 Go 命令写 build cache。
- 可安全重复执行；失败先检查 Ollama、地址和模型名。

## 6. 结果怎么看

原始 JSON 的 `model_info` 与顶层 `template` 是关键。摘要中的 `Context length` 是解析后的正整数；`Template` 是模型提供的包装规则。当前示例的 `32768` 仅是当时 `qwen:7b` 元数据报告的值，换模型、版本或运行配置都可能不同。

## 7. 算一遍或走一遍

若 `general.architecture="qwen2"`，代码查找键 `qwen2.context_length`。当该值为 32768 时，表示本次示例报告的总上下文上限，不是“可给 recent history 的 32768 token”。

## 8. 常见误解

“32768 可以直接写进所有请求”是错的；它由当前模型元数据决定，实际运行还能设置更小上下文。也不能只算 user 正文：template 中的 role、标记、换行和 assistant 前缀同样进入模型输入。

## 9. 当前实现与生产边界

已保证：按架构动态读取 context key，保留模型 template，并拒绝缺失元数据。未保证：Ollama 实际运行时的所有覆盖参数、资产与模型严格同源；生产可缓存只读元数据并持续核对变更。

## 10. 面试怎么说

30 秒版：我不把上下文容量和 chat 包装写死，而是调用 Ollama `/api/show`。客户端先读架构，再查对应 context key，并拿到 template；这使预算和渲染跟随当前模型。展开点：POST 请求字段、32768 的示例性、为何模板影响 token。

## 11. 自检题与答案

问：`/api/show` 会生成回答吗？答：不会，它只读模型详情。问：32768 是否只属于 history？答：不是，是示例中的总容量。问：架构元数据缺失时应该返回 0 吗？答：不应，代码返回错误。

## 12. 事实锚点

- 原课：[ollama-model-inspect-sop.md](../ollama-model-inspect-sop.md)
- 命令：`cmd/ollama-model-inspect/main.go`
- Go 组件：`internal/recentchat/ollama.go`
- 测试：`internal/recentchat/ollama_show_test.go`
