# 03 Upload Session 与 App 契约

这份文档回答的是：

**如果服务端要给客户端或 daemon 下发一个上传任务，最小契约应该长什么样？**

## 1. 推荐不要下发“纯 curl 字符串”

更稳的做法是下发一份结构化 upload session。

原因：

- 字段更容易校验
- 客户端更容易替换动态值
- 以后切换 provider 或模式时更容易扩展

## 2. 普通表单上传的最小 upload session

建议至少包含：

```json
{
  "provider": "qiniu",
  "mode": "form_post",
  "uploadUrl": "https://up-z2.qiniup.com",
  "uploadToken": "xxxxx",
  "keyPrefix": "videos/task123/device456/",
  "expiresAt": 1780380873,
  "expiresInSec": 1800,
  "constraints": {
    "maxFileSize": 5368709120
  }
}
```

## 3. 客户端真正需要负责什么

普通上传时，客户端最核心的职责只有几件：

1. 扫描本地目录
2. 找到待上传文件
3. 生成最终 `key`
4. 把 `token + key + file` 发给 `uploadUrl`

也就是说，客户端不需要知道：

- AK/SK
- 七牛签名算法
- 七牛后台配置细节

## 4. 服务端应该负责什么

服务端负责：

1. 生成短期 UploadToken
2. 决定 keyPrefix
3. 决定是否限制大小/MIME
4. 决定 token 是否按任务共享还是按设备拆分
5. 给客户端返回统一 upload session

## 5. 一个 token 覆盖多个文件的前提

如果你希望客户端扫描一个目录并上传很多文件，那最自然的模型是：

- 一个任务
- 一个设备
- 一个前缀 token
- 多个文件复用

例如：

- `videos/task789/device123/`

这样客户端只要保证每个文件的最终 `key` 都落在此前缀下，就可以共享同一个 token。

## 6. 什么时候需要客户端先上抛信息

### 不上抛也可以直接发 token

适合：

- 服务端 push 任务
- 客户端本地自行扫描
- 服务端只关心前缀范围

### 先上抛再签 token 更稳

适合：

- 需要控制文件总数
- 需要提前防止重名覆盖
- 需要单批次审计
- 需要根据文件大小决定普通上传还是断点续传

## 7. 和当前材料的对应关系

对应普通上传：

- `scripts/qiniu_prefix_uptoken.py`
- `scripts/upload_local_directory_to_qiniu.py`
- `integrations/mobile_upload_integration.md`
- `integrations/mobile_upload_daemon_flow.md`
- `integrations/mobile_upload_report_to_server.md`

对应断点续传：

- `resumable/qiniu_resumable_auth.py`
- `resumable/qiniu_resumable_upload.py`
