# 04 表单上传模板与多云兼容

这份文档把原来关于“能不能做通用模板”的讨论收束成一个结论：

**可以抽象上传配方，但不能假设所有云的字段名完全一样。**

## 1. 最稳的抽象不是 curl，而是“上传配方”

推荐的思路：

- 服务端下发结构化规则
- App 只负责替换少量动态变量
- App 自己组装请求

而不是直接下发一整条 curl 作为协议本身。

## 2. 什么场景下可以做统一协议

如果范围限定为：

- 单文件
- `multipart/form-data`
- 服务端预先签好字段
- 客户端原样带上字段并上传

那七牛、S3、OSS 可以做到“执行层通用”。

## 3. 什么不能完全统一

不能强行统一的是：

- 字段名
- 签名算法
- policy 字段
- 某些返回格式
- 分片/断点续传能力

所以真正能通用的是：

- App 执行逻辑

不是：

- 各家字段长得一模一样

## 4. 推荐的通用配方形态

可以把协议抽成：

```json
{
  "provider": "qiniu",
  "mode": "form_post",
  "url": "https://upload.qiniup.com",
  "fileFieldName": "file",
  "fields": [
    { "name": "token", "value": "UPLOAD_TOKEN" },
    { "name": "key", "value": "videos/{taskId}/{deviceId}/{fileName}" }
  ]
}
```

客户端负责替换：

- `{taskId}`
- `{deviceId}`
- `{fileName}`
- 本地文件路径

## 5. 多云兼容的真实边界

### 七牛

典型字段：

- `token`
- `key`
- `file`
- 可选 `x:*`

### S3

典型字段：

- `key`
- `policy`
- `x-amz-*`
- `file`

### OSS

典型字段：

- `key`
- `policy`
- `OSSAccessKeyId`
- `Signature`
- `file`

所以结论是：

- 可通用的是上传引擎
- 不可通用的是具体字段集合

## 6. 当前项目里相关的材料

- `integrations/upload_rule_template_compatibility.md`
- `docs/03-upload-session-and-app-contract.md`
- `integrations/mobile_upload_integration.md`
