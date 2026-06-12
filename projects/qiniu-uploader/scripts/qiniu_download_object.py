#!/usr/bin/env python3
"""
从七牛下载文件到本地。

支持两种输入方式：
1. 直接传完整 URL：--url
2. 传公开域名 + key：--base-url + --key

如果是私有空间，可加 --private 并提供 AK/SK 自动签名下载 URL。
"""

from __future__ import annotations

import argparse
import pathlib
import urllib.request

from qiniu_check_object_exists import build_private_download_url, join_base_url_and_key, normalize_url_for_request


def resolve_target_url(args: argparse.Namespace) -> str:
    """根据参数得到最终下载 URL。"""
    if args.url:
        return normalize_url_for_request(args.url)
    if args.private:
        return build_private_download_url(args.base_url, args.key, args.access_key, args.secret_key, args.expires_in)
    return join_base_url_and_key(args.base_url, args.key)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    """解析命令行参数。"""
    parser = argparse.ArgumentParser(description="Download a Qiniu object to a local file.")
    parser.add_argument("--url", help="直接下载完整 URL")
    parser.add_argument("--base-url", help="绑定到 bucket 的访问域名，例如 https://cdn.example.com")
    parser.add_argument("--key", help="对象 key，例如 videos/task123/a.mp4")
    parser.add_argument("--private", action="store_true", help="按私有空间 URL 生成规则签名")
    parser.add_argument("--access-key", help="私有空间下载签名用的 AK")
    parser.add_argument("--secret-key", help="私有空间下载签名用的 SK")
    parser.add_argument("--expires-in", type=int, default=600, help="私有下载 URL 的有效期（秒）")
    parser.add_argument("--output", required=True, help="下载后的本地保存路径")
    args = parser.parse_args(argv)

    if not args.url and not (args.base_url and args.key):
        parser.error("请传 --url，或者同时传 --base-url 和 --key")
    if args.private and not (args.access_key and args.secret_key and args.base_url and args.key):
        parser.error("--private 模式下必须同时提供 --access-key、--secret-key、--base-url、--key")
    return args


def main(argv: list[str] | None = None) -> int:
    """命令行入口：把七牛对象下载到本地。"""
    args = parse_args(argv)
    target_url = resolve_target_url(args)
    output_path = pathlib.Path(args.output).expanduser().resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with urllib.request.urlopen(target_url) as response:
        output_path.write_bytes(response.read())

    print(f"下载完成：{output_path}")
    print(f"来源地址：{target_url}")
    return 0


if __name__ == "__main__":
    import sys

    raise SystemExit(main(sys.argv[1:]))
