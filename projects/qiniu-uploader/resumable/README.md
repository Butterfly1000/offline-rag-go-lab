# 断点续传版执行说明

**本地跑通请直接看：[SOP.md](./SOP.md)**（三步：进目录 → `setup_venv.sh` → auth → upload）

---

这份文档告诉你如何执行 `resumable/` 目录下的 3 个 Python 脚本：

- `qiniu_resumable_auth.py`
- `qiniu_resumable_upload.py`
- `qiniu_resumable_download.py`

注意：

- 这套是“纯断点续传版”
- 服务端下发配置里不再包含 `largeFileThreshold`
- 也不再包含 `resumeRecordDir`
- 断点记录目录由客户端脚本自己在本地维护

---

## 0. 在哪个目录执行（很重要）

下面命令分两种写法，**二选一**，不要混用。

### 方式 A：在 `projects/qiniu-uploader/` 目录下执行（README 默认）

```bash
cd /path/to/offline-rag-go-lab/projects/qiniu-uploader
python3 resumable/qiniu_resumable_auth.py ...
python3 resumable/qiniu_resumable_upload.py --session-file resumable/session.json
```

### 方式 B：在 `projects/qiniu-uploader/resumable/` 目录下执行（你当前终端常见）

```bash
cd /path/to/offline-rag-go-lab/projects/qiniu-uploader/resumable
python3 qiniu_resumable_auth.py ...
python3 qiniu_resumable_upload.py --session-file session.json
```

常见错误：人已在 `resumable/` 里，仍写 `python3 resumable/qiniu_resumable_auth.py`，会变成 `resumable/resumable/...`，报 `No such file or directory`。

---

## 1. 先准备什么

### Python

建议：

- Python 3.10 或更高

检查命令：

```bash
python3 --version
```

### 七牛 Python SDK

上传脚本依赖七牛官方 Python SDK。

推荐安装（与 `python3` 同一解释器）：

```bash
cd projects/qiniu-uploader/resumable
python3 -m pip install -r requirements.txt
```

或：

```bash
python3 -m pip install qiniu
```

说明：

- 若执行 `pip install qiniu` 报 `ModuleNotFoundError: No module named 'pip'`，说明 `/usr/local/bin/pip` 已损坏，请改用上面的 **`python3 -m pip`**。
- 安装后自检：`python3 -c "from qiniu.services.storage.uploaders import ResumeUploaderV2; print('ok')"`

---

## 2. 脚本分别是做什么的

### `qiniu_resumable_auth.py`

作用：

- 模拟服务端
- 生成断点续传版上传配置 JSON

### `qiniu_resumable_upload.py`

作用：

- 模拟 daemon / 客户端
- 读取上传配置
- 扫描本地目录
- 调用七牛断点续传上传
- 输出每个文件做了哪些动作

### `qiniu_resumable_download.py`

作用：

- 下载七牛文件到本地

---

## 3. 第一步：生成上传配置

在 **`projects/qiniu-uploader/`** 下执行：

```bash
python3 resumable/qiniu_resumable_auth.py \
  --pid "pid_20260603_001" \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "DeviceName/Date/" \
  --local-dir "/你的本地目录路径" \
  --file-name-pattern ".*\\.(mp4|pdf|jpg)$" \
  --upload-host "https://up-z2.qiniup.com" \
  --max-file-size 5368709120
```

在 **`projects/qiniu-uploader/resumable/`** 下执行时，把上面命令里的 `resumable/` 前缀去掉，输出可写到当前目录：

```bash
python3 qiniu_resumable_auth.py \
  --pid "pid_20260603_001" \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "DeviceName/Date/" \
  --local-dir "/你的本地目录路径" \
  --file-name-pattern ".*\\.(mp4|pdf|jpg)$" \
  --upload-host "https://up-z2.qiniup.com" \
  --max-file-size 5368709120 \
  > session.json
```

作用：

- 生成一份断点续传上传配置 JSON

如果你想保存成文件：

```bash
python3 resumable/qiniu_resumable_auth.py \
  --pid "pid_20260603_001" \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "DeviceName/Date/" \
  --local-dir "/你的本地目录路径" \
  --file-name-pattern ".*\\.(mp4|pdf|jpg)$" \
  --upload-host "https://up-z2.qiniup.com" \
  --max-file-size 5368709120 \
  > resumable/session.json
```

---

## 4. 第二步：执行断点续传上传

先确认已安装 qiniu SDK（见第 1 节），否则会提示「缺少 qiniu SDK」。

### 用配置文件上传

在 **`projects/qiniu-uploader/`** 下：

```bash
python3 resumable/qiniu_resumable_upload.py \
  --session-file resumable/session.json
```

在 **`projects/qiniu-uploader/resumable/`** 下：

```bash
python3 qiniu_resumable_upload.py \
  --session-file session.json
```

### 用 JSON 字符串上传

在 **`projects/qiniu-uploader/`** 下：

```bash
python3 resumable/qiniu_resumable_upload.py \
  --session-json '{"pid":"pid_20260603_001",...}'
```

在 **`projects/qiniu-uploader/resumable/`** 下把 `resumable/` 前缀去掉即可，例如：

```bash
python3 qiniu_resumable_upload.py \
  --session-json '{"pid":"pid_20260603_001","provider":"qiniu","mode":"resumable_v2",...}'
```

### 临时覆盖本地扫描目录

如果配置里有 `localDir`，但你想临时换一个目录：

```bash
python3 resumable/qiniu_resumable_upload.py \
  --session-file resumable/session.json \
  --local-dir "/另一个本地目录"
```

---

## 5. 上传脚本实际做了什么

`qiniu_resumable_upload.py` 每处理一个文件，会做这些事：

1. 读取服务端下发配置
2. 扫描本地目录
3. 按正则过滤文件
4. 校验文件大小
5. 生成七牛 `key`
6. 检查本地断点记录目录 `.qiniu_resume_records`
7. 判断本次是新上传还是恢复上传
8. 调用七牛官方 SDK 的 `ResumeUploaderV2`
9. 上传完成后输出结果

输出里有一个很重要的字段：

- `actions`

这个字段会列出脚本对每个文件做过哪些动作，例如：

- `validated_file`
- `built_object_key`
- `checked_local_resume_record`
- `new_upload_started`
- `wrote_local_resume_state`
- `calling_qiniu_resume_uploader_v2`
- `sdk_upload_completed`
- `cleared_local_resume_state`

如果失败，也会看到类似：

- `sdk_upload_failed`
- `updated_local_resume_state_after_failure`

---

## 6. 第三步：下载文件

命令：

```bash
python3 resumable/qiniu_resumable_download.py \
  --url "https://你的下载地址/DeviceName/Date/demo.mp4" \
  --output "/你的本地保存路径/demo.mp4"
```

---

## 7. 本地断点记录放在哪里

上传脚本会在当前工作目录下自动维护一个本地目录：

```text
.qiniu_resume_records
```

作用：

- 保存断点续传过程中本地状态
- 下次再次执行上传时，可以识别为恢复上传

说明：

- 这个目录是客户端本地实现细节
- 不是服务端下发字段

---

## 8. 最小执行顺序

如果你只想先跑通，按这个顺序：

1. 安装依赖

```bash
cd projects/qiniu-uploader/resumable
python3 -m pip install -r requirements.txt
```

2. 生成上传配置（在 `projects/qiniu-uploader/resumable/` 下示例）

```bash
python3 qiniu_resumable_auth.py \
  --pid "pid_20260603_001" \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "DeviceName/Date/" \
  --local-dir "/你的本地目录路径" \
  --file-name-pattern ".*\\.(mp4|pdf|jpg)$" \
  > session.json
```

3. 执行上传

```bash
python3 qiniu_resumable_upload.py \
  --session-file session.json
```

4. 如果需要，再执行下载

```bash
python3 resumable/qiniu_resumable_download.py \
  --url "https://你的下载地址/DeviceName/Date/demo.mp4" \
  --output "/你的本地保存路径/demo.mp4"
```

---

## 9. 一句话总结

`qiniu_resumable_auth.py` 负责生成服务端下发配置，`qiniu_resumable_upload.py` 负责按断点续传方式上传并输出执行动作，`qiniu_resumable_download.py` 负责下载文件。
