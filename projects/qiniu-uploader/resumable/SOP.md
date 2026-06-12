# 断点续传本地跑通 — 最简 SOP

**只在一个目录里操作：** `offline-rag-go-lab/projects/qiniu-uploader/resumable`

**只用一种 Python：** 本目录下的 `.venv`（不要用系统自带的 `python3` 直接跑上传脚本）

---

## 第 0 步：进入目录（每次新开终端都要做）

```bash
cd /Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/projects/qiniu-uploader/resumable
```

---

## 第 1 步：安装依赖（只做一次）

```bash
bash setup_venv.sh
```

成功会看到：`qiniu SDK 已就绪`

> 不要用 `pip install qiniu`（你机器上 `/usr/local/bin/pip` 已坏）。  
> 不要用 `python3 -m pip install` 装到系统 Python（Homebrew 会拦）。

---

## 第 2 步：生成上传配置 `session.json`

把 AK/SK 换成你的（或先 `export` 再写 `$QINIU_ACCESS_KEY`）：

```bash
export QINIU_ACCESS_KEY='你的AK'
export QINIU_SECRET_KEY='你的SK'

.venv/bin/python qiniu_resumable_auth.py \
  --pid "pid_20260603_001" \
  --access-key "$QINIU_ACCESS_KEY" \
  --secret-key "$QINIU_SECRET_KEY" \
  --bucket "bizdl-airdroid-com" \
  --key-prefix "videos/task123/device456/" \
  --local-dir "/Users/huangyanyu/Downloads/滴滴出行电子发票及行程报销单" \
  --file-name-pattern '.*\.(mp4|pdf|jpg)$' \
  --upload-host "https://up-z2.qiniup.com" \
  --max-file-size 5368709120 \
  > session.json
```

检查：

```bash
head -5 session.json
```

应看到 `"mode": "resumable_v2"` 和 `"uploadToken"`。

---

## 第 3 步：断点续传上传

```bash
.venv/bin/python qiniu_resumable_upload.py --session-file session.json
```

或用你已有的 JSON（**仍必须用 `.venv/bin/python`**）：

```bash
.venv/bin/python qiniu_resumable_upload.py --session-json '你的整段JSON'
```

成功：终端输出每个文件的 `actions`、`ok: true` 等。

---

## 常见错误对照

| 报错 | 原因 | 处理 |
|------|------|------|
| `can't open file '.../resumable/resumable/...'` | 目录错了又多写一层 `resumable/` | 确认 `pwd` 在 `projects/qiniu-uploader/resumable`，脚本名不要加前缀 |
| `缺少 qiniu SDK` | 用了系统 `python3`，没装 SDK | 必须用 `.venv/bin/python` 或先 `source .venv/bin/activate` |
| `ModuleNotFoundError: No module named 'pip'` | 用了坏的 `pip` 命令 | 只跑 `bash setup_venv.sh` |
| `externally-managed-environment` | 往系统 Python 装包 | 只用 `.venv`，不要 `python3 -m pip install` 到系统 |
| `Incorrect padding` | uploadToken 的 Base64 少了末尾 `=` | 用最新 `qiniu_resumable_auth.py` **重新生成** session.json，不要用旧 token |

---

## 可选：激活虚拟环境（少打字）

```bash
source .venv/bin/activate
python qiniu_resumable_auth.py ...
python qiniu_resumable_upload.py --session-file session.json
```

退出虚拟环境：`deactivate`

---

## 三步记忆

```text
cd projects/qiniu-uploader/resumable
bash setup_venv.sh          # 一次
.venv/bin/python ...auth... > session.json
.venv/bin/python ...upload... --session-file session.json
```
