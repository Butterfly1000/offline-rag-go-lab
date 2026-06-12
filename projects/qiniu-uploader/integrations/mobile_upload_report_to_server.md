# 手机端上传结果回传服务端说明

这份文档是对现有设备活动日志接口的补充说明。

现有接口文档：

- [biz-device-update-activity-log-status](https://gitlab.airdroid.com/airdroid_server/id-airdroid-com/-/wikis/biz-device-update-activity-log-status)

说明：

- 服务端在下发七牛上传信息时，会先创建一条设备活动日志
- 该日志的唯一关联标识为 `pid`
- `pid` 继续沿用现有接口字段，手机端按现有方式回传即可
- 本文档不重复描述现有接口已有字段

本文只补充两件事：

1. `progress` 固定传 `2`
2. 新增参数 `extra_upload_info`，用于回传文件上传信息

---

## 1. 基本约定

### `pid`

- 由服务端下发
- 作为本次上传任务的唯一关联标识
- 手机端后续所有回传都带上这个 `pid`

### `progress`

- 保持传 `2`
- 含义：本次“执行动作”本身是成功的

注意：

- 这里的 `progress=2` 不是“文件上传成功”
- 文件本身是否上传成功，要看 `extra_upload_info` 里的每个文件记录

---

## 2. 新增参数

新增参数命名为：

`extra_upload_info`

类型：

- 数组

说明：

- 支持批量
- 每个元素代表一个文件的上传信息

---

## 3. 回传时机

分两步：

### 第一步：扫描完成后，批量回传“待执行文件列表”

手机端扫描 `localDir` 后，把本次准备上传的文件列表一次性回传。

用途：

- 服务端提前知道这次准备上传哪些文件
- 方便后续逐个更新状态

### 第二步：每上传一个文件，再传一次该文件结果(可能有执行中，成功/失败)

用途：

- 服务端拿到每个文件的实时状态
- 便于统计成功/失败
- 便于失败排查

---

## 4. `extra_upload_info` 必传字段

下面这些字段作为 `extra_upload_info` 中每个文件对象的必传字段。

### `filename`

- 文件名
- 示例：`a.mp4`

### `path`

- 手机端本地路径
- 示例：`/sdcard/AirDroid/videos/a.mp4`

### `key`

- 上传到七牛后的对象 key
- 示例：`videos/task123/device456/a.mp4`

### `filesize`

- 文件原始大小
- 单位：字节

### `uploaded_size`

- 实际已上传大小
- 单位：字节

规则：

- 成功时：`uploaded_size = filesize`
- 失败时：传实际已上传大小，没有则传 `0`

### `status`

- 文件上传结果

取值：

- `0`：待执行
- `1`：执行中
- `2`：失败
- `3`：成功

### `upload_time`

- 客户端上传完成时间
- 传秒级时间戳

### `cost_time`

- 本文件上传耗时
- 单位：秒

### `error_info`

- 错误信息

规则：

- 成功时传空字符串
- 失败时传具体错误内容

---

## 5. 同样需要传的字段

下面这些字段也需要一起传，方便服务端记录和排查。

### `file_type`

- 文件类型
- 传文件后缀或 MIME 类型
- 示例：`mp4`、`pdf`、`jpg`

### `cloud_type`

- 云类型
- 固定传：`q`

说明：

- `q` 表示七牛

### `error_code`

- 错误码
- 成功时传 `0`
- 失败时传实际错误码

### `uploadToken`

- 本次上传使用的 upload token

---


## 6. 待执行批量回传示例

扫描完成后，可以先批量回传待执行文件列表。

此时：

- `status` 传 `0`，表示待执行

示例：

```json
{
  "pid": "pid_20260602_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "videos/task123/device456/a.mp4",
      "filesize": 10485760,
      "file_type": "mp4",
      "cloud_type": "q",
      "status": 0
    },
    {
      "filename": "b.pdf",
      "path": "/sdcard/AirDroid/videos/b.pdf",
      "key": "videos/task123/device456/b.pdf",
      "filesize": 204800,
      "file_type": "pdf",
      "cloud_type": "q",
      "status": 0
    }
  ]
}
```

---

## 8. 单文件上传结果回传示例

每个文件上传完成后，再回传一次该文件的最终结果。

### 成功示例

```json
{
  "pid": "pid_20260602_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "videos/task123/device456/a.mp4",
      "file_type": "mp4",
      "cloud_type": "q",
      "filesize": 10485760,
      "uploaded_size": 10485760,
      "status": 3,
      "upload_time": 1780380100,
      "cost_time": 12,
      "error_info": ""
    }
  ]
}
```

### 失败示例

```json
{
  "pid": "pid_20260602_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "b.pdf",
      "path": "/sdcard/AirDroid/videos/b.pdf",
      "key": "videos/task123/device456/b.pdf",
      "file_type": "pdf",
      "cloud_type": "q",
      "filesize": 204800,
      "uploaded_size": 0,
      "status": 2,
      "upload_time": 1780380112,
      "cost_time": 3,
      "error_code": 401,
      "error_info": "bad token"
    }
  ]
}
```

---

## 9. 最小必传集合

手机端每个文件最终需要回传这些：

- `filename`
- `path`
- `key`
- `filesize`
- `uploaded_size`
- `status`
- `upload_time`
- `cost_time`
- `error_info`
- `file_type`
- `cloud_type`
- `error_code`
- `uploadToken`

外层仍然带：

- `pid`
- `progress = 2`

---

## 10. 一句话给手机端

手机端可以按下面这句话实现：

“扫描完成后，先用 `extra_upload_info` 批量回传待上传文件列表；每个文件上传结束后，再带 `pid` 和 `progress=2` 回传该文件的 `filename/path/key/filesize/uploaded_size/status/upload_time/cost_time/error_info/file_type/cloud_type/error_code/uploadToken`。”
