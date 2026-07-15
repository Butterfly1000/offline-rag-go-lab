# AI Initialization

主题：新 AI 接手项目时的初始化检查、依赖解析和运行排障。

这份文档必须在开始修改代码或判断“某个依赖不兼容”之前阅读。它补充
AI_COLLABORATION.md，前者负责环境和依赖，后者负责教学和协作方式。

## 1. 初始化顺序

新 AI 接手后，先按下面顺序执行：

1. 阅读 AI_INITIALIZATION.md 和 AI_COLLABORATION.md
2. 阅读 docs/teaching/00-teaching-protocol.md
3. 阅读 docs/teaching/00-learning-status.md
4. 检查工作区状态，不覆盖用户已有修改
5. 检查 Go 版本和 module 实际依赖来源
6. 先运行最小测试，再运行 demo 或修改代码

不要只根据 import 自动重新创建一个 Go 项目。必须先确认当前仓库的
go.mod、replace 和本地依赖目录。

## 2. 当前 Go 环境和依赖规则

根模块声明在 go.mod：

    require github.com/sugarme/tokenizer v0.3.0
    replace github.com/sugarme/tokenizer => ./third_party/github.com/sugarme/tokenizer

这里的 replace 很重要：

- require 记录逻辑上的模块名称和版本
- replace 指定实际使用的源代码位置
- ./third_party/github.com/sugarme/tokenizer 是仓库中的本地模块
- go.mod 不会把 replace 指向的源代码嵌入文件本身

完整 clone 仓库时必须同时保留：

    go.mod
    go.sum
    third_party/github.com/sugarme/tokenizer/

验证实际解析结果：

    go version
    go list -m -json github.com/sugarme/tokenizer

输出中必须能看到 Replace 指向：

    ./third_party/github.com/sugarme/tokenizer

如果没有 Replace，当前构建使用的是 module cache 或网络下载的上游版本，
不能把它和本项目的本地版本当成同一份代码。

## 3. tokenizer 的特殊兼容边界

某些 Hugging Face tokenizer.json 会包含正则回溯引用，例如 \1、\2、\3。

Go 标准库 regexp 基于 RE2，不支持这类 backreference。当前项目在本地
tokenizer 版本中做了定向兼容：

- 普通正则继续使用标准库 regexp
- 检测到回溯引用时使用 github.com/dlclark/regexp2
- 保持 tokenizer 原有的匹配接口和 offset 行为

相关代码：

- third_party/github.com/sugarme/tokenizer/normalizer/pattern.go
- third_party/github.com/sugarme/tokenizer/go.mod
- 根目录 go.mod 中的 replace

遇到 tokenizer 加载错误时，不能直接下结论说 Go 1.26 不兼容。应先判断：

1. 当前 Go 版本是否真的不支持项目代码
2. 是否使用了本地 replace 版本
3. 报错是否来自 tokenizer.json 中的正则规则
4. 是否缺少本地 tokenizer 目录或 regexp2 依赖

当前项目已在 Go 1.26 环境验证通过。这不能证明所有 tokenizer 都兼容，
但能证明“Go 1.26 必然不兼容本项目”是不准确的判断。

## 4. tokenizer 资产和 tokenizer 代码是两件事

不要混淆：

- tokenizer 代码库：Go 如何解析和执行 tokenizer 规则
- tokenizer.json：某个具体模型的配置、词表和 processor

代码库可以加载成功，不代表资产一定和 Ollama 模型严格匹配。资产来源、模型名称、
tokenizer 指纹和完整 chat template 需要单独验证。

## 5. 最小验证顺序

在判断环境是否可用时，使用：

    go version
    go list -m -json github.com/sugarme/tokenizer
    go test ./internal/tokenizerdemo
    go run ./cmd/tokenizer-demo --text '初始化验证：Go tokenizer。'

如果最后一步失败，先保留完整错误，不要立即升级或替换 Go 版本。

## 6. 常见故障判断

### 找不到本地 replacement 目录

如果看到 replacement directory 不存在，确认完整仓库已经 clone，
没有只复制 cmd、internal 或 go.mod。

### 没有使用本地 replacement

如果 go list -m -json 没有 Replace，或 Dir 指向 module cache，
回到仓库根目录检查 go.mod 和本地目录，不要在独立临时目录里重新 go get。

### 加载 tokenizer.json 时出现正则错误

处理顺序：

1. 检查实际使用的 module 目录
2. 检查 normalizer/pattern.go 是否包含 regexp2 分支
3. 检查 third_party tokenizer 的 go.mod 是否声明 regexp2
4. 再判断 tokenizer 资产是否包含其他未支持规则

### 代码加载成功但 token 结果异常

继续检查 tokenizer 资产来源、模型匹配、完整 chat template，以及是否只计算
content 而不是模型实际收到的完整 prompt。

## 7. 给新 AI 的固定要求

遇到依赖或运行错误时，必须先报告：

1. 完整错误信息
2. go version 结果
3. go list -m -json 中的 Replace 和 Dir
4. 失败发生在编译、加载还是编码阶段
5. 当前判断是 Go 版本、依赖来源、tokenizer 资产还是代码问题

只有拿到这些证据后，才能提出升级 Go、更换依赖或重写实现的建议。

## 8. 初始化完成标准

新 AI 应该能明确回答：

- 当前项目实际使用哪个 Go 版本
- tokenizer 实际从哪里加载
- 为什么仓库需要 third_party 本地版本
- 为什么 Go 标准 regexp 不能直接处理当前 tokenizer 的某些规则
- tokenizer 代码和 tokenizer 资产分别如何验证
- 下一步应该继续教学、运行验证还是修改代码

