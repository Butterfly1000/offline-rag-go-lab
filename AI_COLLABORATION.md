# AI Collaboration

主题：如何让不同模型在这个项目里无缝接手、继续教学、继续实现。

这份文档放在根目录，是给“新接手的 AI”与“用户自己”共同使用的协作入口。

新 AI 必须先阅读根目录的 [AI_INITIALIZATION.md](/offline-rag-go-lab/AI_INITIALIZATION.md:1)，
完成环境、module 实际来源和 tokenizer 兼容边界检查后，再按本文档继续教学或实现。

目标只有一个：

- 不管是换模型、换设备、换会话，都能先读统一文档，再按同一套方式继续。

---

## 1. 这份文档解决什么问题

这个项目不是单纯问答，而是持续教学 + 持续实现。

因此，AI 不能只依赖当前聊天上下文，而要依赖项目内已经沉淀的文档继续工作。

这份文档主要固定三件事：

1. 新模型开始前先读什么
2. 你怎么告诉 AI 继续当前工作
3. 一轮结束后，AI 必须把什么写回仓库

---

## 2. 新模型开始前必须先读什么

新开一个模型时，不要直接让它开始讲。

先让它阅读这些文档：

1. `docs/teaching/00-teaching-protocol.md`
2. `docs/teaching/00-learning-status.md`

如果这次要继续 `recent-window / memory / recent-chat` 相关内容，再继续读：

3. `docs/teaching/recent-window-layer-01.md`
4. `docs/teaching/recent-window-runtime-sop.md`
5. `docs/2026-06-29-recent-window-real-implementation-plan.md`

这一步的目的是：

- 先对齐教学方式
- 先对齐当前学习进度
- 先对齐当前实现状态

---

## 3. 你如何告诉 AI 先读再讲

新模型开场时，直接发送下面这段：

```text
先阅读 docs/teaching/00-teaching-protocol.md 和 docs/teaching/00-learning-status.md。

如果这次要继续 recent-window / memory 相关内容，再继续读：
- docs/teaching/recent-window-layer-01.md
- docs/teaching/recent-window-runtime-sop.md
- docs/2026-06-29-recent-window-real-implementation-plan.md

读取后先告诉我三件事：
1. 我们现在学到哪了
2. 下一小段应该讲什么
3. 你这次会把哪些新结论写回 docs/teaching
```

这样做的意义是：

- 不让模型一上来就重讲项目总览
- 先验证它是否真的接上上下文
- 先确认它知道这次要读哪些文档、写哪些文档

---

## 4. 一轮教学或实现结束后，怎么让 AI 回写

当你准备结束一轮工作时，比如：

- 今天先到这里
- 准备换设备
- 准备换模型
- 准备第二天继续

直接发送下面这段：

```text
现在请收口并回写文档：

1. 把这次我已经听懂的内容归档到 docs/teaching
2. 更新 docs/teaching/00-learning-status.md，写明现在学到哪、下一章是什么
3. 如果这次有新的运行步骤或验证结果，也补到 docs/teaching
4. 最后告诉我：下一个模型接手前，最少必须先读哪几份文档
```

这一步的目标是把“聊天里的结论”变成“仓库里的结论”。

---

## 5. 如果 AI 教了一整晚，第二天怎么接

第二天换一个新模型前，建议再补一句：

```text
请先不要继续新内容，先检查昨晚是否已经把教学结果完整写回 docs/teaching；如果没有，先补齐再继续。
```

这样可以防止：

- 夜里讲了很多，但没有归档
- 新模型接手时只看到一半状态

---

## 6. 新模型接手正确的标志

如果一个新模型接手正确，它应该做到：

1. 先读文档，再继续
2. 知道当前已学到哪
3. 知道下一章是什么
4. 不重讲已经确认“懂了”的内容
5. 每讲一小段就停下来确认理解
6. 在结束前把结论回写到 `docs/teaching/`

如果它没有做到这些，就让它重新按第 3 节的提示词执行。

---

## 7. 当前项目的协作原则

这个项目当前已经明确的协作原则是：

1. 教学按“行为 / 层”推进
2. 每次只讲一个小段
3. 用户说“懂了”后，才视为真正学会
4. 学会的内容必须归档到 `docs/teaching/`
5. 不只讲 demo，也要讲生产级常见做法
6. 能真实运行验证的地方，优先做真实验证

---

## 8. 给新模型的一句话要求

如果你只想给新模型一句最短的要求，可以直接发：

```text
先读 AI_INITIALIZATION.md 和 AI_COLLABORATION.md，再按其中要求去读 docs/teaching 里的文档；不要直接开讲，先告诉我当前学到哪、下一步讲什么、你会把什么写回文档。
```
