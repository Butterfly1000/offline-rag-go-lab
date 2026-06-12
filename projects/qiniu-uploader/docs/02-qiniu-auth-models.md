# 02 七牛授权模型

这份文档把原来零散讨论的“七牛凭证、授权、预授权上传”收成一张图。

## 1. 先记住 4 个角色

### `AK/SK`

长期密钥，只能放服务端。

用途：

- 生成 UploadToken
- 生成私有下载 URL
- 调管理接口

### `UploadToken`

短期上传凭证。

用途：

- 给客户端做上传授权
- 可限制 bucket、key、keyPrefix、过期时间、大小、类型

### 私有下载 URL

这是给下载/存在性检查用的，不是上传 token。

用途：

- GET 下载
- HEAD 检查对象是否存在

### OAuth2

更适合“第三方七牛账号授权给你”的场景。

当前这个子项目里没有把 OAuth2 实现出来，但讨论里已经明确：

- 自家七牛账号更适合 AK/SK 路线
- 第三方授权更适合 OAuth2 路线

## 2. 七牛上传是不是 S3 那种 presigned URL

严格说，不完全是。

七牛标准上传更常见的模型是：

- 服务端生成 `UploadToken`
- 客户端拿 `uploadUrl + uploadToken + key` 去 POST

所以真正起授权作用的核心是：

- `UploadToken`

不是单独一条签名 URL。

## 3. 一个文件一个 token 吗

不一定。

如果策略是：

- `scope = <bucket>:<key>`

那比较接近“一文件一次授权”。

如果策略是：

- `scope = <bucket>:<keyPrefix>`
- `isPrefixalScope = 1`

那一个 token 可以覆盖同一前缀下的多个文件。

这正是这个项目里最重要的授权思路。

## 4. 客户端必须先上抛文件清单，服务端才能发 token 吗

不是七牛机制强制要求。

更准确地说：

- 从七牛协议本身看，不需要客户端先上抛文件名/大小，也能直接签发 UploadToken
- 从业务控制角度看，客户端上抛信息会帮助服务端做审计、限流、冲突控制和任务拆分

所以这是两个层面：

1. 七牛协议层
   不强制
2. 业务风控层
   往往有价值

## 5. 多设备能不能共用一个 token

技术上可以。

工程上要谨慎。

如果多个设备共用同一个前缀 token，会遇到：

- key 冲突
- 审计困难
- 单设备无法单独撤销
- token 泄漏影响整批设备

所以更推荐：

- 同一任务可以共用同一授权模型
- 但 token 最好按设备维度签发

## 6. 这份授权模型最后落到哪里

对应到本项目：

- 普通上传授权脚本：`scripts/qiniu_prefix_uptoken.py`
- 对象检查/下载签名：`scripts/qiniu_check_object_exists.py`、`scripts/qiniu_download_object.py`
- 断点续传授权：`resumable/qiniu_resumable_auth.py`
