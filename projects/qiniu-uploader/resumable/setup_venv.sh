#!/usr/bin/env bash
# 在 resumable 目录下执行一次：创建虚拟环境并安装 qiniu SDK。
set -euo pipefail
cd "$(dirname "$0")"
if [[ ! -d .venv ]]; then
  echo "创建虚拟环境 .venv ..."
  python3 -m venv .venv
fi
echo "安装依赖 ..."
.venv/bin/pip install -q -r requirements.txt
.venv/bin/python -c "from qiniu.services.storage.uploaders import ResumeUploaderV2; print('qiniu SDK 已就绪')"
echo "完成。之后请用：source .venv/bin/activate  或  .venv/bin/python <脚本>"
