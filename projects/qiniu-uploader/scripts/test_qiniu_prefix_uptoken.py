# 标准库：json 用于验证策略能否序列化；pathlib 解析文件路径；sys 改 import 路径。
import json
import pathlib
import sys
import tempfile
import time  # 生成与真实脚本一致的 deadline 时间戳。
import unittest  # Python 内置单元测试框架。

# 把「本测试文件所在目录」插入 sys.path 最前面，这样才能 import 同目录的 qiniu_prefix_uptoken。
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

# 从被测模块导入三个核心函数：拼策略、签 token、拼上传会话。
from qiniu_prefix_uptoken import build_put_policy, build_upload_session, generate_upload_token
from upload_local_directory_to_qiniu import load_upload_session


class QiniuPrefixUploadTokenTests(unittest.TestCase):
    """覆盖上传策略、token 结构、上传会话结构这 3 类核心行为。"""

    def test_build_put_policy_sets_prefix_scope(self) -> None:
        """确认策略里写入了「前缀上传」所必需的字段。"""
        # 模拟「当前时间 + 1800 秒」作为过期时刻，与生产脚本默认 expires-in 一致。
        deadline = int(time.time()) + 1800
        # 调用 build_put_policy，传入 bucket、前缀、deadline 及全部可选约束，检查返回值字段。
        policy = build_put_policy(
            bucket="media-prod",  # 存储空间名。
            key_prefix="videos202606/task123/device456/",  # 允许上传的目录前缀。
            deadline=deadline,  # 策略过期 Unix 时间戳。
            # mime_limit="video/*",  # 只允许视频 MIME。
            mime_limit=None,  # 不限制 MIME。
            fsize_limit=5 * 1024 * 1024 * 1024,  # 最大 5 GiB。
            # callback_url="https://example.com/callback",  # 上传成功回调 URL。
            # callback_body="bucket=$(bucket)&key=$(key)",  # 回调 body 模板。
            callback_url=None,  # 上传成功回调 URL。
            callback_body=None,  # 回调 body 模板。
            insert_only=True,  # 禁止覆盖同名文件。
        )

        # scope 必须是「bucket:前缀」格式。
        self.assertEqual(policy["scope"], "media-prod:videos202606/task123/device456/")
        # 前缀上传必须带 isPrefixalScope=1。
        self.assertEqual(policy["isPrefixalScope"], 1)
        # deadline 应与传入值完全一致。
        self.assertEqual(policy["deadline"], deadline)
        # 可选字段应原样出现在 policy 里（键名是七牛 JSON 驼峰）。
        # self.assertEqual(policy["mimeLimit"], "video/*")
        # self.assertEqual(policy["mimeLimit"], None)
        self.assertEqual(policy["fsizeLimit"], 5 * 1024 * 1024 * 1024)
        # self.assertEqual(policy["callbackUrl"], "https://example.com/callback")
        # self.assertEqual(policy["callbackBody"], "bucket=$(bucket)&key=$(key)")
        self.assertEqual(policy["insertOnly"], 1)

    def test_generate_upload_token_has_three_parts(self) -> None:
        """七牛 upload token 由 3 段组成，第一段必须是 AK。"""
        deadline = int(time.time()) + 1800
        # generate_upload_token 需要 AK、SK 和已拼好的 policy；policy 用 build_put_policy 内联生成。
        token = generate_upload_token(
            access_key="test-ak",  # 测试用 AK，应出现在 token 第一段。
            secret_key="test-sk",  # 测试用 SK，只参与签名。
            policy=build_put_policy(
                bucket="media-prod",
                key_prefix="videos202606/task123/device456/",
                deadline=deadline,
            ),
        )

        # 七牛 token 格式为 AK:sign:policy，用冒号拆成 3 段。
        parts = token.split(":")
        self.assertEqual(len(parts), 3)
        # 第一段必须是 AccessKey。
        self.assertEqual(parts[0], "test-ak")

    def test_policy_is_json_serializable(self) -> None:
        """策略最终要参与签名，所以必须能稳定转成 JSON。"""
        deadline = int(time.time()) + 1800
        policy = build_put_policy(
            bucket="media-prod",
            key_prefix="videos202606/task123/device456/",
            deadline=deadline,
            # mime_limit="video/*",
            mime_limit=None,
        )

        # sort_keys=True 固定键顺序，便于断言 JSON 字符串里是否包含预期片段。
        encoded = json.dumps(policy, separators=(",", ":"), sort_keys=True)
        # 签名前 policy 里 scope 字段应可被序列化为该子串。
        self.assertIn('"scope":"media-prod:videos202606/task123/device456/"', encoded)

    def test_build_upload_session_returns_frontend_payload(self) -> None:
        """确认脚本输出的会话结构和前端 TypeScript 约定一致。"""
        # build_upload_session 会内部算 deadline、调 build_put_policy 和 generate_upload_token。
        session = build_upload_session(
            access_key="test-ak",
            secret_key="test-sk",
            bucket="media-prod",
            key_prefix="videos202606/task123/device456/",
            upload_url="https://upload.qiniup.com",  # 表单 POST 目标。
            expires_in=1800,  # 相对有效秒数。
            mime_limit="video/*",
            fsize_limit=1024,  # 1 KiB，测试用小值即可。
        )

        # 以下字段名与前端约定一致，值应与传入参数或推导结果匹配。
        self.assertEqual(session["provider"], "qiniu")
        self.assertEqual(session["mode"], "form_post")
        self.assertEqual(session["uploadUrl"], "https://upload.qiniup.com")
        self.assertEqual(session["keyPrefix"], "videos202606/task123/device456/")
        self.assertEqual(session["expiresInSec"], 1800)
        # constraints 里 MIME 与大小限制应回显给前端。
        self.assertEqual(session["constraints"]["mimeLimit"], "video/*")
        self.assertEqual(session["constraints"]["maxFileSize"], 1024)
        # uploadToken 由签名生成，只断言存在即可（具体内容随时间变）。
        self.assertIn("uploadToken", session)

    def test_load_upload_session_from_json_file(self) -> None:
        """客户端上传脚本应能直接读取服务端下发的 session JSON 文件。"""
        session = {
            "provider": "qiniu",
            "mode": "form_post",
            "uploadUrl": "https://air-test-qn.airdroid.com",
            "uploadToken": "demo-token",
            "keyPrefix": "videos/task123/device456/",
            "expiresAt": 1780380873,
            "expiresInSec": 1800,
            "constraints": {"maxFileSize": 5368709120},
        }

        with tempfile.TemporaryDirectory() as tmp_dir:
            session_path = pathlib.Path(tmp_dir) / "session.json"
            session_path.write_text(json.dumps(session), encoding="utf-8")
            loaded = load_upload_session(session_file=str(session_path), session_json=None)

        self.assertEqual(loaded["provider"], "qiniu")
        self.assertEqual(loaded["uploadUrl"], "https://air-test-qn.airdroid.com")
        self.assertEqual(loaded["keyPrefix"], "videos/task123/device456/")


# 直接运行本文件时，执行 unittest 并打印通过/失败。
if __name__ == "__main__":
    unittest.main()
