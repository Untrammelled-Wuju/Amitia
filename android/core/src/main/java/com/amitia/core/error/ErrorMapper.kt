package com.amitia.core.error

import com.amitia.core.network.client.AmitiaApiException
import java.io.IOException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ErrorMapper @Inject constructor() {

    fun map(throwable: Throwable): AmitiaError {
        return when (throwable) {
            is AmitiaError -> throwable
            is AmitiaApiException.NetworkUnavailable -> AmitiaError.NetworkUnavailable.create(throwable)
            is AmitiaApiException.RemoteUnreachable -> AmitiaError.RemoteUnreachable.create(throwable.url, throwable)
            is AmitiaApiException.TokenExpired -> AmitiaError.TokenExpired.create(throwable)
            is AmitiaApiException.Timeout -> AmitiaError.ServiceTimeout.create("backend", throwable)
            is AmitiaApiException.SseDisconnected -> AmitiaError.StreamDisconnected.create(throwable.reason, throwable)
            is AmitiaApiException.UploadFailed -> AmitiaError.UploadFailed.create(throwable.reason ?: "上传失败", throwable)
            is AmitiaApiException.AudioFailed -> AmitiaError.AudioFailed.create(throwable.reason ?: "音频处理失败", throwable)
            is AmitiaApiException.MigrationFailed -> AmitiaError.MigrationFailed.create(throwable.reason ?: "数据迁移失败", throwable)
            is AmitiaApiException.ServerError -> AmitiaError.Unknown.create("服务器错误: HTTP ${throwable.status}", throwable)
            is AmitiaApiException.NotFound -> AmitiaError.Unknown.create("资源不存在: ${throwable.path}", throwable)
            is AmitiaApiException.Unknown -> map(throwable.raw)
            is SocketTimeoutException -> AmitiaError.ServiceTimeout.create("backend", throwable)
            is UnknownHostException -> AmitiaError.NetworkUnavailable.create(throwable)
            is IOException -> AmitiaError.NetworkUnavailable.create(throwable)
            else -> AmitiaError.Unknown.create(throwable.message, throwable)
        }
    }

    fun mapFailedState(message: String, retryable: Boolean, requiresUserAction: Boolean): AmitiaError {
        val lower = message.lowercase()
        return when {
            lower.contains("rootfs") -> AmitiaError.RootfsInstallFailed.create(message)
            lower.contains("qdrant") -> AmitiaError.QdrantStartFailed.create(message)
            lower.contains("surreal") -> AmitiaError.SurrealdbStartFailed.create(message)
            lower.contains("backend") || lower.contains("后端") -> AmitiaError.BackendStartFailed.create(message)
            lower.contains("port") || lower.contains("端口") || lower.contains("address already in use") -> {
                val portMatch = Regex("(\\d{4,5})").find(message)?.value?.toIntOrNull()
                if (portMatch != null) AmitiaError.PortConflict.create(portMatch) else AmitiaError.PortConflict.create(0, null)
            }
            lower.contains("timeout") || lower.contains("超时") -> AmitiaError.ServiceTimeout.create("runtime")
            lower.contains("permission") || lower.contains("权限") -> AmitiaError.DataDirNoPermission.create(message)
            lower.contains("killed") || lower.contains("终止") -> AmitiaError.RuntimeKilled.create()
            lower.contains("binary") || lower.contains("二进制") || lower.contains("incompatible") -> AmitiaError.BinaryIncompatible.create(message)
            !retryable && requiresUserAction -> AmitiaError.RuntimeStartFailed.create(message)
            else -> AmitiaError.RuntimeStartFailed.create(message)
        }
    }

    fun toUserMessage(error: AmitiaError): String {
        return when (error) {
            is AmitiaError.RootfsNotInstalled -> "运行时根文件系统未安装，请前往运行时管理界面下载并安装"
            is AmitiaError.RootfsInstallFailed -> "根文件系统安装失败：${error.message}"
            is AmitiaError.RuntimeStartFailed -> "运行时启动失败：${error.message}"
            is AmitiaError.QdrantStartFailed -> "向量数据库启动失败：${error.message}"
            is AmitiaError.SurrealdbStartFailed -> "SurrealDB 启动失败：${error.message}"
            is AmitiaError.BackendStartFailed -> "后端服务启动失败：${error.message}"
            is AmitiaError.PortConflict -> error.message + "，请释放端口后重试"
            is AmitiaError.BinaryIncompatible -> "二进制文件不兼容：${error.message}"
            is AmitiaError.DataDirNoPermission -> "数据目录无访问权限，请在系统设置中授予存储权限"
            is AmitiaError.ServiceTimeout -> "${error.service} 启动超时，请重试"
            is AmitiaError.TokenExpired -> "登录已过期，请重新登录"
            is AmitiaError.NetworkUnavailable -> "网络不可用，请检查网络连接"
            is AmitiaError.RemoteUnreachable -> "无法连接到远程服务，请稍后重试"
            is AmitiaError.StreamDisconnected -> "实时连接已断开，正在尝试重连"
            is AmitiaError.UploadFailed -> "文件上传失败：${error.message}"
            is AmitiaError.AudioFailed -> "音频处理失败：${error.message}"
            is AmitiaError.MigrationFailed -> "数据迁移失败：${error.message}"
            is AmitiaError.RuntimeKilled -> "运行时被系统终止，正在尝试恢复"
            is AmitiaError.Unknown -> error.message
        }
    }

    fun toDiagnosticInfo(error: AmitiaError): String {
        val builder = StringBuilder()
        builder.appendLine("Error code: ${error.code}")
        builder.appendLine("Message: ${error.message}")
        builder.appendLine("Retryable: ${error.retryable}")
        builder.appendLine("RequiresUserAction: ${error.requiresUserAction}")
        builder.appendLine("Type: ${error::class.simpleName}")
        error.cause?.let { cause ->
            builder.appendLine("Cause: ${cause::class.simpleName}: ${cause.message}")
            val stack = cause.stackTrace.take(8)
            builder.appendLine("Stack trace (top 8):")
            stack.forEach { element ->
                builder.appendLine("  at $element")
            }
        }
        return builder.toString()
    }
}
