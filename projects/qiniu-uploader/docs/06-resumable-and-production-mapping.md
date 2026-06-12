# 06 断点续传与生产映射

这份文档把两个主题放在一起：

1. 什么时候要从普通上传切到断点续传
2. 这个案例和生产项目 `uc.airdroid.com` 的对应关系

## 1. 什么时候必须考虑断点续传

普通表单上传适合：

- 小文件
- 简单接入
- 快速验证

断点续传更适合：

- 大文件
- 断网恢复
- 移动端弱网
- 后台长时间执行

## 2. 当前断点续传区域在哪

看这里：

- [resumable/README.md](../resumable/README.md)
- [resumable/SOP.md](../resumable/SOP.md)
- [resumable/qiniu_resumable_auth.py](../resumable/qiniu_resumable_auth.py)
- [resumable/qiniu_resumable_upload.py](../resumable/qiniu_resumable_upload.py)
- [resumable/qiniu_resumable_download.py](../resumable/qiniu_resumable_download.py)

## 3. 断点续传版和普通版最大的区别

普通版：

- `mode = form_post`
- 一次请求传完整文件

断点续传版：

- `mode = resumable_v2`
- 客户端需要七牛分片上传能力
- 需要维护本地断点记录

## 4. 当前生产映射要点

原有材料已经说明了几个关键对应关系：

- 上传 token 和下载签名是两套东西
- bucket、上传域名、访问域名必须对齐
- 私有下载 URL 不是 uploadToken

如果你要对照 `uc.airdroid.com`，最值得优先确认的是：

1. bucket 名
2. 上传机房域名
3. 下载/访问域名
4. token 的实际用途

## 5. 这部分最适合怎么学

推荐顺序：

1. 先学普通上传版
2. 再看 `integrations/qiniu_resumable_upload_adjustments.md`
3. 再看 `resumable/SOP.md`
4. 最后再对照生产映射材料

这样不容易一开始就把“普通上传”和“断点续传”混在一起。
