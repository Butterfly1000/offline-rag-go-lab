# 统一上传规则模板兼容方案讨论

这份文档是独立讨论稿。

目的：

- 单独模拟“服务端下发上传规则，客户端只负责执行”是否可行
- 看看七牛、OSS、S3 兼容场景下怎么统一

核心思路：

- 服务端不下发某家云的 SDK 逻辑
- 服务端下发一份“HTTP 上传规则模板”
- 手机端 / daemon 只负责：
  - 扫描文件
  - 生成文件 key
  - 注入文件内容
  - 按规则发请求

也就是说，客户端不关心“这是不是七牛/OSS/S3”，只关心：

- 请求地址是什么
- 请求方法是什么
- 固定字段有哪些
- 动态字段怎么拼
- 文件字段名是什么

---

## 1. 这个方向的局限和难点

先说结论：

- 这个方向可以讨论
- 但它不是“天然无缝”
- 也不是“下发一条 curl 就能全兼容”

它的局限主要在下面几类地方。

### 1.1 不同云厂商虽然都叫对象存储，但上传协议并不完全一致

表面上看，七牛、OSS、S3 都能“上传文件”。

但实际差异很大：

- 有的是 `multipart/form-data`
- 有的是预签名 `PUT`
- 有的鉴权放表单字段
- 有的鉴权放 query 参数
- 有的成功返回 `200`
- 有的可能返回 `204`
- 有的返回 JSON
- 有的返回空 body

所以不能简单理解成：

- “反正最后都是 HTTP 上传，所以完全一样”

更准确的说法应该是：

- “有机会抽象成少数几类上传模式，但不是完全统一”

### 1.2 文件上传只是第一层，复杂场景会更难抽象

简单单文件直传比较容易统一。

但一旦进入这些场景，难度会明显上升：

- 分片上传
- 断点续传
- 秒传
- 上传前预检查
- 多阶段签名
- 上传后回调校验
- 自定义 header 校验

也就是说：

- 这个思路更适合“简单直传”
- 不适合直接假设“未来所有上传能力都能靠一份模板覆盖”

### 1.3 客户端仍然需要实现一层协议执行器

即使服务端下发规则，客户端也不是完全不用开发。

客户端至少仍然要处理：

- 文件扫描
- 正则过滤
- key 生成
- multipart 表单组装
- raw PUT 请求发送
- 成功失败判断
- 错误重试
- 上传结果回传

所以它减少的是“云厂商差异耦合”，不是“客户端零实现成本”。

### 1.4 真正难的地方不是“发请求”，而是“怎么定义稳定协议”

如果协议定义得太简单：

- 未来兼容不了更多场景

如果协议定义得太复杂：

- 手机端 / daemon 实现成本会上升
- 服务端也更难维护

所以最大的难点不在 HTTP 本身，而在于：

- 怎样抽象到“足够通用”
- 又不把客户端协议做成一个过度设计的大系统

---

## 2. 如果真的要做，只能走相对简单的实现

如果目标是：

- 尽量不发版
- 服务端主导云厂商切换
- 客户端只做执行

那比较现实的做法，不是“万能兼容”，而是：

- 先只覆盖最常见、最简单的上传模式

也就是：

1. `multipart_form`
2. `raw_put`

换句话说：

- 不去追求“一套规则覆盖所有高级上传能力”
- 只先覆盖“简单单文件上传”这类高频场景

这是相对简单、也相对可落地的做法。

---

## 3. 统一抽象长什么样

建议服务端下发的不是一条 `curl` 字符串，而是一份“等价于 curl 的结构化配置”。

示例：

```json
{
  "uploadMode": "multipart_form",
  "method": "POST",
  "url": "https://upload.example.com",
  "headers": {},
  "fileFieldName": "file",
  "fixedFields": {
    "token": "xxxxx"
  },
  "dynamicFields": {
    "key": "{keyPrefix}{fileName}"
  },
  "context": {
    "keyPrefix": "DeviceName/Date/"
  },
  "successRule": {
    "statusCode": [200]
  }
}
```

客户端只做两件事：

1. 按规则替换动态字段
2. 把本地文件内容放进 `fileFieldName`

等价思路就是：

```bash
curl -X POST "$url" \
  -F "token=xxxxx" \
  -F "key=DeviceName/Date/demo.mp4" \
  -F "file=@/local/path/demo.mp4"
```

但协议层不直接下发 curl 文本，而是下发结构化 JSON。

---

## 4. 为什么不要直接下发 curl 字符串

理论上可以下发 curl 字符串，但不建议这样做。

原因：

1. Android / iOS / daemon 不会真的去执行 shell curl
2. 引号、空格、中文文件名、转义很容易出错
3. 文件内容没法直接靠模板表达，最终还是客户端自己读文件再塞进去
4. 安全性和可维护性都差

所以更合理的方式是：

- 下发“curl 等价规则”
- 客户端自己组装 HTTP 请求

---

## 5. 三种兼容场景

下面模拟 3 种情况，看客户端能否用同一套思路兼容。

---

## 6. 场景一：七牛当前方案

这是你们当前目录最接近的情况。

特点：

- `POST`
- `multipart/form-data`
- 固定字段：`token`
- 动态字段：`key`
- 文件字段：`file`

### 服务端下发规则示例

```json
{
  "provider": "qiniu",
  "uploadMode": "multipart_form",
  "method": "POST",
  "url": "https://air-test-qn.airdroid.com",
  "fileFieldName": "file",
  "fixedFields": {
    "token": "xxxxx:xxxxx:xxxxx"
  },
  "dynamicFields": {
    "key": "{keyPrefix}{fileName}"
  },
  "context": {
    "keyPrefix": "DeviceName/Date/"
  },
  "successRule": {
    "statusCode": [200]
  }
}
```

### 客户端执行方式

1. 读取本地文件 `demo.mp4`
2. 生成 `key = DeviceName/Date/demo.mp4`
3. 按 `multipart/form-data` 发请求
4. 注入：
   - `token`
   - `key`
   - `file`

### 是否容易兼容

非常容易。  
因为你们现有实现本质就是这个模型。

---

## 7. 场景二：OSS / S3 兼容的表单直传

这一类通常也是：

- `POST`
- `multipart/form-data`
- 固定字段比七牛更多
- 文件字段通常仍然是 `file`

常见固定字段可能包括：

- `key`
- `policy`
- `OSSAccessKeyId` 或其他鉴权字段
- `Signature` 或签名字段

### 服务端下发规则示例

```json
{
  "provider": "oss",
  "uploadMode": "multipart_form",
  "method": "POST",
  "url": "https://bucket-example.oss-cn-hangzhou.aliyuncs.com",
  "fileFieldName": "file",
  "fixedFields": {
    "policy": "xxxxx",
    "OSSAccessKeyId": "test-access-id",
    "Signature": "test-signature",
    "success_action_status": "200"
  },
  "dynamicFields": {
    "key": "{keyPrefix}{fileName}"
  },
  "context": {
    "keyPrefix": "DeviceName/Date/"
  },
  "successRule": {
    "statusCode": [200, 204]
  }
}
```

### 客户端执行方式

1. 读取本地文件 `demo.mp4`
2. 生成 `key = DeviceName/Date/demo.mp4`
3. 按 `multipart/form-data` 发请求
4. 注入：
   - `policy`
   - `OSSAccessKeyId`
   - `Signature`
   - `success_action_status`
   - `key`
   - `file`

### 是否容易兼容

也比较容易。  
因为它和七牛一样，本质还是“表单上传 + 固定字段 + 动态 key + 文件内容”。

也就是说：

- 客户端不需要理解 OSS 细节
- 只要按服务端给的字段原样提交

---

## 8. 场景三：S3 预签名 PUT 上传

这一类和前两种不太一样。

特点：

- 不是 `multipart/form-data`
- 通常是 `PUT`
- 请求体直接就是文件二进制
- 鉴权信息可能在 URL 上，也可能在 header 上

### 服务端下发规则示例

```json
{
  "provider": "s3",
  "uploadMode": "raw_put",
  "method": "PUT",
  "url": "https://bucket-example.s3.amazonaws.com/DeviceName/Date/demo.mp4?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=xxxxx",
  "headers": {
    "Content-Type": "video/mp4"
  },
  "fileFieldName": null,
  "fixedFields": {},
  "dynamicFields": {},
  "context": {},
  "successRule": {
    "statusCode": [200]
  }
}
```

### 客户端执行方式

1. 读取本地文件 `demo.mp4`
2. 不再组 `multipart/form-data`
3. 直接发 `PUT`
4. 请求体就是文件二进制
5. 带上服务端要求的 header

### 是否容易兼容

可以兼容，但比前两种多一个分支。

原因：

- 前两种是“表单上传”
- 这一种是“原始二进制上传”

所以客户端协议层至少要支持两种上传模式：

1. `multipart_form`
2. `raw_put`

只要协议层支持这两个模式，客户端还是不用理解某个云厂商本身。

---

## 9. 三种场景怎么统一

如果要统一，不要统一成“一个固定 curl”。

而是统一成“一个上传规则协议”，协议里至少有这些字段：

```json
{
  "provider": "qiniu | oss | s3",
  "uploadMode": "multipart_form | raw_put",
  "method": "POST | PUT",
  "url": "上传地址",
  "headers": {},
  "fileFieldName": "file 或 null",
  "fixedFields": {},
  "dynamicFields": {},
  "context": {},
  "successRule": {
    "statusCode": [200]
  }
}
```

客户端统一逻辑：

1. 读取文件
2. 根据 `uploadMode` 选择请求拼装方式
3. 根据 `dynamicFields` 生成 `key` 等动态值
4. 注入 `fixedFields`
5. 发请求
6. 根据 `successRule` 判断是否成功

---

## 10. 这种实现本身的不合理和问题

即使按上面的方式实现，它本身也有明显问题，不适合被描述成“完美方案”。

### 10.1 协议会逐渐膨胀

一开始可能只要几个字段：

- `url`
- `method`
- `headers`
- `fixedFields`
- `dynamicFields`

但随着兼容需求增加，协议很容易膨胀出更多字段：

- 成功判定规则
- 错误映射规则
- 重试规则
- 分片规则
- 回调规则

最后可能会把“上传客户端”做成一个很重的解释器。

### 10.2 调试难度会升高

以前如果是七牛专用逻辑，排查时大家都知道：

- 就是七牛 token、key、file 这条链路

如果变成规则驱动：

- 同一个客户端行为要先看服务端下发了什么规则
- 再看客户端如何解释规则
- 再看目标云厂商是否接受

定位问题时会多一层“协议解释”的复杂度。

### 10.3 客户端表面通用，实际仍然要维护多种分支

即使不暴露“七牛分支”“OSS 分支”“S3 分支”这些名字，
客户端内部通常还是要处理：

- `multipart_form`
- `raw_put`
- 不同返回格式
- 不同状态码

所以它不是“彻底统一”，而是“把厂商分支收敛成协议分支”。

### 10.4 可能会削弱类型约束和明确性

如果现在是七牛专用协议，字段都很明确：

- `uploadToken`
- `keyPrefix`
- `uploadUrl`

如果以后抽象成统一模板，字段会更泛化。

好处是更灵活，坏处是：

- 业务语义没那么直观
- 新人更难一眼看懂
- 接口文档和调试信息会更抽象

### 10.5 不一定真的比“服务端多实现几种存储适配”更省

这个方案的初衷是减少手机端发版。

但如果未来切换云厂商本来就不频繁，或者上传模式长期固定，
那未必值得为了“可能的未来兼容”去提前引入一层较复杂的通用协议。

也就是说：

- 它更像一种架构投资
- 是否值得，要看未来切换厂商的频率和实际收益

---

## 11. 兼容性结论

### 七牛

完全适合放进这套抽象里。

### OSS 表单上传

大概率也适合放进这套抽象里。

### S3 预签名 PUT

也可以兼容，但客户端要多支持一种 `raw_put` 模式。

---

## 12. 最终判断

如果目标是：

- 后续尽量不因为换对象存储而发版
- 服务端负责云厂商差异
- 客户端只负责执行上传规则

那么这条路是可行的。

但前提是：

1. 不要下发 curl 文本
2. 要下发结构化上传规则
3. 客户端至少支持两种模式：
   - `multipart_form`
   - `raw_put`

---

## 13. 一句话总结

“统一的不是某家云厂商，而是上传请求模板；只要服务端下发的是结构化 HTTP 规则，客户端就可以在七牛、OSS 表单直传、S3 预签名 PUT 之间做兼容，而不需要频繁发版。”
