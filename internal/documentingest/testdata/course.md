# Document Ingestion

生产文档入库先确定逻辑文档身份，再计算内容版本，最后生成稳定 chunk ID。

## Stable Identity

未变化的内容位于相同结构路径时，应当跨文档版本保留相同 chunk ID。内容改变或移动章节时，应当生成新的 chunk ID。

## Version State

版本从 pending 进入 building。构建成功后进入 ready，发布后才成为 active；构建失败则进入 failed 并允许重试。

```go
if err := ValidateTransition(StatusReady, StatusActive); err != nil {
	return err
}
```

## Storage Boundary

MySQL 保存文档版本和 chunk manifest，是权威事实源。Qdrant 保存 embedding 和检索 payload，是可以重建的派生索引。
