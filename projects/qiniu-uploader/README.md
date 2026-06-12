# qiniu-uploader

这个目录是把仓库里原本分散在 `docs/qiniu/` 和 `file/` 的七牛相关内容，整理成一个单独的子项目。

目标不是重写所有脚本，而是把它们放进一个更容易理解和维护的结构里：

- `scripts/`
  已经跑通的普通上传/下载/检查/前端工具脚本
- `resumable/`
  断点续传版脚本、依赖和单独 SOP
- `docs/`
  重新整理过的主题文档
- `docs/manuals/`
  从旧 `file/doc` 迁过来的细手册
- `integrations/`
  手机端、daemon、上传模板兼容等接入说明

## 你应该怎么读

如果你是第一次进入这个目录，建议按这个顺序：

1. [docs/01-project-map.md](./docs/01-project-map.md)
2. [docs/02-qiniu-auth-models.md](./docs/02-qiniu-auth-models.md)
3. [docs/03-upload-session-and-app-contract.md](./docs/03-upload-session-and-app-contract.md)
4. [docs/04-form-upload-template-and-multi-provider.md](./docs/04-form-upload-template-and-multi-provider.md)
5. [docs/05-daemon-and-batch-upload-flow.md](./docs/05-daemon-and-batch-upload-flow.md)
6. [docs/06-resumable-and-production-mapping.md](./docs/06-resumable-and-production-mapping.md)
7. [docs/manuals/README.md](./docs/manuals/README.md)

## 目录说明

### `scripts/`

这里放的是普通版脚本，逻辑不做重写，只做迁移和归类。

- `qiniu_prefix_uptoken.py`
- `upload_local_directory_to_qiniu.py`
- `qiniu_check_object_exists.py`
- `qiniu_download_object.py`
- `test_qiniu_prefix_uptoken.py`
- `qiniuBatchUpload.ts`

### `resumable/`

这里放的是断点续传版。

- 上传鉴权
- 断点上传
- 断点下载
- 本地 `.venv` 安装 SOP
- 手机端断点续传接入说明

### `integrations/`

这里放的是更偏业务接入的材料：

- 手机端普通上传流程
- daemon 上传流程图
- 上传结果回传服务端
- 上传规则模板兼容性讨论
- 标准表单上传升级到断点续传时的改造说明

## 当前边界

这个子项目现在已经整理成“可单独理解”的结构，但有几个边界仍然保留：

- 普通上传脚本保持原样，不主动改逻辑
- 断点续传仍然单独维护，不强行和普通上传混成一套
- 多云兼容目前还是设计与协议讨论，不是完整实现

## 一句话理解

如果把这个目录压成一句话：

**它是一个围绕七牛上传授权、批量上传、断点续传和手机端接入整理出来的独立案例项目。**
