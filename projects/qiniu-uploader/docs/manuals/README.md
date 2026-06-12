# 七牛前缀上传与下载 — 文档索引

本目录是对 `scripts/` 下脚本与实战踩坑的整理，按主题拆成多篇，便于按需阅读。

## 建议阅读顺序

| 顺序 | 文档 | 内容 |
|------|------|------|
| 1 | [01-概念与总流程](./01-概念与总流程.md) | 上传 token、下载 token、三类域名 |
| 2 | [02-上传](./02-上传.md) | 生成 session、curl 上传、机房域名 |
| 3 | [03-下载与存在性检查](./03-下载与存在性检查.md) | 私有下载 URL、HEAD/GET、浏览器下载 |
| 4 | [04-常见错误排查](./04-常见错误排查.md) | BadToken、incorrect region、404/405 等 |
| 5 | [05-脚本速查](./05-脚本速查.md) | 各脚本命令一行版 |
| 6 | [06-与-uc-airdroid-对照](./06-与-uc-airdroid-对照.md) | 生产项目配置与域名对应关系 |

## 相关代码（上级目录）

| 文件 | 作用 |
|------|------|
| `qiniu_prefix_uptoken.py` | 服务端生成 upload session / uploadToken |
| `upload_local_directory_to_qiniu.py` | 客户端按 session 批量上传本地目录 |
| `qiniu_check_object_exists.py` | HEAD 检查对象是否可访问 |
| `qiniu_download_object.py` | GET 下载到本地 |
| `qiniuBatchUpload.ts` | 前端表单上传工具 |
| `test_qiniu_prefix_uptoken.py` | 单元测试 |

更完整的项目入口见上级目录 [README.md](../../README.md)。

## 已跑通的最小闭环（bizdl 示例）

1. 用 `qiniu_prefix_uptoken.py` 生成 session（`--upload-url https://up-z2.qiniup.com`，bucket `bizdl-airdroid-com`）。
2. curl 或 `upload_local_directory_to_qiniu.py` 上传，得到 `key` 与 `hash`。
3. 用 `bizdl.airdroid.com` 作为 `--base-url`，加 `--private` 做存在性检查与下载。

细节见各分册文档。
