# L03：用 SHA256 固定 tokenizer 文件身份

对应工程课：[Tokenizer 小节 03](../tokenizer-fingerprint-sop.md)。SHA256 回答“字节是否没变”，不回答“是不是正确模型”。

## 1. 本课一句话

把已确认的 tokenizer 文件 SHA256 写成预期值，启动或 CI 就能拒绝被替换的字节内容。

## 2. 先用人话理解

像给文件盖防拆封蜡印：两次蜡印相同，表示内容相同；它不证明包裹最初来自哪家商店，也不证明里面适合你的设备。

## 3. 系统里有什么

- 入口：`cmd/tokenizer-inspect --expect-sha256 VALUE`。
- 实现：`InspectFile` 读取全部字节计算 SHA256；`VerifySHA256` 不区分十六进制大小写比较。
- 输入：资产路径和人工确认的预期指纹；输出含实际 `SHA256` 与 `Fingerprint check: matched` 或 mismatch 错误。

## 4. 一条完整链路

文件全部字节 → SHA256 → 输出实际摘要 → 与 `--expect-sha256` 比较 → 相同退出 0；不同返回错误，`go run` 显示 `exit status 1`。

## 5. 实际怎么做

对已确认资产执行：

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/tokenizer-inspect --tokenizer assets/tokenizers/qwen2/tokenizer.json --expect-sha256 b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8
```

- 等级：纯本地只读；依赖资产与 Go，仅写 build cache，可安全重复。
- 成功输出 `Fingerprint check: matched`。
- 故意把预期改为 `wrong-hash` 会失败并输出类似 `tokenizer SHA256 mismatch: expected wrong-hash, actual ...`；这是安全的失败验证，不改变资产。

## 6. 结果怎么看

匹配意味着本次读取的字节与记录指纹一致。不匹配时必须停止后续依赖该资产的步骤并调查来源/替换；不要把失败值“更新成新值”来绕过门禁。

## 7. 算一遍或走一遍

假设已记录 SHA 为 `abc`：文件不变时计算还是 `abc`；任意一个字节改动后摘要应不同。比较只忽略 `ABC` 与 `abc` 的大小写，不忽略内容差异。

## 8. 常见误解

“SHA 相同就证明来自官方或模型匹配”是错的。它只能证明字节未变；若一开始记录了错误资产，SHA 会稳定地验证那个错误资产。

## 9. 当前实现与生产边界

已保证：文件身份可重复检查、差异会以非零退出显式失败。未保证：上游来源真实性、模型 tokenizer/chat template 兼容、官方 token ID 一致性；仍须结合 L02、L04 和对照样例。

## 10. 面试怎么说

30 秒版：SHA256 是 tokenizer 资产身份门禁。部署前比较预期与实际字节摘要，不同就失败，防止资产静默漂移；但哈希不是模型兼容证明。展开点：为什么全字节计算、为何大小写无关、为什么失败不能直接接受新 hash。

## 11. 自检题与答案

问：`wrong-hash` 预期会修改 tokenizer 文件吗？答：不会，只会让命令失败。问：相同 SHA 能证明 chat template 一样吗？答：不能，SHA 只针对该文件字节。

## 12. 事实锚点

- 原课：[tokenizer-fingerprint-sop.md](../tokenizer-fingerprint-sop.md)
- 命令：`cmd/tokenizer-inspect/main.go`
- Go 组件：`internal/tokenizerdemo/inspect.go`
- 测试：`internal/tokenizerdemo/inspect_test.go`
