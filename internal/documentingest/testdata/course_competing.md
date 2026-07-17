# Analytics Pipeline

这个竞争项目使用 Python 编写批处理任务，不使用 Go。PostgreSQL 保存权威业务记录。

## Vector Storage

向量检索使用 Qdrant，但 collection 归属 analytics-competing scope，不能被 document-ingestion-course 查询召回。

## Release Process

发布采用直接更新服务配置，不使用文档 snapshot alias。回滚通过恢复上一份配置完成。
