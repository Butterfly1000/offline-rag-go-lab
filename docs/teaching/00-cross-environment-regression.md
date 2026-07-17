# 跨环境回归与排坑

主题：把一台机器上遇到的问题变成其他机器可以重复执行的检查，而不是只依赖聊天上下文。

## 1. 第 8 节一键回归

从仓库根目录执行：

```bash
sh scripts/regression/lesson-08.sh
```

脚本不访问 MySQL、Qdrant、Ollama，也不主动调用业务公网接口，只会：

1. 检查当前 `go` 命令和 `GOMOD`。
2. 检查仓库内的 Qwen2 `tokenizer.json`。
3. 确认 `github.com/sugarme/tokenizer` 实际指向本地兼容版本。
4. 运行 Tokenizer 和 Qwen 消息格式单元测试。
5. 验证中文黄金文本必须得到 15 tokens。
6. 验证合法消息边界和非法 role 拒绝逻辑。

如果新机器的 Go module 缓存为空，`go test` 仍可能按照本机 `GOPROXY` 配置下载
`go.mod` 中的间接依赖。这属于 Go 项目初始化，不是脚本调用业务外部服务。离线机器
应先准备 Go module cache，再运行本回归。

全部通过时，最后会输出：

```text
Lesson 08 cross-environment regression passed.
```

## 2. 为什么第 8 节也要回归 Tokenizer

第 8 节讲的是 Qwen 消息格式，但格式化后的字符串最终还要交给 Tokenizer 计算。
如果 Tokenizer 在新机器上加载了不同实现，消息边界看起来正确，最终预算仍可能错误。

依赖链是：

```text
go.mod 与本地 replace
    -> tokenizer.json 加载
    -> 中文与 added-token 编码
    -> Qwen message 格式化
    -> message token 计算
```

所以本节回归必须覆盖前面的 Tokenizer 基础，而不能只测试 `strings.Builder` 输出。

## 3. 已确认的跨机器问题

### 问题一：Go 1.26 不是直接根因

另一台机器曾在 Go 1.26 下报 `regexp` 相关错误。不能据此直接得出“这个包不兼容
Go 1.26”。真实根因是 Qwen2 的 `tokenizer.json` 包含 Go 标准库 `regexp` 不支持的
正则能力，例如回溯引用；上游 `github.com/sugarme/tokenizer v0.3.0` 直接使用标准库
处理时会失败。

本仓库能运行，是因为 `go.mod` 包含：

```go
replace github.com/sugarme/tokenizer => ./third_party/github.com/sugarme/tokenizer
```

本地版本使用 `regexp2` 处理这类规则，并补充了兼容性修复。因此判断顺序应该是：

1. 先确认实际解析到哪个模块目录。
2. 再确认本地修复目录是否存在。
3. 最后才判断 Go 版本是否相关。

### 问题二：`go.mod` 上传了，不代表本地 replace 一定可用

`replace` 的右侧是相对路径。新机器必须同时拿到：

```text
go.mod
third_party/github.com/sugarme/tokenizer
```

如果只有 `go.mod`，没有 `third_party` 内容，Go 不会自动从网络下载“本地版本”。常见
原因包括分支没有更新、clone/复制不完整，或者运行命令时不在本仓库模块中。

使用下面的命令查看 Go 最终选择的模块：

```bash
go list -m -json github.com/sugarme/tokenizer
```

关键字段是：

```json
{
  "Path": "github.com/sugarme/tokenizer",
  "Version": "v0.3.0",
  "Replace": {
    "Path": "./third_party/github.com/sugarme/tokenizer"
  }
}
```

`Version` 表示依赖声明的基准版本，`Replace` 表示编译时真正使用的代码。看到
`Replace` 才能证明当前机器没有直接使用原始 v0.3.0 源码。

### 问题三：正则能运行，不代表中文 offset 正确

`regexp2` 返回的是 rune 位置，而很多 Go 字符串切片和当前 Tokenizer 内部结构需要
UTF-8 byte 位置。英文通常一个字符一个 byte，错误不容易暴露；中文通常占多个
byte，直接混用会切错文本。

因此回归必须包含中文，不能只用 `hello world`。

### 问题四：旧的 8 tokens 结果是错误结果

下面这段文本当前黄金结果是 15：

```text
我叫小黄，这个项目是 Go 写的。
```

早期曾得到 8，原因包括 Tokenizer 执行链和 added-token split 处理不完整。脚本会把
非 15 的结果直接判定为失败，避免“命令成功执行”掩盖计算错误。

### 问题五：Tokenizer 资产不是 `go mod download` 下载的

本项目默认读取：

```text
assets/tokenizers/qwen2/tokenizer.json
```

它是项目资产，不是 Go module。新机器缺少这个文件时，即使所有 Go 依赖都存在，
Tokenizer demo 仍然无法运行。

### 问题六：Go 构建缓存目录可能不可写

受限环境可能无法写默认 `GOCACHE`。回归脚本默认使用项目内：

```text
.cache/go-build
```

如果手动执行 Go 命令也遇到缓存权限问题，可以运行：

```bash
env GOCACHE="$PWD/.cache/go-build" go test ./internal/tokenizerdemo
```

## 4. 新机器诊断 SOP

不要看到第一条错误就修改依赖版本。按以下顺序执行：

```bash
git status --short
go version
go env GOMOD
go list -m -json github.com/sugarme/tokenizer
test -f assets/tokenizers/qwen2/tokenizer.json
test -f third_party/github.com/sugarme/tokenizer/go.mod
sh scripts/regression/lesson-08.sh
```

判断方法：

- `GOMOD` 不是当前仓库的 `go.mod`：先进入仓库根目录。
- `Replace` 缺失：当前 `go.mod` 不是本项目预期版本。
- `third_party` 缺失：仓库内容或分支不完整。
- Tokenizer 资产缺失：补齐项目资产，不要反复执行 `go mod tidy`。
- 中文测试失败：检查本地兼容代码，不要用英文测试代替。
- 只有消息格式测试失败：再检查第 8 节 `internal/chatprompt` 实现。

## 5. 后续章节如何记录类似问题

每一节不必都创建新文档。出现跨环境问题时，按以下格式追加到本文：

1. 现象：原始错误或错误输出是什么。
2. 根因：经过回归确认的真正原因，不记录未经验证的猜测。
3. 影响：会影响哪些章节、命令或生产行为。
4. 检查：一条可以证明当前状态的命令。
5. 修复：最小修复步骤和修复后的黄金结果。

如果问题能够自动判断，就同时加入对应章节的回归脚本；纯外部服务状态才保留为手动 SOP。
