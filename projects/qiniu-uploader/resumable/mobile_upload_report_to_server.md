# 手机端断点续传结果回传服务端说明

这份文档是断点续传版活动日志补充说明。

现有活动日志接口不改，仍然沿用：

- `pid`
- `progress`
- `extra_upload_info`

本版只补充断点续传下的状态和字段。

---

## 1. 基本约定

### `pid`

- 由服务端下发
- 作为整批上传任务的唯一标识

### `progress`

- 固定传 `2`
- 含义不变：本次执行动作本身成功进入处理流程

注意：

- `progress=2` 不代表文件上传成功
- 文件是否成功，取决于 `extra_upload_info` 里的 `status`

---

## 2. `extra_upload_info` 继续沿用数组

类型：

- 数组

说明：

- 支持批量回传待执行文件列表
- 支持每个文件上传后单独回传结果

---

## 3. 断点续传版状态定义

相比普通上传版，断点续传版只多保留一个“断点恢复中”状态，不做过度扩展。

`status` 取值：

- `0`：待执行
- `1`：上传中
- `2`：断点恢复中
- `3`：成功
- `4`：失败

解释：

- `0`：扫描完成，文件已经入待执行队列
- `1`：正常开始上传
- `2`：检测到已有本地断点记录，继续恢复上传
- `3`：上传完成
- `4`：上传失败

---

## 4. 必传字段

每个文件对象都传这些字段：

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
- `is_resumed`

字段说明：a

`uploaded_size`

- 成功时等于 `filesize`
- 失败时传已上传的大小，没有则传 `0`

`cloud_type`

- 固定传 `q`

`is_resumed`

- `0`：不是恢复上传
- `1`：本次是从历史断点恢复

---

## 5. 回传时机

分两步：

### 第一步：扫描完成后，批量回传待执行列表

此时每个文件：

- `status = 0`
- `uploaded_size = 0`
- `is_resumed = 0`

### 第二步：每个文件上传开始和结束时分别回传

开始上传时：

- `status = 1` 或 `2`

上传完成时：

- 成功：`status = 3`
- 失败：`status = 4`

---

## 6. 批量待执行回传示例

```json
{
  "pid": "pid_20260603_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "DeviceName/Date/a.mp4",
      "filesize": 10485760,
      "uploaded_size": 0,
      "status": 0,
      "upload_time": 0,
      "cost_time": 0,
      "error_info": "",
      "file_type": "mp4",
      "cloud_type": "q",
      "error_code": 0,
      "uploadToken": "xxxxx:xxxxx:xxxxx",
      "is_resumed": 0
    }
  ]
}
```

---

## 7. 单文件开始上传回传示例

### 普通开始上传

```json
{
  "pid": "pid_20260603_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "DeviceName/Date/a.mp4",
      "filesize": 10485760,
      "uploaded_size": 0,
      "status": 1,
      "upload_time": 1780460100,
      "cost_time": 0,
      "error_info": "",
      "file_type": "mp4",
      "cloud_type": "q",
      "error_code": 0,
      "uploadToken": "xxxxx:xxxxx:xxxxx",
      "is_resumed": 0
    }
  ]
}
```

### 断点恢复开始上传

```json
{
  "pid": "pid_20260603_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "DeviceName/Date/a.mp4",
      "filesize": 10485760,
      "uploaded_size": 4194304,
      "status": 2,
      "upload_time": 1780460200,
      "cost_time": 0,
      "error_info": "",
      "file_type": "mp4",
      "cloud_type": "q",
      "error_code": 0,
      "uploadToken": "xxxxx:xxxxx:xxxxx",
      "is_resumed": 1
    }
  ]
}
```

---

## 8. 单文件最终结果回传示例

### 成功

```json
{
  "pid": "pid_20260603_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "DeviceName/Date/a.mp4",
      "filesize": 10485760,
      "uploaded_size": 10485760,
      "status": 3,
      "upload_time": 1780460300,
      "cost_time": 18,
      "error_info": "",
      "file_type": "mp4",
      "cloud_type": "q",
      "error_code": 0,
      "uploadToken": "xxxxx:xxxxx:xxxxx",
      "is_resumed": 1
    }
  ]
}
```

### 失败

```json
{
  "pid": "pid_20260603_001",
  "progress": 2,
  "extra_upload_info": [
    {
      "filename": "a.mp4",
      "path": "/sdcard/AirDroid/videos/a.mp4",
      "key": "DeviceName/Date/a.mp4",
      "filesize": 10485760,
      "uploaded_size": 4194304,
      "status": 4,
      "upload_time": 1780460312,
      "cost_time": 7,
      "error_info": "network timeout",
      "file_type": "mp4",
      "cloud_type": "q",
      "error_code": 504,
      "uploadToken": "xxxxx:xxxxx:xxxxx",
      "is_resumed": 1
    }
  ]
}
```

---

## 9. 一句话给手机端

“断点续传版继续沿用 `pid + progress + extra_upload_info`，只是在文件状态上增加 `断点恢复中`，并额外回传 `is_resumed` 和更准确的 `uploaded_size`。”
