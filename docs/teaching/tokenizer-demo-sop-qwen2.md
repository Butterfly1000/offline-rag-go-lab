# Tokenizer Demo SOP

主题：如何用 `qwen:7b` 对应的 tokenizer 在 Go 里做本地 token 计数

这份 SOP 只解决一件事：

- 让你在本地真实跑通一个 tokenizer demo

它不是 recent window 升级文档，也不是 session summary 文档。

---

## 1. 先回答一个关键问题

为什么要算 token？

因为模型能承受的上下文长度有限。

你本机 `qwen:7b` 的真实模型信息里显示：

- `context length = 32768`

这个值来自：

```bash
ollama show qwen:7b
```

但 `32768` 不是给 recent history 独占的，而是整次请求的总预算。

通常要一起分给：

- system prompt
- recent history
- 当前用户输入
- retrieval context
- 模型输出预留

所以 token 控制既是为了：

1. 不超上下文上限
2. 也为了给不同部分做预算分配

---

## 2. 这次 demo 做了什么

本次 demo 新增入口：

- [cmd/tokenizer-demo/main.go](/offline-rag-go-lab/cmd/tokenizer-demo/main.go:1)

核心计数组件：

- [internal/tokenizerdemo/tokenizer.go](/offline-rag-go-lab/internal/tokenizerdemo/tokenizer.go:1)

当前支持两种演示：

1. 单段中文文本 token 计数
2. 多条 chat messages 的 token 计数

注意：

- 当前 message demo 先按 `content` 本身计数
- 也额外演示了一个简单 transcript 的总 token 数
- 这还不是完整 chat template 计数

---

## 3. tokenizer 资产放哪里

当前代码默认从这里读取：

```text
assets/tokenizers/qwen2/tokenizer.json
```

这个文件默认不会被 git 提交。

---

## 4. tokenizer.json 怎么得到

这是这次最关键的一步。

### 方式 1：从模型上游仓库获取

对 `qwen:7b` 这类本地模型，真正要找的不是 “Ollama 的某个专用接口”，而是：

- 这个模型家族上游对应的 tokenizer 资产

对你当前模型来说，`ollama show qwen:7b` 已经显示：

- `architecture = qwen2`

所以我们要找的是：

- `Qwen2` 对应的 tokenizer 资产

通常最常见的就是：

- `tokenizer.json`

这类文件一般来自模型的 Hugging Face 仓库或同源模型分发目录。

也就是说，下一台机器、下一个模型，最通用的 SOP 不是：

- “先找我之前电脑上的某个目录”

而是：

1. 先确认当前模型属于哪个家族
2. 再去这个模型家族的上游仓库找 `tokenizer.json`
3. 再放到项目约定目录里

### 方式 2：复用你本机已经有的 tokenizer 资产

这次我没有成功直接从外网下载 `Qwen2 tokenizer.json`，所以实际走的是本机复用路线。

我在你本机上找到了这份文件：

```text
~/Dolphin/hf_model/tokenizer.json
```

然后复制到项目默认位置：

```text
assets/tokenizers/qwen2/tokenizer.json
```

### 这份 `~/Dolphin/hf_model/tokenizer.json` 是怎么来的？

这里我不能乱说。

我目前只能确认：

- 这份文件在你机器上真实存在
- 它长得就是 Hugging Face 风格的 `tokenizer.json`
- 当前 demo 已经用它成功跑通

但我**不能确认**它是不是：

1. 你之前手动下载模型时自动带下来的
2. 某个其他工具落到这个目录的
3. 还是你自己之前放进去的

所以这一步要诚实记录成：

- `~/Dolphin/hf_model/tokenizer.json` 是这次实际可用来源
- 但它的生成来源当前未知

这也是为什么，给后续模型的通用借鉴应该是：

- 优先按“模型家族 -> 上游 tokenizer 资产”去找
- 本机已有文件只能作为当前机器的快捷路径

---

## 5. 这次为什么没有直接走外网下载

这次真实过程里，我尝试过直接下载 `Qwen2` 的 `tokenizer.json` 到项目目录。

但当前环境里，这条路没有成功，表现是：

- 外网下载长时间无返回

所以这次实际采用的是：

- 先复用你机器上已有的 `~/Dolphin/hf_model/tokenizer.json`

这也是一条真实生产里常见的处理方式：

- 如果线上下载链路不稳定，就优先复用已经验证过的本地模型资产

---

## 6. 你现在机器上的可用来源

我这次实际使用的是你本机已经存在的一份文件：

```text
~/Dolphin/hf_model/tokenizer.json
```

我已经把它复制到了项目里的默认位置：

```text
assets/tokenizers/qwen2/tokenizer.json
```

如果你以后换机器，没有这个文件，就需要自己准备一份与 `qwen:7b / qwen2` 匹配的 `tokenizer.json`。

---

## 7. 如何运行 demo

在项目根目录执行：

```bash
go run ./cmd/tokenizer-demo
```

这条命令具体做了什么：

1. 编译 [cmd/tokenizer-demo/main.go](/offline-rag-go-lab/cmd/tokenizer-demo/main.go:1)
2. 载入 `assets/tokenizers/qwen2/tokenizer.json`
3. 对一段写死的中文文本做 token 计数
4. 对一组写死的 messages 做 token 计数
5. 把结果直接打印到终端

也就是说，这不是“启动一个长期服务”，而是：

- 跑一次
- 打印一次结果
- 进程结束

如果你想显式指定 tokenizer 路径，也可以：

```bash
go run ./cmd/tokenizer-demo --tokenizer assets/tokenizers/qwen2/tokenizer.json
```

---

## 8. 预期输出怎么看

运行后，你会看到 2 段输出。

### 6.1 Text Demo

这里会输出：

- 原文
- token 总数
- 前若干个 tokens
- 前若干个 token ids

这一步的意义是：

- 证明 Go 已经能真实加载 tokenizer
- 证明这不是字符数截断，而是真实 token 化

### 6.2 Messages Demo

这里会输出：

- 每条 message 的 token 数
- 所有 message 的 `content-only total`
- 拼成简单 transcript 后的 token 总数

这一步的意义是：

- 让你看到“同样几条 message”，token 成本是可以具体计算的
- 也让你看到“逐条相加”和“拼成整体文本后再算”不一定完全一样

---

## 9. 这次真实跑通时遇到的卡点

这部分很重要，因为它解释了：

- 为什么这次不是一条命令就结束
- 为什么 SOP 里不能只写“go run 一下就好了”

### 卡点 1：默认 Go 构建缓存目录没有权限

直接执行：

```bash
go run ./cmd/tokenizer-demo
```

会碰到系统缓存目录权限问题。

所以这次实际验证时，我改成了：

```bash
env GOCACHE=/private/tmp/offline-rag-go-lab-gocache GOSUMDB=off go run -mod=mod ./cmd/tokenizer-demo
```

也就是说：

- `GOCACHE` 被显式改到了可写的临时目录

### 卡点 2：第三方依赖下载不稳定

新增 Go tokenizer 依赖后，`go mod tidy` / `go run` 一度卡在外部依赖下载上。

表现包括：

- 旧代理配置不可用
- 外部模块下载超时

所以这次处理方式是：

- 尽量减少额外外网依赖
- 把缺的最小依赖做成本地 replace

### 卡点 3：Go tokenizer 库和 Qwen2 tokenizer.json 不完全兼容

这是这次最有价值的一个真实问题。

当前使用的 Go tokenizer 库在加载 `Qwen2 tokenizer.json` 时，卡在一个正则表达式上。

原因是：

- `Qwen2 tokenizer.json` 里有带回溯引用的正则，比如 `\\1`
- Go 标准库 `regexp` 不支持这种写法

所以这次最终处理是：

- 把 `github.com/sugarme/tokenizer` 做成本地 replace
- 用 `regexp2` 兼容这类正则

这一步非常值得记住：

**拿到 tokenizer.json，不等于任意语言库都能零成本直接吃进去。**

---

## 10. 当前 demo 的边界

这次 demo 已经是真实 tokenizer 计数，但它还有明确边界：

1. 还没有接入 `recent-chat`
2. 还没有实现 token-budget window
3. 还没有应用完整 chat template

也就是说，这次先解决的是：

**本地 tokenizer 怎么真实加载，Go 怎么真实计数。**

下一步才是：

- 把它接到 recent window
- 按 token budget 从后往前装消息

---

## 11. 你学这一步最应该确认什么

如果这一步理解了，你应该能确认下面三件事：

1. `32768` 是模型上下文上限，不是 recent history 独占上限
2. token 计数必须依赖与当前模型匹配的 tokenizer
3. Go 可以直接本地加载 `tokenizer.json` 做真实 token 计数
