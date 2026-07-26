package com.amitia.core.error

sealed class AmitiaError(
    val code: String,
    open override val message: String,
    val retryable: Boolean,
    val requiresUserAction: Boolean,
    open override val cause: Throwable? = null
) : RuntimeException(message, cause) {

    data class RootfsNotInstalled(
        override val message: String = "运行时根文件系统未安装",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_ROOTFS_NOT_INSTALLED, message, retryable = true, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = RootfsNotInstalled(message ?: "运行时根文件系统未安装", cause)
        }
    }

    data class RootfsInstallFailed(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_ROOTFS_INSTALL_FAILED, message, retryable = true, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = RootfsInstallFailed(message, cause)
        }
    }

    data class RuntimeStartFailed(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_RUNTIME_START_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = RuntimeStartFailed(message, cause)
        }
    }

    data class QdrantStartFailed(
        override val message: String = "Qdrant 向量数据库启动失败",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_QDRANT_START_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = QdrantStartFailed(message ?: "Qdrant 向量数据库启动失败", cause)
        }
    }

    data class SurrealdbStartFailed(
        override val message: String = "SurrealDB 启动失败",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_SURREALDB_START_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = SurrealdbStartFailed(message ?: "SurrealDB 启动失败", cause)
        }
    }

    data class BackendStartFailed(
        override val message: String = "后端服务启动失败",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_BACKEND_START_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = BackendStartFailed(message ?: "后端服务启动失败", cause)
        }
    }

    data class PortConflict(
        val port: Int,
        override val message: String = "端口 $port 已被占用",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_PORT_CONFLICT, message, retryable = true, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(port: Int, cause: Throwable? = null) = PortConflict(port, "端口 $port 已被占用", cause)
        }
    }

    data class BinaryIncompatible(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_BINARY_INCOMPATIBLE, message, retryable = false, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = BinaryIncompatible(message, cause)
        }
    }

    data class DataDirNoPermission(
        override val message: String = "数据目录无访问权限",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_DATA_DIR_NO_PERMISSION, message, retryable = false, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = DataDirNoPermission(message ?: "数据目录无访问权限", cause)
        }
    }

    data class ServiceTimeout(
        val service: String,
        override val message: String = "服务 $service 启动超时",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_SERVICE_TIMEOUT, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(service: String, cause: Throwable? = null) = ServiceTimeout(service, "服务 $service 启动超时", cause)
        }
    }

    data class TokenExpired(
        override val message: String = "认证令牌已过期，请重新登录",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_TOKEN_EXPIRED, message, retryable = false, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(cause: Throwable? = null) = TokenExpired("认证令牌已过期，请重新登录", cause)
        }
    }

    data class NetworkUnavailable(
        override val message: String = "网络不可用，请检查网络连接",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_NETWORK_UNAVAILABLE, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(cause: Throwable? = null) = NetworkUnavailable("网络不可用，请检查网络连接", cause)
        }
    }

    data class RemoteUnreachable(
        val url: String,
        override val message: String = "无法连接到远程服务",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_REMOTE_UNREACHABLE, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(url: String, cause: Throwable? = null) = RemoteUnreachable(url, "无法连接到远程服务", cause)
        }
    }

    data class StreamDisconnected(
        override val message: String = "实时连接已断开",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_STREAM_DISCONNECTED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = StreamDisconnected(message ?: "实时连接已断开", cause)
        }
    }

    data class UploadFailed(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_UPLOAD_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = UploadFailed(message, cause)
        }
    }

    data class AudioFailed(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_AUDIO_FAILED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = AudioFailed(message, cause)
        }
    }

    data class MigrationFailed(
        override val message: String,
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_MIGRATION_FAILED, message, retryable = false, requiresUserAction = true, cause = cause) {
        companion object Factory {
            fun create(message: String, cause: Throwable? = null) = MigrationFailed(message, cause)
        }
    }

    data class RuntimeKilled(
        override val message: String = "运行时被系统终止",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_RUNTIME_KILLED, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(cause: Throwable? = null) = RuntimeKilled("运行时被系统终止", cause)
        }
    }

    data class Unknown(
        override val message: String = "未知错误",
        override val cause: Throwable? = null
    ) : AmitiaError(CODE_UNKNOWN, message, retryable = true, requiresUserAction = false, cause = cause) {
        companion object Factory {
            fun create(message: String? = null, cause: Throwable? = null) = Unknown(message ?: "未知错误", cause)
        }
    }

    companion object {
        const val CODE_ROOTFS_NOT_INSTALLED = "ROOTFS_NOT_INSTALLED"
        const val CODE_ROOTFS_INSTALL_FAILED = "ROOTFS_INSTALL_FAILED"
        const val CODE_RUNTIME_START_FAILED = "RUNTIME_START_FAILED"
        const val CODE_QDRANT_START_FAILED = "QDRANT_START_FAILED"
        const val CODE_SURREALDB_START_FAILED = "SURREALDB_START_FAILED"
        const val CODE_BACKEND_START_FAILED = "BACKEND_START_FAILED"
        const val CODE_PORT_CONFLICT = "PORT_CONFLICT"
        const val CODE_BINARY_INCOMPATIBLE = "BINARY_INCOMPATIBLE"
        const val CODE_DATA_DIR_NO_PERMISSION = "DATA_DIR_NO_PERMISSION"
        const val CODE_SERVICE_TIMEOUT = "SERVICE_TIMEOUT"
        const val CODE_TOKEN_EXPIRED = "TOKEN_EXPIRED"
        const val CODE_NETWORK_UNAVAILABLE = "NETWORK_UNAVAILABLE"
        const val CODE_REMOTE_UNREACHABLE = "REMOTE_UNREACHABLE"
        const val CODE_STREAM_DISCONNECTED = "STREAM_DISCONNECTED"
        const val CODE_UPLOAD_FAILED = "UPLOAD_FAILED"
        const val CODE_AUDIO_FAILED = "AUDIO_FAILED"
        const val CODE_MIGRATION_FAILED = "MIGRATION_FAILED"
        const val CODE_RUNTIME_KILLED = "RUNTIME_KILLED"
        const val CODE_UNKNOWN = "UNKNOWN"
    }
}
