# Handoff Guide

主题：如何在新模型里继续当前教学与实现工作

这份文档是给用户自己看的操作说明。目标是：换一个新模型或换一个环境后，仍然能让它按照当前这套方式继续，而不是从头开始乱讲。

---

## 1. 什么时候需要用这份文档

下面这些场景都应该用它：

- 公司里用一个模型，回家换另一个模型
- 当前上下文太长，需要新开一个会话
- 想让新模型直接接手，不想重复解释背景

---

## 2. 新模型开始前，让它先看什么

开新模型后，不要先直接问技术问题。

先让它读取这些文档：

1. [00-teaching-protocol.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/00-teaching-protocol.md:1)
2. [00-learning-status.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/00-learning-status.md:1)
3. [recent-window-layer-01.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-layer-01.md:1)
4. [recent-window-runtime-sop.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-runtime-sop.md:1)

如果新模型要继续 recent-chat 的代码实现，也要让它读：

5. [2026-06-29-recent-window-real-implementation-plan.md](/Users/huangyanyu/offline-rag-go-lab/docs/2026-06-29-recent-window-real-implementation-plan.md:1)

---

## 3. 给新模型的推荐开场提示词

可以直接复制下面这段给新模型：

```text
请先阅读这些文档，再继续当前教学和实现工作：

1. docs/teaching/00-teaching-protocol.md
2. docs/teaching/00-learning-status.md
3. docs/teaching/recent-window-layer-01.md
4. docs/teaching/recent-window-runtime-sop.md
5. docs/2026-06-29-recent-window-real-implementation-plan.md

要求：
- 按 teaching-protocol 里的方式继续教学
- 每次只讲一个小段，讲完停下来确认我是否理解
- 我说“懂了”之后，再把这一段视为已学会并归档
- 先从当前 learning-status 记录的下一章开始，不要重新从项目总览讲起
- 如果继续做实现或验证，也要把运行结果和结论同步沉淀到 docs/teaching
```

---

## 4. 如果新模型是继续教学，不是继续改代码

那你可以用更短的版本：

```text
先阅读 docs/teaching/00-teaching-protocol.md 和 docs/teaching/00-learning-status.md，然后按里面记录的方式继续教学。从当前下一章开始，不要重讲已经学会的内容。每讲一小段就停下来等我确认。
```

---

## 5. 如果新模型是继续实现 recent-chat / memory system

那你可以用这版：

```text
先阅读：
- docs/teaching/00-teaching-protocol.md
- docs/teaching/00-learning-status.md
- docs/2026-06-29-recent-window-real-implementation-plan.md
- docs/teaching/recent-window-layer-01.md
- docs/teaching/recent-window-runtime-sop.md

然后先告诉我：
1. 当前已经完成了什么
2. 下一步应该做什么
3. 如果继续实现，哪些文件要动

要求你保持和文档里一致的教学方式与记录方式。
```

---

## 6. 新模型继续后，要记录什么

后续无论是教学还是实现，都应该继续记录：

1. 新学会的章节或层
2. 新跑通的真实验证步骤
3. 新的运行 SOP
4. 当前学习位置有没有变化
5. 下一章变成了什么

优先记录到：

- `docs/teaching/`

不要只留在聊天上下文里。

---

## 7. 如何判断新模型有没有接对

如果新模型符合下面这些，就说明接得对：

- 它先读文档再继续
- 它知道第 1 层已经学会
- 它知道下一章是“从 recent 到重要”
- 它不会重讲项目概览
- 它会在你说“懂了”后再归档
- 它会继续维护 `docs/teaching/`

如果它没有做到这些，就让它重新按第 3 节的提示词执行。
