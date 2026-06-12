#!/usr/bin/env python3
"""
七牛断点续传版下载示例。

说明：
- 下载链路和普通版差别不大
- 这里保留为独立脚本，便于与 resumable 目录下的 auth/upload 示例配套
"""

from __future__ import annotations

import argparse
import pathlib
import urllib.request


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Download a file from Qiniu to local path.")
    parser.add_argument("--url", required=True, help="文件下载地址")
    parser.add_argument("--output", required=True, help="本地保存路径")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    output = pathlib.Path(args.output).expanduser().resolve()
    output.parent.mkdir(parents=True, exist_ok=True)

    with urllib.request.urlopen(args.url) as response:
        output.write_bytes(response.read())

    print(f"下载完成：{output}")
    print(f"来源地址：{args.url}")
    return 0


if __name__ == "__main__":
    import sys

    raise SystemExit(main(sys.argv[1:]))
