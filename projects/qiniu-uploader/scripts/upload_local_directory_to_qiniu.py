#!/usr/bin/env python3
"""
按本地目录批量上传文件到七牛。

这个脚本的定位是“客户端上传器”：
1. 服务端先生成 upload session JSON
2. app / 客户端拿到这个 JSON
3. 再额外传一个 local-dir
4. 脚本读取本地目录并逐个上传

注意：
- 这个脚本不负责生成 token
- 这个脚本不需要 AK / SK
- AK / SK 应该只保留在服务端
"""

from __future__ import annotations

import argparse
import json
import mimetypes
import pathlib
import sys
import uuid
import urllib.error
import urllib.request
from typing import Any


def normalize_prefix(prefix: str) -> str:
    """保证 key 前缀以 / 结尾，避免字符串拼接出错。"""
    return prefix if prefix.endswith("/") else f"{prefix}/"


def safe_file_name(file_name: str) -> str:
    """把文件名里不够安全的字符替换掉，减少 key 异常情况。"""
    return "".join(char if char.isalnum() or char in "._-()" else "_" for char in file_name)


def build_object_key(key_prefix: str, file_path: pathlib.Path) -> str:
    """把七牛目录前缀和本地文件名拼成最终对象 key。"""
    return f"{normalize_prefix(key_prefix)}{safe_file_name(file_path.name)}"


def load_upload_session(*, session_file: str | None, session_json: str | None) -> dict[str, Any]:
    """
    读取服务端下发的 upload session。

    支持两种传法：
    - --session-file: 传一个 JSON 文件路径
    - --session-json: 直接传一个 JSON 字符串
    """
    if bool(session_file) == bool(session_json):
        raise SystemExit("请二选一传入 --session-file 或 --session-json")

    if session_file:
        payload = pathlib.Path(session_file).expanduser().resolve().read_text(encoding="utf-8")
    else:
        payload = session_json or ""

    try:
        session = json.loads(payload)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"upload session 不是合法 JSON：{exc}") from exc

    required_fields = ["provider", "mode", "uploadUrl", "uploadToken", "keyPrefix", "expiresAt", "expiresInSec"]
    missing_fields = [field for field in required_fields if field not in session]
    if missing_fields:
        raise SystemExit(f"upload session 缺少必要字段：{', '.join(missing_fields)}")

    if session["provider"] != "qiniu":
        raise SystemExit(f"当前只支持 provider=qiniu，实际收到：{session['provider']}")
    if session["mode"] != "form_post":
        raise SystemExit(f"当前只支持 mode=form_post，实际收到：{session['mode']}")

    return session


def collect_files(directory: pathlib.Path, recursive: bool) -> list[pathlib.Path]:
    """
    收集目录里的文件。

    - recursive=False: 只拿当前目录第一层文件
    - recursive=True: 递归拿全部子目录文件
    """
    if recursive:
        candidates = directory.rglob("*")
    else:
        candidates = directory.iterdir()
    return sorted(path for path in candidates if path.is_file())


def build_multipart_body(token: str, key: str, file_path: pathlib.Path) -> tuple[bytes, str]:
    """手动构造 multipart/form-data 请求体，避免额外依赖 requests。"""
    boundary = f"----CodexQiniuBoundary{uuid.uuid4().hex}"
    content_type = mimetypes.guess_type(file_path.name)[0] or "application/octet-stream"
    file_bytes = file_path.read_bytes()

    parts: list[bytes] = []

    def add_text_field(name: str, value: str) -> None:
        parts.append(f"--{boundary}\r\n".encode("utf-8"))
        parts.append(f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode("utf-8"))
        parts.append(value.encode("utf-8"))
        parts.append(b"\r\n")

    add_text_field("token", token)
    add_text_field("key", key)

    parts.append(f"--{boundary}\r\n".encode("utf-8"))
    parts.append(
        (
            f'Content-Disposition: form-data; name="file"; filename="{file_path.name}"\r\n'
            f"Content-Type: {content_type}\r\n\r\n"
        ).encode("utf-8")
    )
    parts.append(file_bytes)
    parts.append(b"\r\n")
    parts.append(f"--{boundary}--\r\n".encode("utf-8"))

    return b"".join(parts), boundary


def upload_single_file(upload_url: str, upload_token: str, key: str, file_path: pathlib.Path) -> dict[str, Any]:
    """上传单个文件到七牛并返回结果。"""
    body, boundary = build_multipart_body(upload_token, key, file_path)
    request = urllib.request.Request(
        upload_url,
        data=body,
        method="POST",
        headers={
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "Content-Length": str(len(body)),
        },
    )

    try:
        with urllib.request.urlopen(request) as response:
            response_text = response.read().decode("utf-8", errors="replace")
            return {
                "fileName": file_path.name,
                "localPath": str(file_path),
                "key": key,
                "ok": True,
                "status": response.status,
                "responseText": response_text,
            }
    except urllib.error.HTTPError as exc:
        return {
            "fileName": file_path.name,
            "localPath": str(file_path),
            "key": key,
            "ok": False,
            "status": exc.code,
            "responseText": exc.read().decode("utf-8", errors="replace"),
        }


def validate_file_against_constraints(file_path: pathlib.Path, session: dict[str, Any]) -> None:
    """根据服务端 session 里的 constraints 做本地预校验。"""
    constraints = session.get("constraints") or {}
    max_file_size = constraints.get("maxFileSize")
    if max_file_size is not None and file_path.stat().st_size > max_file_size:
        raise ValueError(f"文件过大，超过 maxFileSize 限制：{file_path}")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    """解析命令行参数。"""
    parser = argparse.ArgumentParser(description="Upload files from a local directory to Qiniu.")
    parser.add_argument("--local-dir", required=True, help="本地目录路径，脚本会读取其中的文件")
    parser.add_argument("--session-file", help="服务端下发的 upload session JSON 文件路径")
    parser.add_argument("--session-json", help="服务端下发的 upload session JSON 字符串")
    parser.add_argument("--recursive", action="store_true", help="递归扫描子目录")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    """命令行入口：读取服务端 session，扫描目录，上传全部文件。"""
    args = parse_args(argv)
    local_dir = pathlib.Path(args.local_dir).expanduser().resolve()

    if not local_dir.exists():
        raise SystemExit(f"本地目录不存在：{local_dir}")
    if not local_dir.is_dir():
        raise SystemExit(f"传入的路径不是目录：{local_dir}")

    files = collect_files(local_dir, recursive=args.recursive)
    if not files:
        raise SystemExit(f"目录里没有可上传文件：{local_dir}")

    session = load_upload_session(session_file=args.session_file, session_json=args.session_json)

    results = []
    for file_path in files:
        key = build_object_key(session["keyPrefix"], file_path)
        try:
            validate_file_against_constraints(file_path, session)
        except ValueError as exc:
            results.append(
                {
                    "fileName": file_path.name,
                    "localPath": str(file_path),
                    "key": key,
                    "ok": False,
                    "status": "local_validation_failed",
                    "responseText": str(exc),
                }
            )
            continue
        results.append(upload_single_file(session["uploadUrl"], session["uploadToken"], key, file_path))

    json.dump(
        {
            "localDir": str(local_dir),
            "fileCount": len(files),
            "session": {
                "uploadUrl": session["uploadUrl"],
                "keyPrefix": session["keyPrefix"],
                "expiresAt": session["expiresAt"],
            },
            "results": results,
        },
        fp=sys.stdout,
        indent=2,
        ensure_ascii=False,
    )
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
