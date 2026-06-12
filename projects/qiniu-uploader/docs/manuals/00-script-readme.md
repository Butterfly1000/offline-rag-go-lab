# 七牛前缀上传本地示例

分主题文档（概念、上传、下载、排错、与 uc 对照）见 **[manuals/README.md](./README.md)**。

这个目录现在有 7 个文件，作用分别是：

1. `qiniu_prefix_uptoken.py`
   这是后端/本地脚本，用来生成七牛上传 token 和上传会话 JSON。
2. `qiniuBatchUpload.ts`
   这是前端工具代码，用来拿着后端给的 token 批量上传文件。
3. `test_qiniu_prefix_uptoken.py`
   这是 Python 测试文件，用来确认 token 生成逻辑没写错。
4. `README.md`
   这是当前说明文档，你可以直接照着执行。
5. `upload_local_directory_to_qiniu.py`
   这是客户端/桌面 app 场景下最实用的脚本：服务端先下发 upload session，客户端再传一个本地目录路径，自动把里面的文件上传到七牛。
6. `qiniu_check_object_exists.py`
   这是检查七牛文件/地址是否存在的脚本。
7. `qiniu_download_object.py`
   这是从七牛下载文件到本地的脚本。

如果你是第一次接触前后端，可以把这个项目理解成两段：

- Python 文件负责“发通行证”
- TypeScript 文件负责“拿着通行证去上传文件”
- 新增的批量上传/检查/下载脚本负责“让客户端或本地程序直接操作文件”

---

## 1. 这个示例能做什么

它演示的是“前缀上传”：

- 你不想让前端随便上传到整个 bucket
- 你只想允许它上传到某一个目录下
- 比如只能传到 `videos/task123/device456/` 这个前缀

这样更安全，也更容易按任务、设备、用户去管理文件。

---

## 2. 本地运行环境

先确认你本地至少有这些：

### Python 环境

建议版本：

- Python 3.10 或更高

检查命令：

```bash
python3 --version
```

### Node / 前端环境

建议版本：

- Node.js 18 或更高

检查命令：

```bash
node -v
```

说明：

- 当前目录里的 TypeScript 文件只是“给前端项目引用的工具文件”
- 它不是一个单独执行的脚本
- 所以本地最先验证的是 Python 这部分

---

## 3. 先跑测试，确认环境正常

在当前目录执行：

```bash
python3 -m unittest test_qiniu_prefix_uptoken.py
```

如果成功，你会看到类似：

```text
....
----------------------------------------------------------------------
Ran 4 tests in 0.00Xs

OK
```

这说明：

- Python 环境没问题
- 上传策略生成逻辑没问题
- upload session 输出结构也没问题

---

## 4. 直接生成一个本地 upload session

下面这条命令是你最应该先跑通的：

```bash
python3 qiniu_prefix_uptoken.py \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "videos/task123/device456/" \
  --upload-url "https://upload.qiniup.com" \
  --expires-in 1800 \
  --mime-limit "video/*" \
  --fsize-limit 2147483648
```

执行成功后，会输出一段 JSON，类似：

```json
{
  "provider": "qiniu",
  "mode": "form_post",
  "uploadUrl": "https://upload.qiniup.com",
  "uploadToken": "这里会是一长串token",
  "keyPrefix": "videos/task123/device456/",
  "expiresAt": 1760000000,
  "expiresInSec": 1800,
  "constraints": {
    "mimeLimit": "video/*",
    "maxFileSize": 2147483648
  }
}
```

这段 JSON 就是前端上传时要使用的“会话信息”。

---

## 5. 每个参数是什么意思

下面这一节是最重要的，小白可以直接照着理解。

### 必填参数

#### `--access-key`

意思：

- 七牛账号的 AccessKey
- 可以理解为“公开的账号标识”

用途：

- 用来告诉七牛“是谁在签名”

#### `--secret-key`

意思：

- 七牛账号的 SecretKey
- 可以理解为“签名密码”

用途：

- 用来给上传策略做签名

注意：

- 这个值绝对不能放到前端
- 只能放在后端或你本地安全环境里

#### `--bucket`

意思：

- 七牛云的存储空间名称

举例：

- `media-prod`
- `my-video-bucket`

用途：

- 告诉七牛文件要进哪个存储空间

#### `--key-prefix`

意思：

- 允许上传的对象前缀
- 可以理解为“七牛里的目录前缀”

举例：

- `videos/task123/device456/`

用途：

- 限定本次 token 只能上传到这个目录下面

建议：

- 最后带上 `/`

#### `--upload-url`

意思：

- 七牛接收上传请求的地址

常见值：

- `https://upload.qiniup.com`

用途：

- 前端最终会把文件 POST 到这个地址

注意：

- 不同机房、加速方案、私有部署场景，上传域名可能不同
- 你最好以七牛控制台或官方文档里的上传域名为准

### 可选参数

#### `--expires-in`

意思：

- token 有效期，单位是秒

举例：

- `1800` = 30 分钟
- `600` = 10 分钟

用途：

- 超过这个时间，前端再拿这个 token 上传就会失败

#### `--mime-limit`

意思：

- 限制允许上传的文件类型

举例：

- `video/*`
- `image/*`
- `application/pdf`

用途：

- 防止前端传错类型

#### `--fsize-limit`

意思：

- 限制单个文件最大大小
- 单位是字节

举例：

- `1048576` = 1MB
- `2147483648` = 2GB

用途：

- 防止上传超大文件

#### `--callback-url`

意思：

- 文件上传成功后，七牛服务端会回调你的业务接口

用途：

- 常用于上传完成后通知你自己的系统入库

#### `--callback-body`

意思：

- 七牛回调时携带的请求体模板

举例：

```text
bucket=$(bucket)&etag=$(etag)&key=$(key)&traceid=task123
```

用途：

- 让你的业务接口收到 bucket、key、etag 等信息

#### `--callback-body-type`

意思：

- 七牛回调 body 的内容类型

常见值：

- `application/x-www-form-urlencoded`
- `application/json`

#### `--insert-only`

意思：

- 只允许新增，不允许覆盖已存在的同名文件

用途：

- 避免误覆盖老文件

---

## 6. 你需要去七牛那边拿到什么

如果你要把这个示例真正跑起来，至少需要从七牛获取下面这些信息。

### 一定要有的

1. `AccessKey`
2. `SecretKey`
3. `Bucket` 名称
4. 上传域名 `uploadUrl`

### 建议同时确认的

1. 你准备上传到哪个目录前缀
2. 是否限制文件类型
3. 是否限制文件大小
4. 是否需要上传成功后的业务回调地址
5. 是否允许覆盖同名文件

你可以把它理解成一张准备清单：

```text
七牛后台需要确认：
AK = ?
SK = ?
Bucket = ?
上传域名 = ?
上传目录前缀 = ?
允许文件类型 = ?
单文件大小上限 = ?
是否需要回调 = ?
回调地址 = ?
```

---

## 7. 前端怎么接这个 TypeScript 文件

`qiniuBatchUpload.ts` 不是单独运行的，它通常放在你的前端项目里被引用。

最小接入方式：

```ts
import { uploadDirectoryFiles } from "./qiniuBatchUpload";
```

你的前端需要先从后端拿到 upload session：

```ts
const session = await fetch("/api/qiniu/upload-session").then((r) => r.json());
```

然后拿文件列表上传：

```ts
const input = document.querySelector<HTMLInputElement>("#videos");
const files = Array.from(input?.files ?? []).map((file) => ({ file }));
const results = await uploadDirectoryFiles(session, files, "prefix_plus_filename");
console.log(results);
```

这里做了两件事：

1. 用后端给的 `keyPrefix` 和文件名，拼出最终七牛 key
2. 把 `token + key + file` 一起提交到七牛

---

## 8. 当前目录如何理解“谁负责什么”

### Python 负责的事

- 生成七牛上传策略
- 使用 AK/SK 签名
- 输出前端需要的上传凭证 JSON

### TypeScript 负责的事

- 接收 Python/后端返回的 JSON
- 拼接上传 key
- 调七牛上传接口

### 客户端 / app 负责的事

- 接收服务端下发的本地目录路径
- 读取该目录里的文件
- 决定调用 Python 脚本上传、检查或下载

---

## 9. 如果不是浏览器，而是桌面 app，该怎么上传

这一段非常重要。

如果你们是普通网页前端：

- 网页不能只靠一个本地路径字符串就去读用户电脑里的文件

但如果你们是桌面 app、客户端、Electron、Tauri、Python 程序：

- 程序本身有本地文件系统权限
- 就可以拿到一个本地路径
- 读取目录里的文件
- 再调用上传逻辑

也就是说，你现在这个场景更准确地说不是“浏览器前端上传”，而是：

- 服务端给 app 一个本地目录路径
- app 扫描这个目录
- app 把目录中的文件逐个上传到七牛

---

## 10. 目录批量上传脚本怎么用

这里要特别区分清楚：

- 服务端负责生成 upload session
- 客户端 / app 不保存 AK / SK
- 客户端只拿到服务端下发的 JSON，再加一个 `local-dir` 去上传

作用：

- 读取 `--local-dir` 指向的目录
- 找到里面的文件
- 每个文件都上传到服务端 session 里的 `keyPrefix` 指定前缀下

### 方式 1：把服务端返回的 JSON 存成文件再上传

比如服务端先返回：

```json
{
  "provider": "qiniu",
  "mode": "form_post",
  "uploadUrl": "https://air-test-qn.airdroid.com",
  "uploadToken": "这里是一长串token",
  "keyPrefix": "videos/task123/device456/",
  "expiresAt": 1780380873,
  "expiresInSec": 1800,
  "constraints": {
    "maxFileSize": 5368709120
  }
}
```

客户端把这段内容保存成 `session.json` 后执行：

```bash
python3 upload_local_directory_to_qiniu.py \
  --local-dir "/你的本地目录路径" \
  --session-file "./session.json"
```

### 方式 2：直接把服务端返回的 JSON 字符串传进去

```bash
python3 upload_local_directory_to_qiniu.py \
  --local-dir "/你的本地目录路径" \
  --session-json '{"provider":"qiniu","mode":"form_post","uploadUrl":"https://air-test-qn.airdroid.com","uploadToken":"这里是一长串token","keyPrefix":"videos/task123/device456/","expiresAt":1780380873,"expiresInSec":1800,"constraints":{"maxFileSize":5368709120}}'
```

比如本地目录里有：

```text
/Users/you/demo-files/
  a.mp4
  b.jpg
  c.txt
```

如果服务端下发的 `keyPrefix` 是：

```text
videos/task123/device456/
```

那么上传后对应的七牛 key 大概会是：

```text
videos/task123/device456/a.mp4
videos/task123/device456/b.jpg
videos/task123/device456/c.txt
```

说明：

- 这个脚本不管是不是视频
- 只要是文件，就会上传
- 如果服务端 session 里有限制大小，脚本会先做一次本地大小校验

### 递归上传子目录

如果你希望连子目录里的文件也一起上传：

```bash
python3 upload_local_directory_to_qiniu.py \
  --local-dir "/你的本地目录路径" \
  --session-file "./session.json" \
  --recursive
```

注意：

- 当前脚本递归时会扫描子目录
- 但上传到七牛时默认还是只保留文件名，不保留原子目录层级
- 如果你后面希望“把本地相对路径也保留下来”，我也可以继续帮你加

---

## 11. app 里应该怎么接这个上传逻辑

你可以这样理解 app 侧流程：

1. 服务端返回一个本地目录路径
2. app 拿到这个路径
3. app 把服务端返回的 upload session 传给 `upload_local_directory_to_qiniu.py`
4. 脚本扫描目录并上传
5. 脚本返回 JSON 结果
6. app 再把结果展示给用户

如果你们 app 能直接运行 Python 命令，那么最简单就是调用：

```bash
python3 upload_local_directory_to_qiniu.py --local-dir "..." --session-file "./session.json"
```

如果你们 app 不直接跑 Python，也可以把脚本里的逻辑改写进你们 app 本身。

---

## 12. 检查某个文件是否存在

### 方式 1：直接检查完整 URL

```bash
python3 qiniu_check_object_exists.py \
  --url "https://cdn.example.com/videos/task123/device456/a.mp4"
```

### 方式 2：传域名和 key

```bash
python3 qiniu_check_object_exists.py \
  --base-url "https://cdn.example.com" \
  --key "videos/task123/device456/a.mp4"
```

### 如果是私有空间

```bash
python3 qiniu_check_object_exists.py \
  --base-url "https://cdn.example.com" \
  --key "videos/task123/device456/a.mp4" \
  --private \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK"
```

输出会类似：

```json
{
  "exists": true,
  "status": 200,
  "url": "实际访问地址"
}
```

---

## 13. 下载七牛文件到本地

### 方式 1：直接传完整 URL

```bash
python3 qiniu_download_object.py \
  --url "https://cdn.example.com/videos/task123/device456/a.mp4" \
  --output "/你的本地保存路径/a.mp4"
```

### 方式 2：传域名和 key

```bash
python3 qiniu_download_object.py \
  --base-url "https://cdn.example.com" \
  --key "videos/task123/device456/a.mp4" \
  --output "/你的本地保存路径/a.mp4"
```

### 私有空间下载

```bash
python3 qiniu_download_object.py \
  --base-url "https://cdn.example.com" \
  --key "videos/task123/device456/a.mp4" \
  --private \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --output "/你的本地保存路径/a.mp4"
```

---

## 14. 最常见报错排查

### 报错：`401 unauthorized`

通常说明：

- AK/SK 写错了
- token 已过期
- bucket 不匹配

### 报错：`invalid token`

通常说明：

- token 签名不对
- policy 字段不符合七牛要求
- 前端传上去的 token 被改动过

### 报错：上传域名不通

通常说明：

- `uploadUrl` 写错了
- 用了不对应机房的上传地址
- 本地网络有拦截

### 报错：文件类型或大小不允许

通常说明：

- 命中了 `mimeLimit`
- 命中了 `fsizeLimit`

---

## 15. 本地最小工作流

如果你只是想先跑通，按这个顺序做最稳：

1. 先执行测试

```bash
python3 -m unittest test_qiniu_prefix_uptoken.py
```

2. 再生成 upload session

```bash
python3 qiniu_prefix_uptoken.py \
  --access-key "你的七牛AK" \
  --secret-key "你的七牛SK" \
  --bucket "你的bucket名字" \
  --key-prefix "videos/task123/device456/"
```

3. 把这段 JSON 返回给 app，或者保存成 `session.json`

4. app 用 `local-dir + session.json` 调用上传脚本

5. 如果你们是桌面 app，本地目录上传优先使用：

```bash
python3 upload_local_directory_to_qiniu.py \
  --local-dir "/你的本地目录路径" \
  --session-file "./session.json"
```

---

## 16. 一句话总结

你现在只需要先记住：

- `qiniu_prefix_uptoken.py` 用来生成上传凭证
- `qiniuBatchUpload.ts` 用来真正上传文件
- `upload_local_directory_to_qiniu.py` 用来拿“服务端下发的 session + 本地目录”批量上传
- `qiniu_check_object_exists.py` 用来检查文件/地址是否存在
- `qiniu_download_object.py` 用来下载文件
- 你必须从七牛拿到 `AK / SK / Bucket / 上传域名`
- `key-prefix` 决定前端这次“只允许上传到哪里”
