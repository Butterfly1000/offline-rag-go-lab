#!/usr/bin/env python3
"""
生成七牛断点续传版上传配置。

这个脚本是服务端示例：
1. 使用 AK/SK 生成 upload token
2. 组装 daemon 需要的断点续传配置 JSON
3. 输出给客户端使用

注意：
- AK/SK 只应保留在服务端
- 客户端只接收最终下发的 JSON
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import sys
import time
from typing import Any


def urlsafe_base64(data: bytes) -> str:
    """生成七牛 token 使用的 URL 安全 Base64（须保留末尾 =，与官方 SDK 验签/解码一致）。"""
    return base64.urlsafe_b64encode(data).decode("utf-8")


def build_put_policy(
    *,
    bucket: str,
    key_prefix: str,
    deadline: int,
    fsize_limit: int | None = None,
) -> dict[str, Any]:
    """生成适用于前缀范围上传的 put policy。"""
    policy: dict[str, Any] = {
        "scope": f"{bucket}:{key_prefix}",
        "isPrefixalScope": 1,
        "deadline": deadline,
    }
    if fsize_limit is not None:
        policy["fsizeLimit"] = fsize_limit
    return policy


def generate_upload_token(*, access_key: str, secret_key: str, policy: dict[str, Any]) -> str:
    """根据 put policy 生成 upload token。"""
    encoded_policy = urlsafe_base64(json.dumps(policy, separators=(",", ":"), ensure_ascii=False).encode("utf-8"))
    sign = hmac.new(secret_key.encode("utf-8"), encoded_policy.encode("utf-8"), hashlib.sha1).digest()
    encoded_sign = urlsafe_base64(sign)
    return f"{access_key}:{encoded_sign}:{encoded_policy}"


def build_resumable_session(
    *,
    pid: str,
    access_key: str,
    secret_key: str,
    bucket: str,
    key_prefix: str,
    local_dir: str,
    file_name_pattern: str,
    expires_in: int,
    upload_host: str | None = None,
    max_file_size: int | None = None,
) -> dict[str, Any]:
    """构建断点续传版下发配置 JSON。"""
    deadline = int(time.time()) + expires_in
    token = generate_upload_token(
        access_key=access_key,
        secret_key=secret_key,
        policy=build_put_policy(
            bucket=bucket,
            key_prefix=key_prefix,
            deadline=deadline,
            fsize_limit=max_file_size,
        ),
    )

    session: dict[str, Any] = {
        "pid": pid,
        "provider": "qiniu",
        "mode": "resumable_v2",
        "bucketName": bucket,
        "uploadToken": token,
        "keyPrefix": key_prefix,
        "expiresAt": deadline,
        "expiresInSec": expires_in,
        "localDir": local_dir,
        "fileNamePattern": file_name_pattern,
        "resumable": {
            "uploadApiVersion": "v2",
        },
    }
    if upload_host:
        session["uploadHost"] = upload_host
    if max_file_size is not None:
        session["constraints"] = {"maxFileSize": max_file_size}
    return session


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build a Qiniu resumable upload session JSON.")
    parser.add_argument("--pid", required=True, help="服务端创建的活动日志 pid")
    parser.add_argument("--access-key", required=True, help="七牛 AccessKey（只在服务端使用）")
    parser.add_argument("--secret-key", required=True, help="七牛 SecretKey（只在服务端使用）")
    parser.add_argument("--bucket", required=True, help="七牛存储空间名称")
    parser.add_argument("--key-prefix", required=True, help="上传对象前缀，例如 DeviceName/Date/")
    parser.add_argument("--local-dir", required=True, help="客户端扫描目录")
    parser.add_argument("--file-name-pattern", required=True, help="客户端文件名匹配正则")
    parser.add_argument("--expires-in", type=int, default=7200, help="upload token 有效期，默认 7200 秒")
    parser.add_argument("--upload-host", help="可选：指定上传域名")
    parser.add_argument("--max-file-size", type=int, help="可选：单文件最大大小")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    session = build_resumable_session(
        pid=args.pid,
        access_key=args.access_key,
        secret_key=args.secret_key,
        bucket=args.bucket,
        key_prefix=args.key_prefix,
        local_dir=args.local_dir,
        file_name_pattern=args.file_name_pattern,
        expires_in=args.expires_in,
        upload_host=args.upload_host,
        max_file_size=args.max_file_size,
    )
    json.dump(session, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
