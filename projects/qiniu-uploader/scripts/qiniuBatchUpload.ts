/**
 * 这个文件是“前端上传工具”。
 *
 * 它不负责生成 token，而是负责：
 * 1. 接收后端返回的 upload session
 * 2. 为每个文件拼出最终 key（七牛里的文件路径）
 * 3. 把 token + key + file 通过表单 POST 到七牛
 */

export type QiniuBatchUploadSession = {
    // 固定写 qiniu，方便前端统一识别当前使用的是哪家云存储。
    provider: "qiniu";

    // 当前示例使用的是浏览器 multipart/form-data 直传模式。
    mode: "form_post";

    // 七牛上传地址，例如 https://upload.qiniup.com
    uploadUrl: string;

    // 后端生成的上传凭证，前端必须原样带上。
    uploadToken: string;

    // 约定本次上传允许写入的目录前缀，例如 videos/task123/device456/
    keyPrefix: string;

    // token 绝对过期时间（Unix 时间戳，秒）
    expiresAt: number;

    // token 从生成时开始还能用多少秒
    expiresInSec: number;

    // 可选的前端展示约束，主要用于提示用户，不是浏览器自动强校验。
    constraints?: {
        mimeLimit?: string | null;
        maxFileSize?: number | null;
    };
};

export type LocalUploadFile = {
    // 浏览器里的 File 对象，通常来自 <input type="file"> 或拖拽上传。
    file: File;

    // 如果你是按目录上传，这里可以额外保存相对路径；当前示例没有直接用到。
    relativePath?: string;
};

export type UploadResult = {
    // 原始本地文件名，方便页面展示结果。
    fileName: string;

    // 上传到七牛后对应的对象 key。
    key: string;

    // HTTP 请求是否成功（2xx 为 true）。
    ok: boolean;

    // HTTP 状态码，例如 200、401、614。
    status: number;

    // 七牛返回的原始文本，出错排查时非常有用。
    responseText: string;
};

// 两种命名策略：
// 1. prefix_plus_filename: 目录前缀 + 原文件名
// 2. prefix_plus_index: 目录前缀 + 序号
export type NamingStrategy = "prefix_plus_filename" | "prefix_plus_index";

function normalizePrefix(prefix: string): string {
    // 七牛 key 本质是字符串路径；这里统一保证前缀以 / 结尾，避免拼接出错。
    return prefix.endsWith("/") ? prefix : `${prefix}/`;
}

function safeFileName(fileName: string): string {
    // 把不太安全或不稳定的字符替换成下划线，减少对象 key 异常情况。
    return fileName.replace(/[^\w.\-()]/g, "_");
}

export function buildObjectKey(
    session: QiniuBatchUploadSession,
    file: File,
    index: number,
    strategy: NamingStrategy = "prefix_plus_filename",
): string {
    const prefix = normalizePrefix(session.keyPrefix);

    // 如果用序号命名，就保留扩展名，避免上传后文件类型不好识别。
    if (strategy === "prefix_plus_index") {
        const ext = file.name.includes(".") ? file.name.slice(file.name.lastIndexOf(".")) : "";
        return `${prefix}${index}${ext}`;
    }

    // 默认用“前缀 + 安全文件名”作为七牛对象 key。
    return `${prefix}${safeFileName(file.name)}`;
}

export async function uploadSingleFile(
    session: QiniuBatchUploadSession,
    file: File,
    key: string,
): Promise<UploadResult> {
    // 七牛表单上传接口要求至少带上 token、key、file 这 3 个字段。
    const form = new FormData();
    form.set("token", session.uploadToken);
    form.set("key", key);
    form.set("file", file, file.name);

    const response = await fetch(session.uploadUrl, {
        method: "POST",
        body: form,
    });

    const responseText = await response.text();
    return {
        fileName: file.name,
        key,
        ok: response.ok,
        status: response.status,
        responseText,
    };
}

export async function uploadDirectoryFiles(
    session: QiniuBatchUploadSession,
    files: LocalUploadFile[],
    strategy: NamingStrategy = "prefix_plus_filename",
): Promise<UploadResult[]> {
    const results: UploadResult[] = [];

    // 这里串行上传最直观，也更容易排查问题。
    // 如果以后追求速度，可以改成并发上传，但要额外处理失败重试和限流。
    for (let i = 0; i < files.length; i += 1) {
        const current = files[i];
        const key = buildObjectKey(session, current.file, i + 1, strategy);
        results.push(await uploadSingleFile(session, current.file, key));
    }

    return results;
}

/*
浏览器端最小示例：

const session: QiniuBatchUploadSession = await fetch("/api/qiniu/upload-session").then((r) => r.json());
const input = document.querySelector<HTMLInputElement>("#videos");
const files = Array.from(input?.files ?? []).map((file) => ({ file }));
const results = await uploadDirectoryFiles(session, files, "prefix_plus_filename");
console.log(results);
*/
