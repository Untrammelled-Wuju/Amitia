package com.amitia.core.network.client

sealed class AmitiaApiException(
    message: String,
    cause: Throwable? = null
) : Exception(message, cause) {

    data object NetworkUnavailable : AmitiaApiException("网络不可用")

    data class RemoteUnreachable(val url: String) :
        AmitiaApiException("无法连接到远程服务: $url")

    data object TokenExpired : AmitiaApiException("认证令牌已过期")

    data class ServerError(val status: Int, val body: String? = null) :
        AmitiaApiException("服务器错误: HTTP $status body=${body?.take(200)}")

    data class NotFound(val path: String) :
        AmitiaApiException("资源不存在: $path")

    data object Timeout : AmitiaApiException("请求超时")

    data class SseDisconnected(val reason: String?) :
        AmitiaApiException("SSE 连接断开: ${reason ?: "unknown"}")

    data class UploadFailed(val reason: String?) :
        AmitiaApiException("文件上传失败: ${reason ?: "unknown"}")

    data class AudioFailed(val reason: String?) :
        AmitiaApiException("音频处理失败: ${reason ?: "unknown"}")

    data class MigrationFailed(val reason: String?) :
        AmitiaApiException("数据迁移失败: ${reason ?: "unknown"}")

    data class Unknown(val raw: Throwable) :
        AmitiaApiException("未知错误: ${raw.message}", raw)
}
