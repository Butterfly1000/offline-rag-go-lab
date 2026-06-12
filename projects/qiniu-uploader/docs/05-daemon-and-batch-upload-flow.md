# 05 Daemon 与批量上传流程

这份文档专门给“服务端下发任务，设备本地扫描目录，再批量上传”的场景。

## 1. 这个流程的核心特点

不是用户手动选一个文件上传，而是：

1. 服务端生成一个上传任务
2. 客户端或 daemon 收到任务
3. 本地扫描某个目录
4. 过滤出符合条件的文件
5. 批量上传
6. 把结果按业务约定回传服务端

## 2. 普通版 Daemon 流程

普通表单上传的最小闭环是：

1. 服务端创建设备活动日志并生成 `pid`
2. 服务端下发 upload session
3. daemon 读取 `localDir`
4. daemon 用 `fileNamePattern` 过滤文件
5. 生成待上传列表
6. 回传待执行列表
7. 逐个上传到七牛
8. 逐个回传结果

对应文档：

- `integrations/mobile_upload_daemon_flow.md`
- `integrations/mobile_upload_integration.md`
- `integrations/mobile_upload_report_to_server.md`

## 3. 为什么这个流程适合前缀 token

因为同一设备在一个任务里，通常要传的是：

- 同一批文件
- 同一类业务
- 同一目标前缀

所以最稳的模型是：

- 一个设备一个 token
- 一个 token 覆盖这个设备这次任务下的多个文件

## 4. 回传服务端的重点

这套材料里已经形成了一个很明确的约定：

- `pid`
- `progress`
- `extra_upload_info`

这里最重要的不是 HTTP 细节，而是业务状态流：

- 扫描后先回传待执行列表
- 每个文件上传后再回传结果

## 5. 什么时候普通版不够用

一旦出现下面这些情况，就该考虑断点续传版：

- 大文件
- 弱网
- 断网恢复
- 长时间后台执行

这时不要再硬撑普通 `form_post`，应该切到 `resumable/` 区域。
