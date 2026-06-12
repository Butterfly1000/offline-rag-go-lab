#!/usr/bin/env python3
"""
检查七牛文件是否存在。

支持两种输入方式：
1. 直接传完整 URL：--url
2. 传公开域名 + key：--base-url + --key

如果是私有空间，还可以加 AK/SK 生成带时效的私有下载 URL 后再检查。
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import time
import urllib.error
import urllib.parse
import urllib.request


def urlsafe_base64(data: bytes) -> str:
    """生成七牛私有下载链接所需的 URL 安全 Base64。"""
    return base64.urlsafe_b64encode(data).decode("utf-8").rstrip("=")


def normalize_url_for_request(url: str) -> str:
    """
    把可能包含中文等非 ASCII 字符的 URL 规范化为可发 HTTP 请求的形式。

    urllib/http.client 在拼请求行时要求 URL 必须是 ASCII；因此路径和查询参数必须做百分号编码。
    """
    parts = urllib.parse.urlsplit(url)
    encoded_path = urllib.parse.quote(parts.path, safe="/%")
    encoded_query = urllib.parse.quote_plus(parts.query, safe="=&%")
    return urllib.parse.urlunsplit((parts.scheme, parts.netloc, encoded_path, encoded_query, parts.fragment))


def join_base_url_and_key(base_url: str, key: str) -> str:
    """把访问域名和对象 key 组合成可访问 URL。"""
    normalized_base = base_url.rstrip("/")
    encoded_key = "/".join(urllib.parse.quote(part) for part in key.split("/"))
    return f"{normalized_base}/{encoded_key}"


def build_private_download_url(base_url: str, key: str, access_key: str, secret_key: str, expires_in: int) -> str:
    """按七牛私有空间规则生成带签名的下载 URL。"""
    deadline = int(time.time()) + expires_in
    public_url = join_base_url_and_key(base_url, key)
    separator = "&" if "?" in public_url else "?"
    url_with_deadline = f"{public_url}{separator}e={deadline}"
    sign = hmac.new(secret_key.encode("utf-8"), url_with_deadline.encode("utf-8"), hashlib.sha1).digest()
    token = f"{access_key}:{urlsafe_base64(sign)}"
    return f"{url_with_deadline}&token={token}"


def resolve_target_url(args: argparse.Namespace) -> str:
    """根据命令行参数，得到最终要检查的 URL。"""
    if args.url:
        return normalize_url_for_request(args.url)
    if args.private:
        return build_private_download_url(args.base_url, args.key, args.access_key, args.secret_key, args.expires_in)
    return join_base_url_and_key(args.base_url, args.key)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    """解析命令行参数。"""
    parser = argparse.ArgumentParser(description="Check whether a Qiniu object URL exists.")
    parser.add_argument("--url", help="直接检查完整 URL")
    parser.add_argument("--base-url", help="绑定到 bucket 的访问域名，例如 https://cdn.example.com")
    parser.add_argument("--key", help="对象 key，例如 videos/task123/a.mp4")
    parser.add_argument("--private", action="store_true", help="按私有空间 URL 生成规则签名")
    parser.add_argument("--access-key", help="私有空间下载签名用的 AK")
    parser.add_argument("--secret-key", help="私有空间下载签名用的 SK")
    parser.add_argument("--expires-in", type=int, default=600, help="私有下载 URL 的有效期（秒）")
    args = parser.parse_args(argv)

    if not args.url and not (args.base_url and args.key):
        parser.error("请传 --url，或者同时传 --base-url 和 --key")
    if args.private and not (args.access_key and args.secret_key and args.base_url and args.key):
        parser.error("--private 模式下必须同时提供 --access-key、--secret-key、--base-url、--key")
    return args


def main(argv: list[str] | None = None) -> int:
    """命令行入口：请求 URL，并返回是否存在。"""
    args = parse_args(argv)
    target_url = resolve_target_url(args)
    request = urllib.request.Request(target_url, method="HEAD")

    result = {"exists": False, "status": None, "url": target_url}
    try:
        with urllib.request.urlopen(request) as response:
            result["exists"] = 200 <= response.status < 400
            result["status"] = response.status
    except urllib.error.HTTPError as exc:
        result["exists"] = False
        result["status"] = exc.code
    except urllib.error.URLError as exc:
        result["exists"] = False
        result["status"] = str(exc.reason)

    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["exists"] else 1


if __name__ == "__main__":
    import sys

    raise SystemExit(main(sys.argv[1:]))
