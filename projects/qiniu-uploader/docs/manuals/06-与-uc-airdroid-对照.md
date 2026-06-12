# 06 — 与 uc.airdroid.com 对照

本文说明本仓库 `scripts/` 示例与生产项目 `uc.airdroid.com` 在七牛上的对应关系，便于对齐域名与 token 用法。

---

## 配置来源（示例）

见 `uc.airdroid.com/web/uc.conf.example`：

```ini
[qiniu]
url.7n air-test-qn.airdroid.com
url.7n.https https://air-test-qn.airdroid.com/
env.key.id ...
env.key.secret ...

[storage]
bucket.bizdl7n bizdl-airdroid-com
biz_dl_download_url https://bizdl.airdroid.com/
```

注意：

- **`url.7n`**：代码里 `Gen7NiuDownloadUrl(resx.Conf.QiNiuUrl, ...)` 使用的下载 host，未必等于 bizdl bucket 的绑定域名。
- **`biz_dl_download_url` / 实际 bizdl 下载**：业务上 bizdl 文件访问以 **`bizdl.airdroid.com`** 为准（与本仓库跑通一致）。
- **上传机房**：Go SDK 中华南为 `http://up-z2.qiniup.com`（`sdk/qiniu.go` 的 `ZoneHuanan`）。

---

## 上传 token

| 本仓库 | uc 项目 |
|--------|---------|
| `qiniu_prefix_uptoken.py` | `sdk.GetQiNiuUptoken` / `CommonGetUpTokenData` |
| 返回 `uploadToken` + `keyPrefix` + `uploadUrl` | 返回 `QiNiuToken{ Token, Key }` 等 |
| 前缀 scope：`bucket:prefix` + `isPrefixalScope=1` | 常见为 `Scope: bucket` + `SaveKey: key`（单 key 策略较多） |

客户端上传字段一致：表单 **`token`**、**`key`**、**`file`**。

---

## 下载 URL（私有）

| 本仓库 | uc 项目 |
|--------|---------|
| `build_private_download_url` in `qiniu_check_object_exists.py` | `sdk.Gen7NiuDownloadUrl` |
| `--base-url https://bizdl.airdroid.com` | 应对齐 bucket 绑定域名，而非随意 `url.7n` |
| `expires_in` 秒 → `e = now + expires_in` | `GetPolicy.Expires` 为**相对秒数**，内部 `deadline = now + expires` |

Go 实现要点（`github.com/qiniu/api.v6/rs`）：

```go
deadline := time.Now().Unix() + int64(expires)
baseUrl += "?e=" + strconv.FormatInt(deadline, 10)
token := digest.Sign(mac, []byte(baseUrl))
return baseUrl + "&token=" + token
```

与 Python 脚本逻辑一致。

---

## 安全模型（回答「是不是只要 key」）

| 场景 | 客户端需要什么 |
|------|----------------|
| 上传 | 短期 **uploadToken** + uploadUrl + 符合 prefix 的 key（**不要** SK） |
| 下载（私有） | 短期 **带 e&token 的完整 URL**（由服务端用 AK/SK 生成，**不要** SK） |
| 仅 key | **不够**；还需正确访问域名 +（私有）签名 |

生产做法：客户端向业务 API 要「上传 session」或「下载 URL」，服务端内置 AK/SK 签名。

---

## 回调

uc 上传 token 常带 `callbackUrl` / `callbackBody`（见 `GetQiNiuUptoken`）。  
本仓库 `qiniu_prefix_uptoken.py` 中 callback 三参数**可选**；未配置时七牛直接把 `hash`/`key` 返回给上传方。

---

## 北美临时桶（了解即可）

`uc.airdroid.com/web/common.go` 提到 `bizdl-airdroid-com-tmp` 与同步到国内 `bizdl-airdroid-com`。  
本仓库示例直接写入华南 `bizdl-airdroid-com`，与当前上传成功路径一致。

---

## 推荐阅读顺序

1. [01-概念与总流程](./01-概念与总流程.md)
2. [02-上传](./02-上传.md)
3. [03-下载与存在性检查](./03-下载与存在性检查.md)
4. [04-常见错误排查](./04-常见错误排查.md)
