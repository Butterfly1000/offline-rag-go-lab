# 01 项目地图

这份文档回答一个问题：

**`qiniu-uploader` 现在到底包含哪些能力，先看哪里最不迷路？**

## 1. 这个子项目解决什么问题

它主要围绕 4 类问题：

1. 七牛上传授权怎么生成
2. 客户端/daemon 如何按服务端下发配置上传本地文件
3. 普通表单上传和断点续传怎么分别落地
4. 如果以后要兼容 S3 / OSS，上传协议能抽象到什么程度

## 2. 四大区域

### `docs/`

是你理解问题的入口。

### `scripts/`

是普通上传版已经跑通的脚本集合。

### `resumable/`

是断点续传版完整材料。

### `integrations/`

是和手机端、daemon、服务端回传相关的业务接入说明。

## 3. 推荐阅读路径

### 路线 A：先理解授权和上传协议

1. `02-qiniu-auth-models.md`
2. `03-upload-session-and-app-contract.md`
3. `04-form-upload-template-and-multi-provider.md`

### 路线 B：先理解手机端/daemon 方案

1. `03-upload-session-and-app-contract.md`
2. `05-daemon-and-batch-upload-flow.md`
3. `integrations/mobile_upload_integration.md`

### 路线 C：直接上手脚本

1. `scripts/qiniu_prefix_uptoken.py`
2. `scripts/upload_local_directory_to_qiniu.py`
3. `scripts/qiniu_check_object_exists.py`
4. `scripts/qiniu_download_object.py`

### 路线 D：只看断点续传

1. `06-resumable-and-production-mapping.md`
2. `resumable/SOP.md`
3. `resumable/qiniu_resumable_auth.py`
4. `resumable/qiniu_resumable_upload.py`

## 4. 现在最重要的理解

先不要把所有问题混成一团。

最稳的拆法是：

- 授权怎么生成
- 客户端拿什么上传
- 多文件是否共用 token
- 什么时候该用表单上传
- 什么时候该切到断点续传

## 5. 当前项目状态

已经明确的部分：

- 七牛 AK/SK、UploadToken、私有下载 URL 的角色边界
- 普通表单上传脚本链路
- 手机端/daemon 批量上传接入方式
- 断点续传版单独实现

仍然属于讨论/待扩展的部分：

- 多云统一上传协议
- 更完整的服务端授权 API 设计
- 统一普通上传与断点续传的客户端抽象
