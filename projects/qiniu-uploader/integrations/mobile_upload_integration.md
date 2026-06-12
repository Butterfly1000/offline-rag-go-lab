# 手机端七牛上传接入说明

这份文档给手机端开发同学使用。

目标只有一件事：

- 服务端下发一份上传配置
- 手机端按配置扫描本地目录
- 找到符合规则的文件后，逐个上传到七牛

---

## 1. 服务端会下发什么

服务端会下发一份 JSON，示例：

```json
{
  "pid": "pid_20260602_001",
  "provider": "qiniu",
  "mode": "form_post",
  "uploadUrl": "https://up-z2.qiniup.com",
  "uploadToken": "xxxxx:xxxxx:xxxxx",
  "keyPrefix": "DeviceName/Date/",
  "expiresAt": 1780380873,
  "expiresInSec": 1800,
  "constraints": {
    "maxFileSize": 5368709120
  },
  "localDir": "/sdcard/AirDroid/videos",
  "fileNamePattern": ".*\\.(mp4|pdf|jpg)$"
}
```

---

## 2. 字段说明

### 业务字段

`pid`

- 本次上传任务/批次的业务 ID（作用类似 task_id）
- 手机端在回传上传结果时应原样带回，服务端用它串联整批文件的状态

### 七牛上传字段

`provider`

- 固定为 `qiniu`
- 表示当前上传目标是七牛

`mode`

- 固定为 `form_post`
- 表示使用 `multipart/form-data` 表单上传

`uploadUrl`

- 上传地址
- 手机端必须直接 POST 到这个地址
- 不要自行替换成别的地址

`uploadToken`

- 七牛上传凭证
- 手机端上传时原样传递
- 不要修改，不要重新编码

`keyPrefix`

- 七牛对象前缀
- 上传文件时，最终 `key` 必须以这个前缀开头
- 例如：`DeviceName/Date/`

`expiresAt`

- token 失效时间
- Unix 时间戳，单位秒

`expiresInSec`

- token 有效期，单位秒

`constraints.maxFileSize`

- 单文件最大大小，单位字节
- 如果文件超过这个值，客户端应跳过或报错

### 客户端扫描字段

`localDir`

- 手机端本地扫描目录
- 只处理这个目录中的文件
- 例如：`/sdcard/AirDroid/videos`

`fileNamePattern`

- 文件名匹配正则
- 只有文件名匹配这个正则的文件才上传
- 例如：`.*\\.(mp4|pdf|jpg)$`

说明：

- 这是“文件名匹配规则”
- 一般只对文件名判断，不对完整路径判断
- 是否区分大小写，由你们客户端实现时自行统一，建议忽略大小写

---

## 3. 手机端需要做什么

手机端流程：

1. 接收服务端下发的 JSON
2. 读取 `localDir`
3. 列出目录中的文件
4. 用 `fileNamePattern` 过滤文件
5. 如果有 `constraints.maxFileSize`，先校验文件大小
6. 为每个文件生成七牛 `key`
7. 按 `multipart/form-data` 上传到 `uploadUrl`

---

## 4. key 如何生成

规则：

- 最终 `key = keyPrefix + 文件名`

例如：

- `keyPrefix = DeviceName/Date/`
- 文件名 = `demo.mp4`

最终：

```text
DeviceName/Date/demo.mp4
```

建议：

- 文件名中的特殊字符如果有风险，可以替换成下划线
- 不要让生成后的 `key` 跑出 `keyPrefix` 范围

---

## 5. 上传请求格式

请求方式：

- `POST`

请求类型：

- `multipart/form-data`

表单字段：

`token`

- 值为服务端下发的 `uploadToken`

`key`

- 值为本次文件上传到七牛后的对象 key

`file`

- 文件二进制内容

等价示意：

```text
POST {uploadUrl}
Content-Type: multipart/form-data

token = {uploadToken}
key   = {keyPrefix + fileName}
file  = {本地文件内容}
```

---

## 6. 客户端伪代码

```text
config = 服务端返回的 JSON

files = 列出 config.localDir 目录下所有文件

for file in files:
    if 文件名不匹配 config.fileNamePattern:
        continue

    if config.constraints.maxFileSize 存在 且 file.size > maxFileSize:
        continue

    key = config.keyPrefix + file.name

    POST config.uploadUrl
        form.token = config.uploadToken
        form.key = key
        form.file = file
```

---

## 7. 注意事项

1. `uploadUrl` 必须使用服务端下发值，不能自行替换。
2. `uploadToken` 可能过期，上传前要注意 `expiresAt`。
3. `key` 必须以 `keyPrefix` 开头。
4. `fileNamePattern` 建议在客户端统一做正则校验。
5. 如果上传失败，建议记录文件名、key、HTTP 状态码、返回内容。

---

## 8. 最小示例

服务端下发：

```json
{
  "provider": "qiniu",
  "mode": "form_post",
  "uploadUrl": "https://up-z2.qiniup.com",
  "uploadToken": "xxxxx:xxxxx:xxxxx",
  "keyPrefix": "DeviceName/Date/",
  "expiresAt": 1780380873,
  "expiresInSec": 1800,
  "constraints": {
    "maxFileSize": 5368709120
  },
  "localDir": "/sdcard/AirDroid/videos",
  "fileNamePattern": ".*\\.mp4$"
}
```

本地目录：

```text
/sdcard/AirDroid/videos
  a.mp4
  b.jpg
  c.txt
```

如果 `fileNamePattern = .*\\.mp4$`

则只上传：

```text
a.mp4
```

最终上传 key：

```text
DeviceName/Date/a.mp4
```
