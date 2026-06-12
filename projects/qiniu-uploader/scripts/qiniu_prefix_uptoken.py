#!/usr/bin/env python3
# 上面这行叫 shebang：告诉操作系统用 python3 来执行本文件（直接 ./xxx.py 时有用）。
"""
本地生成七牛云前缀上传 token，不依赖第三方 SDK。

这个脚本适合后端或本地调试阶段使用，作用是：
1. 按七牛要求拼出上传策略（put policy）
2. 用 AK/SK 对策略做签名
3. 输出前端可直接使用的上传会话 JSON

最常见的执行方式：

  python3 qiniu_prefix_uptoken.py \
    --access-key "$QINIU_ACCESS_KEY" \
    --secret-key "$QINIU_SECRET_KEY" \
    --bucket media-prod \
    --key-prefix videos/task123/device456/ \
    --upload-url https://upload.qiniup.com \
    --expires-in 1800 \
    --mime-limit 'video/*'
"""

# 启用「延迟注解」：类型标注里的 str | None 等写法在旧版 Python 也能用。
from __future__ import annotations

import argparse  # 解析命令行参数（--bucket、--key-prefix 等）。
import base64  # Base64 编解码，七牛 token 里要用 URL 安全变体。
import hashlib  # 提供 sha1 等哈希算法，供 HMAC 签名使用。
import hmac  # 用 SecretKey 对策略做 HMAC-SHA1 签名。
import json  # 把策略 dict 转成 JSON 字符串再参与签名。
import sys  # 访问标准输入输出、命令行参数 sys.argv。
import time  # 获取当前 Unix 时间戳，用于计算 token 过期时间。
from typing import Any  # Any 表示「任意类型」，用于宽松的 dict 类型标注。


def _urlsafe_base64(data: bytes) -> str:
    """
    把二进制内容转成七牛/URL 可安全传输的 Base64 字符串。

    参数：
    - data: 待编码的原始字节（例如 JSON 策略或签名结果）。
    """
    # urlsafe_b64encode：用 - 和 _ 代替 + 和 /，避免在 URL 里出问题。
    # decode("utf-8")：把 bytes 转成 str，方便和 AK 拼成 token 字符串。
    #
    # 注意：这里不要去掉末尾的 '=' padding。
    # 七牛官方 Python SDK 的 urlsafe_base64_encode 会保留 '='，
    # 服务端验签时会按「原样 Base64 串」参与计算；如果本地擅自去掉 padding，
    # 会导致签名不一致，从而出现 401 BadToken。
    return base64.urlsafe_b64encode(data).decode("utf-8")


def build_put_policy(
    *,
    bucket: str,
    key_prefix: str,
    deadline: int,
    mime_limit: str | None = None,
    fsize_limit: int | None = None,
    callback_url: str | None = None,
    callback_body: str | None = None,
    callback_body_type: str | None = None,
    insert_only: bool = False,
) -> dict[str, Any]:
    """
    生成七牛「上传策略」（put policy）字典，后续会参与签名变成 upload token。

    参数（带 * 表示必须用「关键字参数」传入，不能按位置乱传）：
    - bucket: 七牛存储空间名称，例如 media-prod。
    - key_prefix: 允许上传的对象 key 前缀，必须以 / 结尾表示目录，例如 videos/task123/。
    - deadline: token 失效时刻，Unix 时间戳（秒），必须大于当前时间。
    - mime_limit: 可选，限制 MIME 类型，例如 video/*；不传则不限制类型。
    - fsize_limit: 可选，单文件最大字节数；不传则不限制大小。
    - callback_url: 可选，上传成功后七牛服务端 POST 回调的 URL。
    - callback_body: 可选，回调请求体模板，可用 $(bucket)、$(key) 等占位符。
    - callback_body_type: 可选，回调 Content-Type，例如 application/x-www-form-urlencoded。
    - insert_only: 为 True 时只允许新增，同名 key 已存在则拒绝覆盖。
    """
    # 先构造七牛要求的必填字段；policy 是普通的 Python 字典。
    policy: dict[str, Any] = {
        # scope：上传范围，格式为「bucket:前缀」，表示只能传到该 bucket 下该前缀路径。
        "scope": f"{bucket}:{key_prefix}",
        # isPrefixalScope=1：告诉七牛 scope 里是「目录前缀」而不是单个完整文件名。
        "isPrefixalScope": 1,
        # deadline：策略过期时间，七牛收到上传请求时会校验是否已过期。
        "deadline": deadline,
    }

    # 以下字段均为可选；只有调用方传了值才写入 policy，否则七牛侧视为无此项限制。
    if mime_limit:
        # mimeLimit：只允许指定 MIME 的文件，例如 video/* 表示各类视频。
        policy["mimeLimit"] = mime_limit
    if fsize_limit is not None:
        # 用 is not None 是因为 0 也是合法限制值，不能只写 if fsize_limit。
        # fsizeLimit：单文件最大字节数。
        policy["fsizeLimit"] = fsize_limit
    if callback_url:
        # callbackUrl：上传成功后的服务端回调地址。
        policy["callbackUrl"] = callback_url
    if callback_body:
        # callbackBody：回调时七牛发送的 body 内容模板。
        policy["callbackBody"] = callback_body
    if callback_body_type:
        # callbackBodyType：回调请求的 Content-Type。
        policy["callbackBodyType"] = callback_body_type
    if insert_only:
        # insertOnly=1：禁止覆盖已存在的同名对象。
        policy["insertOnly"] = 1

    return policy


def generate_upload_token(*, access_key: str, secret_key: str, policy: dict[str, Any]) -> str:
    """
    根据上传策略生成七牛 upload token 字符串。

    参数：
    - access_key: 七牛 AccessKey（AK），会出现在 token 第一段，用于标识账号。
    - secret_key: 七牛 SecretKey（SK），只用于本地签名，绝不能发给前端以外的不可信方。
    - policy: build_put_policy 返回的策略字典。

    返回值格式：access_key:encoded_sign:encoded_policy（三段用冒号连接）。
    """
    # 把策略 dict 压成紧凑 JSON（无多余空格），再转成 utf-8 字节供 Base64 编码。
    policy_json = json.dumps(policy, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    # encoded_policy：策略的 URL 安全 Base64，是 token 的第三段。
    encoded_policy = _urlsafe_base64(policy_json)

    # 用 SK 对 encoded_policy 字符串做 HMAC-SHA1，得到原始签名字节。
    sign = hmac.new(secret_key.encode("utf-8"), encoded_policy.encode("utf-8"), hashlib.sha1).digest()
    # encoded_sign：签名的 URL 安全 Base64，是 token 的第二段。
    encoded_sign = _urlsafe_base64(sign)
    # 按七牛规定用冒号拼接三段，得到完整 upload token。
    return f"{access_key}:{encoded_sign}:{encoded_policy}"


def build_upload_session(
    *,
    access_key: str,
    secret_key: str,
    bucket: str,
    key_prefix: str,
    upload_url: str,
    expires_in: int,
    mime_limit: str | None = None,
    fsize_limit: int | None = None,
    callback_url: str | None = None,
    callback_body: str | None = None,
    callback_body_type: str | None = None,
    insert_only: bool = False,
) -> dict[str, Any]:
    """
    生成给前端直接使用的「上传会话」JSON 对象（dict，由调用方再 json.dump）。

    参数：
    - access_key / secret_key: 七牛 AK/SK，用于生成 uploadToken。
    - bucket: 存储空间名。
    - key_prefix: 允许上传的 key 前缀目录。
    - upload_url: 表单上传 POST 的目标地址，例如 https://upload.qiniup.com。
    - expires_in: 从「现在」起多少秒后 token 失效（秒），会换算成 policy 里的 deadline。
    - mime_limit / fsize_limit / callback_* / insert_only: 同 build_put_policy，会透传进策略。
    """
    # 当前 Unix 秒 + 有效时长 = 策略里的 deadline 时间戳。
    deadline = int(time.time()) + expires_in
    # 根据 bucket、前缀、deadline 及可选约束拼出七牛 put policy。
    policy = build_put_policy(
        bucket=bucket,
        key_prefix=key_prefix,
        deadline=deadline,
        mime_limit=mime_limit,
        fsize_limit=fsize_limit,
        callback_url=callback_url,
        callback_body=callback_body,
        callback_body_type=callback_body_type,
        insert_only=insert_only,
    )
    # 用 AK/SK 对 policy 签名，得到前端表单字段 token 所需的 uploadToken。
    token = generate_upload_token(access_key=access_key, secret_key=secret_key, policy=policy)

    # session：与前端 TypeScript 约定一致的字段名（驼峰）。
    session: dict[str, Any] = {
        "provider": "qiniu",  # 标识对象存储厂商。
        "mode": "form_post",  # 上传方式：multipart 表单 POST。
        "uploadUrl": upload_url,  # 浏览器/客户端 POST 文件的目标 URL。
        "uploadToken": token,  # 表单里的 token 字段值。
        "keyPrefix": key_prefix,  # 提醒前端：对象 key 必须落在此前缀下。
        "expiresAt": deadline,  # 与 policy.deadline 相同，便于前端展示倒计时。
        "expiresInSec": expires_in,  # 签发时约定的有效秒数（相对时长）。
    }

    # constraints：仅在有 MIME 或大小限制时附带，避免前端误以为每项都必填。
    constraints: dict[str, Any] = {}
    if mime_limit is not None:
        constraints["mimeLimit"] = mime_limit
    if fsize_limit is not None:
        constraints["maxFileSize"] = fsize_limit
    # 只有 constraints 非空时才挂到 session 上。
    if constraints:
        session["constraints"] = constraints

    return session


def parse_args(argv: list[str]) -> argparse.Namespace:
    """
    解析命令行参数，返回包含各选项属性的 Namespace 对象。

    参数：
    - argv: 参数列表，通常不含脚本名；main 里会传 sys.argv[1:]。
    """
    # 创建参数解析器；description 会在 python3 xxx.py -h 时显示。
    parser = argparse.ArgumentParser(
        description="Generate a prefix-scoped Qiniu upload session as JSON.",
    )
    # required=True：命令行必须提供，否则报错退出。
    parser.add_argument("--access-key", required=True, help="七牛 AccessKey（AK）")
    parser.add_argument("--secret-key", required=True, help="七牛 SecretKey（SK）")
    parser.add_argument("--bucket", required=True, help="七牛存储空间名称")
    parser.add_argument("--key-prefix", required=True, help="允许上传到的对象前缀，例如 videos/task123/")
    parser.add_argument(
        "--upload-url",
        default="https://upload.qiniup.com",  # 未传时使用华东公共上传域名。
        help="七牛表单上传地址，默认是华东机房公共上传域名",
    )
    parser.add_argument(
        "--expires-in",
        type=int,  # 把字符串参数转成整数。
        default=1800,  # 默认 30 分钟有效。
        help="token 有效期（秒），默认 1800 秒，也就是 30 分钟",
    )
    parser.add_argument("--mime-limit", help="限制允许上传的 MIME 类型，例如 video/*")
    parser.add_argument("--fsize-limit", type=int, help="限制单文件最大字节数，例如 2147483648")
    parser.add_argument("--callback-url", help="上传成功后，七牛服务端回调到你的地址")
    parser.add_argument("--callback-body", help="七牛回调时发送的 body 模板")
    parser.add_argument(
        "--callback-body-type",
        help="七牛回调时 body 的类型，例如 application/x-www-form-urlencoded",
    )
    parser.add_argument(
        "--insert-only",
        action="store_true",  # 命令行出现该 flag 时值为 True，不出现则为 False。
        help="只允许新增，不允许覆盖同名文件",
    )
    # 根据 argv 解析并返回结果对象，属性名由 --xxx-yyy 转成 xxx_yyy。
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    """
    命令行入口：解析参数、生成 session、打印 JSON 到标准输出。

    参数：
    - argv: 可选，自定义参数列表；为 None 时使用 sys.argv[1:]（去掉脚本名）。

    返回值：0 表示成功（给 SystemExit 用）。
    """
    # 若未传入 argv，则使用真实命令行参数（跳过第 0 项脚本路径）。
    args = parse_args(argv or sys.argv[1:])
    # 用解析结果调用 build_upload_session，字段一一对应 args 上的属性。
    session = build_upload_session(
        access_key=args.access_key,
        secret_key=args.secret_key,
        bucket=args.bucket,
        key_prefix=args.key_prefix,
        upload_url=args.upload_url,
        expires_in=args.expires_in,
        mime_limit=args.mime_limit,
        fsize_limit=args.fsize_limit,
        callback_url=args.callback_url,
        callback_body=args.callback_body,
        callback_body_type=args.callback_body_type,
        insert_only=args.insert_only,
    )
    # indent=2：美化 JSON；ensure_ascii=False：中文等字符原样输出。
    json.dump(session, sys.stdout, indent=2, ensure_ascii=False)
    # 末尾补换行，符合 Unix 文本行惯例。
    sys.stdout.write("\n")
    return 0


# 仅当「直接运行本文件」时执行（被 import 时不会跑 main）。
if __name__ == "__main__":
    # SystemExit(main())：把 main 的返回码作为进程退出码（0=成功）。
    raise SystemExit(main())
