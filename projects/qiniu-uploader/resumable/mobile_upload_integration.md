# 手机端七牛断点续传接入说明

这份文档给手机端 / daemon 开发同学使用。

目标：

- 服务端下发一份“七牛断点续传”配置
- daemon 按配置扫描本地目录
- 对符合规则的文件使用七牛分片上传 v2
- 上传中断后可继续恢复

这份文档只描述断点续传版，不影响 `../scripts/` 目录下现有表单上传方案。

---

## 1. 服务端会下发什么

服务端会下发一份 JSON，示例：

```json
{
  "pid": "pid_20260603_001",
  "provider": "qiniu",
  "mode": "resumable_v2",
  "bucketName": "media-prod",
  "uploadToken": "xxxxx:xxxxx:xxxxx",
  "keyPrefix": "DeviceName/Date/",
  "expiresAt": 1780467273,
  "expiresInSec": 7200,
  "uploadHost": "https://up-z2.qiniup.com",
  "constraints": {
    "maxFileSize": 5368709120
  },
  "localDir": "/sdcard/AirDroid/videos",
  "fileNamePattern": ".*\\.(mp4|pdf|jpg)$",
  "resumable": {
    "uploadApiVersion": "v2"
  }
}
```

---

## 2. 字段说明

### 业务字段

`pid`

- 本次上传任务的唯一业务标识
- 手机端后续回传活动日志时原样带回

### 七牛断点续传字段

`provider`

- 固定为 `qiniu`

`mode`

- 固定为 `resumable_v2`
- 表示这次不是普通表单上传，而是七牛分片上传 v2

`bucketName`

- 七牛存储空间名称
- 断点续传 SDK 初始化时需要

`uploadToken`

- 七牛上传凭证
- 客户端原样使用

`keyPrefix`

- 七牛对象前缀
- 最终 `key = keyPrefix + 文件名`

`uploadHost`

- 上传域名
- 如果服务端指定了，就优先使用这个上传域名

`expiresAt`

- token 绝对过期时间

`expiresInSec`

- token 有效期，单位秒

### 客户端扫描字段

`localDir`

- 本地扫描目录

`fileNamePattern`

- 文件名匹配正则
- 只有匹配的文件才上传

### 断点续传控制字段

`resumable.uploadApiVersion`

- 当前固定为 `v2`

### 限制字段

`constraints.maxFileSize`

- 单文件最大大小
- 超过后直接跳过或报错

---

## 3. daemon 需要做什么

daemon 流程：

1. 接收服务端下发的 JSON
2. 读取 `localDir`
3. 按 `fileNamePattern` 扫描文件
4. 校验 `maxFileSize`
5. 为每个文件生成 `key`
6. 为每个文件创建本地断点记录
7. 使用七牛分片上传 v2 上传
8. 如果中断，保留断点记录
9. 下次继续从断点恢复

---

## 4. key 如何生成

规则：

- `key = keyPrefix + 文件名`

例如：

- `keyPrefix = DeviceName/Date/`
- 文件名 = `demo.mp4`

最终：

```text
DeviceName/Date/demo.mp4
```

---

## 5. 上传都做了什么

这部分是断点续传版最重要的说明。

daemon 在上传一个文件时，实际做的是下面这些动作：

### 第 1 步：校验文件

- 检查文件是否存在
- 检查是否匹配 `fileNamePattern`
- 检查文件大小是否超过 `constraints.maxFileSize`

### 第 2 步：生成最终 key

- 根据 `keyPrefix + 文件名` 生成七牛对象 key

### 第 3 步：查找本地断点记录

- 看 daemon 本地断点记录目录里是否已有这个文件的断点记录
- 如果有，说明这是恢复上传
- 如果没有，说明这是新上传

### 第 4 步：初始化七牛分片上传

- 使用 `bucketName`
- 使用 `uploadToken`
- 使用 `uploadHost`（如果服务端指定）
- 调用七牛分片上传 v2

### 第 5 步：上传文件分片

- SDK 会把大文件切成多个分片
- 每个分片分别上传
- 中途失败时，已完成分片不会丢失

### 第 6 步：中断恢复

- 如果 App 被杀、网络断开、设备重启
- daemon 下次重新启动后，继续从断点位置恢复

### 第 7 步：完成上传

- 所有分片上传完成后
- 七牛侧完成文件合成
- 客户端拿到最终结果

### 第 8 步：回传服务端

- 更新活动日志状态
- 回传文件成功或失败结果

---

## 6. 为什么断点续传和普通表单上传不同

普通表单上传：

- 一次 HTTP 请求
- 整个文件一次性上传

断点续传：

- 底层是多次分片请求
- 会有本地断点记录
- 上传失败后可以继续恢复

所以：

- 断点续传不是给现有表单上传“多加一个参数”
- 而是换成另一种上传模式

---

## 7. 客户端实现建议

建议优先使用七牛官方 SDK 的分片上传 v2 能力。

原因：

- SDK 已经封装了分片上传逻辑
- 接入复杂度比自己实现分片协议低很多
- 更适合 daemon 场景

---

## 8. 最小伪代码

```text
config = 服务端返回的 JSON

files = 扫描 config.localDir

for file in files:
    if 文件名不匹配 config.fileNamePattern:
        continue

    if file.size > config.constraints.maxFileSize:
        continue

    key = config.keyPrefix + file.name

    if 本地已有断点记录:
        标记为恢复上传
    else:
        标记为新上传

    调用七牛 resumable v2 上传

    上传完成后回传结果
```

---

## 9. 注意事项

1. `uploadToken` 有过期时间，大文件上传时要特别注意。
2. daemon 必须在本地维护可持久化的断点记录目录，不能只放内存。
3. `key` 必须稳定，恢复上传时要和第一次保持一致。
4. 断点续传只适合用在有本地持久化能力的 daemon / app 场景。
5. 小文件不一定必须走断点续传，但本版文档默认统一按断点续传处理。
