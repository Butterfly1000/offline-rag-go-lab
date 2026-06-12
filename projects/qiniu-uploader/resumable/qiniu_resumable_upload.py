#!/usr/bin/env python3
"""
七牛断点续传版上传示例。

这个脚本演示 daemon 视角下“上传都做了什么”：
1. 读取服务端下发的断点续传配置
2. 扫描本地目录并按正则过滤
3. 为每个文件生成 key
4. 查找本地断点记录，判断是否恢复上传
5. 调用七牛官方 Python SDK 的 ResumeUploaderV2 上传
6. 输出每个文件的执行步骤、状态和结果

依赖：
    python3 -m pip install qiniu
"""

from __future__ import annotations

import argparse
import json
import pathlib
import re
import sys
import time
import urllib.parse
from typing import Any


def load_session(*, session_file: str | None, session_json: str | None) -> dict[str, Any]:
    """读取服务端下发的断点续传配置。"""
    if bool(session_file) == bool(session_json):
        raise SystemExit("请二选一传入 --session-file 或 --session-json")
    payload = (
        pathlib.Path(session_file).expanduser().resolve().read_text(encoding="utf-8")
        if session_file
        else session_json or ""
    )
    session = json.loads(payload)
    required = [
        "pid",
        "provider",
        "mode",
        "bucketName",
        "uploadToken",
        "keyPrefix",
        "localDir",
        "fileNamePattern",
        "resumable",
    ]
    missing = [name for name in required if name not in session]
    if missing:
        raise SystemExit(f"配置缺少必要字段：{', '.join(missing)}")
    if session["provider"] != "qiniu":
        raise SystemExit(f"当前只支持 qiniu，实际收到：{session['provider']}")
    if session["mode"] != "resumable_v2":
        raise SystemExit(f"当前只支持 resumable_v2，实际收到：{session['mode']}")
    return session


def normalize_prefix(prefix: str) -> str:
    return prefix if prefix.endswith("/") else f"{prefix}/"


def build_key(key_prefix: str, file_path: pathlib.Path) -> str:
    return f"{normalize_prefix(key_prefix)}{file_path.name}"


def collect_files(local_dir: pathlib.Path, file_name_pattern: str) -> list[pathlib.Path]:
    pattern = re.compile(file_name_pattern, re.IGNORECASE)
    return sorted(path for path in local_dir.iterdir() if path.is_file() and pattern.match(path.name))


def ensure_qiniu_sdk() -> tuple[Any, Any]:
    """按需导入七牛 SDK，缺失时给出明确提示。"""
    try:
        from qiniu import Region
        from qiniu.services.storage.uploaders import ResumeUploaderV2
    except ModuleNotFoundError as exc:
        raise SystemExit(
            "缺少 qiniu SDK。请在 file/resumable 目录执行：bash setup_venv.sh\n"
            "然后用：.venv/bin/python qiniu_resumable_upload.py ...（不要用系统 python3）。\n"
            "详见同目录 SOP.md"
        ) from exc
    return ResumeUploaderV2, Region


def resume_state_dir(session: dict[str, Any]) -> pathlib.Path:
    """断点续传演示脚本自己的本地状态目录。"""
    directory = ".qiniu_resume_records"
    path = pathlib.Path(directory).expanduser().resolve()
    path.mkdir(parents=True, exist_ok=True)
    return path


def state_file_for_key(state_dir: pathlib.Path, key: str) -> pathlib.Path:
    safe_name = key.replace("/", "__")
    return state_dir / f"{safe_name}.json"


def validate_file(file_path: pathlib.Path, session: dict[str, Any]) -> None:
    constraints = session.get("constraints") or {}
    max_file_size = constraints.get("maxFileSize")
    if max_file_size is not None and file_path.stat().st_size > max_file_size:
        raise ValueError(f"文件超过 maxFileSize：{file_path}")


def normalize_upload_host(upload_host: str) -> str:
    """
    把 uploadHost 规范成七牛 SDK 期望的 host（不含 https://）。

    传 https://up-z2.qiniup.com 时，SDK 拼 URL 会变成错误形式，导致分片初始化失败，
    进而触发 SDK 重试里对 data=None 调用 seekable() 的二次报错。
    """
    parsed = urllib.parse.urlparse(upload_host.strip())
    if parsed.netloc:
        return parsed.netloc
    return upload_host.strip().removeprefix("https://").removeprefix("http://").rstrip("/")


def build_uploader(session: dict[str, Any]) -> Any:
    """构造七牛 ResumeUploaderV2。"""
    ResumeUploaderV2, Region = ensure_qiniu_sdk()
    bucket_name = session["bucketName"]
    upload_host = session.get("uploadHost")
    if upload_host:
        host = normalize_upload_host(upload_host)
        return ResumeUploaderV2(
            bucket_name,
            regions=[Region(up_host=host)],
            preferred_scheme="https",
        )
    return ResumeUploaderV2(bucket_name, preferred_scheme="https")


def probe_initial_parts(uploader: Any, session: dict[str, Any], key: str, file_path: pathlib.Path) -> str:
    """分片初始化探测：直接调用 initial_parts，拿到七牛原始 HTTP 响应。"""
    try:
        context, resp = uploader.initial_parts(
            up_token=session["uploadToken"],
            key=key,
            file_path=str(file_path),
        )
    except Exception as exc:
        return f"initial_parts 异常：{exc}"

    if resp is None:
        return (
            f"initial_parts 无 HTTP 响应；upload_id={getattr(context, 'upload_id', '')!r}, "
            f"up_hosts={getattr(context, 'up_hosts', [])!r}"
        )

    body = getattr(resp, "text_body", None) or getattr(resp, "body", None) or ""
    if isinstance(body, bytes):
        body = body.decode("utf-8", errors="replace")
    return f"initial_parts HTTP {resp.status_code}: {body[:500]}"


def format_sdk_failure(exc: Exception) -> str:
    """把 SDK 异常转成更易读的说明。"""
    message = str(exc)
    if "seekable" in message:
        return (
            "七牛分片上传初始化失败（未拿到 uploadId），SDK 在重试时触发了二次异常。"
            "请检查：uploadHost 是否为机房域名（如 up-z2.qiniup.com，不要带 https://）、"
            "uploadToken 是否未过期、bucket 与 scope 是否一致。"
            f"原始异常：{message}"
        )
    return message


def upload_one_file(session: dict[str, Any], file_path: pathlib.Path, state_dir: pathlib.Path) -> dict[str, Any]:
    """
    上传单个文件，并把“上传过程中做了什么”写进 actions。

    注意：
    - 这里的断点记录是 daemon 侧状态记录，用于说明流程
    - 真正的分片续传能力由七牛官方 SDK 负责
    """
    uploader = build_uploader(session)
    key = build_key(session["keyPrefix"], file_path)
    local_state = state_file_for_key(state_dir, key)
    actions: list[str] = []
    started_at = int(time.time())
    file_size = file_path.stat().st_size
    is_resumed = 1 if local_state.exists() else 0
    status = 2 if is_resumed else 1

    actions.append("validated_file")
    actions.append("built_object_key")
    actions.append("checked_local_resume_record")
    if is_resumed:
        actions.append("detected_previous_interrupted_upload")
    else:
        actions.append("new_upload_started")

    # 在真正调用 SDK 前，先记录 daemon 本地状态，便于下次识别为“恢复上传”。
    local_state.write_text(
        json.dumps(
            {
                "key": key,
                "path": str(file_path),
                "status": status,
                "updatedAt": started_at,
                "uploadedSize": 0,
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    actions.append("wrote_local_resume_state")
    actions.append("calling_qiniu_resume_uploader_v2")

    try:
        ret, info = uploader.upload(
            key=key,
            file_path=str(file_path),
            up_token=session["uploadToken"],
        )
        if info is not None and hasattr(info, "status_code") and info.status_code != 200:
            raise RuntimeError(f"七牛 SDK 返回非 200：status={info.status_code}, body={info}")
        status = 3
        uploaded_size = file_size
        error_code = 0
        error_info = ""
        actions.append("sdk_upload_completed")
        if local_state.exists():
            local_state.unlink()
            actions.append("cleared_local_resume_state")
        return {
            "filename": file_path.name,
            "path": str(file_path),
            "key": key,
            "filesize": file_size,
            "uploaded_size": uploaded_size,
            "status": status,
            "upload_time": int(time.time()),
            "cost_time": int(time.time()) - started_at,
            "error_info": error_info,
            "file_type": file_path.suffix.lstrip(".").lower(),
            "cloud_type": "q",
            "error_code": error_code,
            "uploadToken": session["uploadToken"],
            "is_resumed": is_resumed,
            "actions": actions,
            "sdkRet": ret,
            "sdkInfo": str(info),
        }
    except Exception as exc:  # pragma: no cover - 依赖网络和 SDK，保留广义错误以便排查
        uploaded_size = 0
        status = 4
        error_code = getattr(exc, "code", -1)
        probe = probe_initial_parts(uploader, session, key, file_path)
        error_info = f"{format_sdk_failure(exc)} | {probe}"
        actions.append("sdk_upload_failed")
        actions.append("probed_initial_parts")
        local_state.write_text(
            json.dumps(
                {
                    "key": key,
                    "path": str(file_path),
                    "status": status,
                    "updatedAt": int(time.time()),
                    "uploadedSize": uploaded_size,
                    "errorInfo": error_info,
                },
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )
        actions.append("updated_local_resume_state_after_failure")
        return {
            "filename": file_path.name,
            "path": str(file_path),
            "key": key,
            "filesize": file_size,
            "uploaded_size": uploaded_size,
            "status": status,
            "upload_time": int(time.time()),
            "cost_time": int(time.time()) - started_at,
            "error_info": error_info,
            "file_type": file_path.suffix.lstrip(".").lower(),
            "cloud_type": "q",
            "error_code": error_code,
            "uploadToken": session["uploadToken"],
            "is_resumed": is_resumed,
            "actions": actions,
        }


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Upload files using Qiniu resumable upload v2.")
    parser.add_argument("--session-file", help="服务端下发的断点续传配置 JSON 文件")
    parser.add_argument("--session-json", help="服务端下发的断点续传配置 JSON 字符串")
    parser.add_argument("--local-dir", help="可选：覆盖 session.localDir")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    session = load_session(session_file=args.session_file, session_json=args.session_json)
    local_dir = pathlib.Path(args.local_dir or session["localDir"]).expanduser().resolve()
    if not local_dir.exists() or not local_dir.is_dir():
        raise SystemExit(f"本地目录不存在或不是目录：{local_dir}")

    files = collect_files(local_dir, session["fileNamePattern"])
    if not files:
        raise SystemExit(f"没有匹配到可上传文件：{local_dir}")

    state_dir = resume_state_dir(session)
    results = []
    for file_path in files:
        try:
            validate_file(file_path, session)
        except ValueError as exc:
            results.append(
                {
                    "filename": file_path.name,
                    "path": str(file_path),
                    "key": build_key(session["keyPrefix"], file_path),
                    "filesize": file_path.stat().st_size,
                    "uploaded_size": 0,
                    "status": 4,
                    "upload_time": int(time.time()),
                    "cost_time": 0,
                    "error_info": str(exc),
                    "file_type": file_path.suffix.lstrip(".").lower(),
                    "cloud_type": "q",
                    "error_code": -1,
                    "uploadToken": session["uploadToken"],
                    "is_resumed": 0,
                    "actions": ["validated_file", "rejected_by_max_file_size"],
                }
            )
            continue
        results.append(upload_one_file(session, file_path, state_dir))

    payload = {
        "pid": session["pid"],
        "mode": session["mode"],
        "localDir": str(local_dir),
        "resumeRecordDir": str(state_dir),
        "results": results,
    }
    json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
