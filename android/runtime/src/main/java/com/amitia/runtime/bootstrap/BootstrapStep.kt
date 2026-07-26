package com.amitia.runtime.bootstrap

import com.amitia.runtime.api.RuntimeStageInfo

sealed class BootstrapStep {

    abstract val stageInfo: RuntimeStageInfo

    data class RootfsCheck(
        override val stageInfo: RuntimeStageInfo
    ) : BootstrapStep()

    data class RuntimeFilesCheck(
        override val stageInfo: RuntimeStageInfo,
        val missingFiles: List<String> = emptyList()
    ) : BootstrapStep()

    data class DataDirCheck(
        override val stageInfo: RuntimeStageInfo,
        val createdDirs: List<String> = emptyList()
    ) : BootstrapStep()

    data class ConfigCheck(
        override val stageInfo: RuntimeStageInfo,
        val configPath: String? = null
    ) : BootstrapStep()

    data class StartSurreal(
        override val stageInfo: RuntimeStageInfo,
        val pid: Int? = null
    ) : BootstrapStep()

    data class WaitSurrealHealthy(
        override val stageInfo: RuntimeStageInfo,
        val port: Int,
        val healthy: Boolean = false
    ) : BootstrapStep()

    data class StartQdrant(
        override val stageInfo: RuntimeStageInfo,
        val pid: Int? = null
    ) : BootstrapStep()

    data class WaitQdrantHealthy(
        override val stageInfo: RuntimeStageInfo,
        val port: Int,
        val healthy: Boolean = false
    ) : BootstrapStep()

    data class StartBackend(
        override val stageInfo: RuntimeStageInfo,
        val pid: Int? = null
    ) : BootstrapStep()

    data class WaitBackendHealthy(
        override val stageInfo: RuntimeStageInfo,
        val port: Int,
        val healthy: Boolean = false
    ) : BootstrapStep()

    data class RepositoryConnect(
        override val stageInfo: RuntimeStageInfo,
        val connected: Boolean = false
    ) : BootstrapStep()

    data class Complete(
        override val stageInfo: RuntimeStageInfo
    ) : BootstrapStep()

    data class Failed(
        override val stageInfo: RuntimeStageInfo,
        val cause: Throwable? = null
    ) : BootstrapStep()

    companion object {
        fun rootfsCheck(progress: Float, message: String): RootfsCheck = RootfsCheck(
            stageInfo = RuntimeStageInfo(
                stage = "RootfsCheck",
                progress = progress,
                readableMessage = message
            )
        )

        fun runtimeFilesCheck(progress: Float, message: String, missing: List<String> = emptyList()): RuntimeFilesCheck =
            RuntimeFilesCheck(
                stageInfo = RuntimeStageInfo(
                    stage = "RuntimeFilesCheck",
                    progress = progress,
                    readableMessage = message,
                    errorCause = if (missing.isEmpty()) null else missing.joinToString(","),
                    needsUserAction = missing.isNotEmpty()
                ),
                missingFiles = missing
            )

        fun dataDirCheck(progress: Float, message: String, created: List<String> = emptyList()): DataDirCheck =
            DataDirCheck(
                stageInfo = RuntimeStageInfo(
                    stage = "DataDirCheck",
                    progress = progress,
                    readableMessage = message
                ),
                createdDirs = created
            )

        fun configCheck(progress: Float, message: String, path: String? = null): ConfigCheck =
            ConfigCheck(
                stageInfo = RuntimeStageInfo(
                    stage = "ConfigCheck",
                    progress = progress,
                    readableMessage = message,
                    errorCause = if (path == null) "配置文件缺失" else null,
                    needsUserAction = path == null
                ),
                configPath = path
            )

        fun startSurreal(progress: Float, message: String, pid: Int? = null): StartSurreal = StartSurreal(
            stageInfo = RuntimeStageInfo(
                stage = "StartSurreal",
                progress = progress,
                readableMessage = message
            ),
            pid = pid
        )

        fun waitSurrealHealthy(progress: Float, port: Int, healthy: Boolean): WaitSurrealHealthy =
            WaitSurrealHealthy(
                stageInfo = RuntimeStageInfo(
                    stage = "WaitSurrealHealthy",
                    progress = progress,
                    readableMessage = if (healthy) "SurrealDB 健康" else "等待 SurrealDB 健康"
                ),
                port = port,
                healthy = healthy
            )

        fun startQdrant(progress: Float, message: String, pid: Int? = null): StartQdrant = StartQdrant(
            stageInfo = RuntimeStageInfo(
                stage = "StartQdrant",
                progress = progress,
                readableMessage = message
            ),
            pid = pid
        )

        fun waitQdrantHealthy(progress: Float, port: Int, healthy: Boolean): WaitQdrantHealthy =
            WaitQdrantHealthy(
                stageInfo = RuntimeStageInfo(
                    stage = "WaitQdrantHealthy",
                    progress = progress,
                    readableMessage = if (healthy) "Qdrant 健康" else "等待 Qdrant 健康"
                ),
                port = port,
                healthy = healthy
            )

        fun startBackend(progress: Float, message: String, pid: Int? = null): StartBackend = StartBackend(
            stageInfo = RuntimeStageInfo(
                stage = "StartBackend",
                progress = progress,
                readableMessage = message
            ),
            pid = pid
        )

        fun waitBackendHealthy(progress: Float, port: Int, healthy: Boolean): WaitBackendHealthy =
            WaitBackendHealthy(
                stageInfo = RuntimeStageInfo(
                    stage = "WaitBackendHealthy",
                    progress = progress,
                    readableMessage = if (healthy) "Go 后端健康" else "等待 Go 后端健康"
                ),
                port = port,
                healthy = healthy
            )

        fun repositoryConnect(progress: Float, connected: Boolean): RepositoryConnect = RepositoryConnect(
            stageInfo = RuntimeStageInfo(
                stage = "RepositoryConnect",
                progress = progress,
                readableMessage = if (connected) "Repository 已连接" else "重建 Repository 连接"
            ),
            connected = connected
        )

        fun complete(): Complete = Complete(
            stageInfo = RuntimeStageInfo(
                stage = "Complete",
                progress = 1f,
                readableMessage = "Runtime 已启动"
            )
        )

        fun failed(message: String, cause: Throwable? = null, retryable: Boolean = true): Failed = Failed(
            stageInfo = RuntimeStageInfo(
                stage = "Failed",
                progress = 0f,
                readableMessage = message,
                errorCause = message,
                isRetryable = retryable,
                needsUserAction = !retryable
            ),
            cause = cause
        )
    }
}
