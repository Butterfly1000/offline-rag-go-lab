import base64
import hashlib
import hmac
import json
import time

_MAX_FILE_SIZE = 5368709120
_EXPIRES_IN_SEC = 1800


def _urlsafe_base64(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("utf-8")


def _build_put_policy(bucket: str, deadline: int) -> dict:
    return {
        "scope": bucket,
        "deadline": deadline,
        "fsizeLimit": _MAX_FILE_SIZE,
    }


def _generate_upload_token(access_key: str, secret_key: str, policy: dict) -> str:
    encoded_policy = _urlsafe_base64(
        json.dumps(policy, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    )
    sign = hmac.new(
        secret_key.encode("utf-8"),
        encoded_policy.encode("utf-8"),
        hashlib.sha1,
    ).digest()
    encoded_sign = _urlsafe_base64(sign)
    return "{}:{}:{}".format(access_key, encoded_sign, encoded_policy)


def main(
    access_key: str,
    secret_key: str,
    bucket: str,
):
    """
    Generate Qiniu uploadToken.
    """
    response = {
        "UploadToken": "",
        "Hint": "",
        "Retryable": False,
        "StatusCode": 200,
        "ErrorMessage": "",
    }

    if not isinstance(access_key, str) or not access_key.strip():
        response.update(
            {
                "StatusCode": -1,
                "ErrorMessage": "AccessKey is required.",
                "Hint": "Configure the Qiniu credential (AccessKey) and retry.",
            }
        )
        return response

    if not isinstance(secret_key, str) or not secret_key.strip():
        response.update(
            {
                "StatusCode": -1,
                "ErrorMessage": "SecretKey is required.",
                "Hint": "Configure the Qiniu credential (SecretKey) and retry.",
            }
        )
        return response

    if not isinstance(bucket, str) or not bucket.strip():
        response.update(
            {
                "StatusCode": -1,
                "ErrorMessage": "Bucket is required.",
                "Hint": "Provide the Qiniu bucket name, then retry.",
            }
        )
        return response

    bucket = bucket.strip()
    access_key = access_key.strip()
    secret_key = secret_key.strip()

    deadline = int(time.time()) + _EXPIRES_IN_SEC

    try:
        token = _generate_upload_token(
            access_key,
            secret_key,
            _build_put_policy(bucket, deadline),
        )
        response["UploadToken"] = token
        response["ErrorMessage"] = ""
        return response

    except Exception as e:
        response.update(
            {
                "StatusCode": 500,
                "ErrorMessage": "Unexpected error: {}".format(str(e)[:500]),
                "Hint": "Verify Qiniu credential and input parameters, then retry; if unresolved, report this tool issue.",
                "Retryable": False,
            }
        )
        return response
