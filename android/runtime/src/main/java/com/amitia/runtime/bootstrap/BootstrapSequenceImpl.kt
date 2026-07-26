package com.amitia.runtime.bootstrap

import com.amitia.core.common.Constants
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeStage
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.amitia.runtime.health.HealthChecker
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.RootfsInstallPhase
import com.amitia.runtime.manager.RuntimeDirectories
import com.amitia.runtime.manager.RuntimeStateMachine
import com.amitia.runtime.process.LinuxProcessManager
import com.amitia.runtime.process.RestartPolicy
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.withContext
import java.io.File
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class BootstrapSequenceImpl @Inject constructor(
    private val directories: RuntimeDirectories,
    private val rootfsManager: LinuxRootfsManager,
    private val processManager: LinuxProcessManager,
    private val healthChecker: HealthChecker,
    private val stateMachine: RuntimeStateMachine
) : BootstrapSequence {

    private val localAuthToken: String = UUID.randomUUID().toString().replace("-", "")

    override suspend fun start(progress: (RuntimeStage) -> Unit): Result<RuntimeServices> =
        withContext(Dispatchers.IO) {
            try {
                progress(stageOf("preparing", 0.05f, "检查 RootFS 安装状态"))

                if (!rootfsManager.isInstalled()) {
                    stateMachine.transition(
                        RuntimeState.Installing(progressValue = 0.1f, message = "正在安装 RootFS")
                    )
                    progress(stageOf("installing", 0.1f, "安装 RootFS"))
                    val installError = collectInstall { pct, msg ->
                        progress(stageOf("installing", 0.1f + pct * 0.2f, msg))
                    }
                    if (installError != null) {
                        return@withContext fail(
                            installError,
                            retryable = true
                        )
                    }
                }

                stateMachine.transition(RuntimeState.Installed)
                progress(stageOf("installed", 0.3f, "RootFS 已就绪"))

                progress(stageOf("checking-bin", 0.32f, "检查 Runtime 二进制"))
                val binCheck = checkRuntimeBinaries()
                if (binCheck.isFailure) {
                    return@withContext fail(
                        binCheck.exceptionOrNull()?.message ?: "Runtime 二进制缺失",
                        retryable = false,
                        requiresUserAction = true
                    )
                }

                progress(stageOf("checking-dirs", 0.35f, "检查数据目录"))
                val dirResult = directories.ensureAllCreated()
                if (dirResult.isFailure) {
                    return@withContext fail(
                        dirResult.exceptionOrNull()?.message ?: "数据目录创建失败",
                        retryable = true
                    )
                }
                if (!directories.validateIsolation()) {
                    return@withContext fail(
                        "RootFS 与用户数据目录未隔离",
                        retryable = false,
                        requiresUserAction = true
                    )
                }

                progress(stageOf("checking-config", 0.4f, "检查配置文件"))
                val configFile = File(directories.configDir(), CONFIG_FILE_NAME)
                if (!configFile.exists()) {
                    configFile.parentFile?.mkdirs()
                    configFile.writeText(buildDefaultConfig())
                    stateMachine.emitLog(
                        RuntimeEvent.LogEmitted.Level.INFO,
                        TAG,
                        "配置文件缺失,已自动生成: ${configFile.absolutePath}"
                    )
                }

                var surrealdbState: ServiceState = ServiceState.Stopped
                var qdrantState: ServiceState = ServiceState.Stopped
                var backendState: ServiceState = ServiceState.Stopped

                stateMachine.transition(
                    RuntimeState.Starting(stage = "surrealdb", progressValue = 0.45f)
                )
                progress(stageOf("starting-surrealdb", 0.45f, "启动 SurrealDB"))
                val surrealdbResult = startSurrealdb()
                if (surrealdbResult.isFailure) {
                    stateMachine.emitLog(
                        RuntimeEvent.LogEmitted.Level.WARN,
                        TAG,
                        "SurrealDB 启动失败(非致命): ${surrealdbResult.exceptionOrNull()?.message}"
                    )
                    surrealdbState = ServiceState.Unhealthy(
                        surrealdbResult.exceptionOrNull()?.message ?: "SurrealDB 启动失败"
                    )
                } else {
                    val healthy = healthChecker.waitForHealthy(
                        name = "surrealdb",
                        check = {
                            healthChecker.checkHttp(
                                url = surrealdbHealthUrl(),
                                timeoutMs = HEALTH_CHECK_TIMEOUT_MS
                            )
                        },
                        intervalMs = HEALTH_CHECK_INTERVAL_MS,
                        timeoutMs = SURREALDB_READY_TIMEOUT_MS
                    )
                    surrealdbState = if (healthy.isSuccess) {
                        ServiceState.Healthy(Constants.SURREALDB_PORT)
                    } else {
                        ServiceState.Unhealthy("SurrealDB 健康检查超时")
                    }
                }

                stateMachine.transition(
                    RuntimeState.Starting(stage = "qdrant", progressValue = 0.6f)
                )
                progress(stageOf("starting-qdrant", 0.6f, "启动 Qdrant"))
                val qdrantResult = startQdrant()
                if (qdrantResult.isFailure) {
                    stateMachine.emitLog(
                        RuntimeEvent.LogEmitted.Level.WARN,
                        TAG,
                        "Qdrant 启动失败(非致命): ${qdrantResult.exceptionOrNull()?.message}"
                    )
                    qdrantState = ServiceState.Unhealthy(
                        qdrantResult.exceptionOrNull()?.message ?: "Qdrant 启动失败"
                    )
                } else {
                    val healthy = healthChecker.waitForHealthy(
                        name = "qdrant",
                        check = {
                            healthChecker.checkHttp(
                                url = qdrantHealthUrl(),
                                timeoutMs = HEALTH_CHECK_TIMEOUT_MS
                            )
                        },
                        intervalMs = HEALTH_CHECK_INTERVAL_MS,
                        timeoutMs = QDRANT_READY_TIMEOUT_MS
                    )
                    qdrantState = if (healthy.isSuccess) {
                        ServiceState.Healthy(Constants.QDRANT_PORT)
                    } else {
                        ServiceState.Unhealthy("Qdrant 健康检查超时")
                    }
                }

                stateMachine.transition(
                    RuntimeState.Starting(stage = "backend", progressValue = 0.75f)
                )
                progress(stageOf("starting-backend", 0.75f, "启动 Go 后端"))
                val backendResult = startBackend()
                if (backendResult.isFailure) {
                    return@withContext fail(
                        backendResult.exceptionOrNull()?.message ?: "Go 后端启动失败",
                        retryable = true
                    )
                }
                val backendHealthy = healthChecker.waitForHealthy(
                    name = "backend",
                    check = {
                        healthChecker.checkHttp(
                            url = backendHealthUrl(),
                            timeoutMs = HEALTH_CHECK_TIMEOUT_MS
                        )
                    },
                    intervalMs = HEALTH_CHECK_INTERVAL_MS,
                    timeoutMs = BACKEND_READY_TIMEOUT_MS
                )
                if (backendHealthy.isFailure) {
                    return@withContext fail(
                        "Go 后端健康检查超时",
                        retryable = true
                    )
                }
                backendState = ServiceState.Healthy(Constants.BACKEND_PORT)

                progress(stageOf("repository-connect", 0.95f, "重建 Repository 连接"))
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "Repository 连接已通过 ConnectionManager 重建 Endpoint"
                )

                val services = RuntimeServices(
                    surrealDb = surrealdbState,
                    qdrant = qdrantState,
                    backend = backendState
                )

                val hasDegraded = surrealdbState is ServiceState.Unhealthy ||
                    qdrantState is ServiceState.Unhealthy
                if (hasDegraded) {
                    val reasons = buildList {
                        if (surrealdbState is ServiceState.Unhealthy) add("surrealdb")
                        if (qdrantState is ServiceState.Unhealthy) add("qdrant")
                    }
                    stateMachine.transition(
                        RuntimeState.Degraded(reason = reasons.joinToString(","), services = services)
                    )
                    progress(stageOf("degraded", 1f, "运行时降级: ${reasons.joinToString(",")}"))
                } else {
                    stateMachine.transition(
                        RuntimeState.Running(uptimeMs = 0L, services = services)
                    )
                    progress(stageOf("running", 1f, "运行时已启动"))
                }

                Result.success(services)
            } catch (e: Exception) {
                fail(e.message ?: "启动异常", retryable = true, cause = e)
            }
        }

    override suspend fun stop(progress: (RuntimeStage) -> Unit): Result<Unit> =
        withContext(Dispatchers.IO) {
            try {
                stateMachine.transition(RuntimeState.Stopping(stage = "reject"))
                progress(stageOf("stopping-reject", 0.1f, "停止接受新请求"))

                progress(stageOf("stopping-backend", 0.3f, "停止 Go 后端"))
                runCatching { processManager.stop(BACKEND_PROC_NAME, timeoutMs = 10000L) }

                progress(stageOf("stopping-qdrant", 0.6f, "停止 Qdrant"))
                runCatching { processManager.stop(QDRANT_PROC_NAME, timeoutMs = 8000L) }

                progress(stageOf("stopping-surrealdb", 0.85f, "停止 SurrealDB"))
                runCatching { processManager.stop(SURREALDB_PROC_NAME, timeoutMs = 8000L) }

                stateMachine.transition(RuntimeState.Stopped)
                progress(stageOf("stopped", 1f, "运行时已停止"))
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "Runtime 已停止,日志已刷新"
                )
                Result.success(Unit)
            } catch (e: Exception) {
                stateMachine.emitError(
                    error = "停止失败: ${e.message}",
                    retryable = true,
                    requiresUserAction = false,
                    cause = e
                )
                Result.failure(e)
            }
        }

    override suspend fun restart(): Result<RuntimeServices> = withContext(Dispatchers.IO) {
        val stopResult = stop { }
        if (stopResult.isFailure) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "重启时停止阶段失败,继续尝试启动"
            )
        }
        start { }
    }

    override suspend fun repair(): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "开始修复 Runtime"
            )
            runCatching { processManager.releaseAll() }

            stateMachine.transition(RuntimeState.Stopping(stage = "repair"))
            val verify = rootfsManager.verify()
            if (verify.isFailure || (verify.getOrNull()?.valid == false)) {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.WARN,
                    TAG,
                    "RootFS 校验失败,尝试重新安装"
                )
                val installError = collectInstall { pct, msg ->
                    stateMachine.emitLog(
                        RuntimeEvent.LogEmitted.Level.INFO,
                        TAG,
                        "RootFS 重装进度: pct=$pct msg=$msg"
                    )
                }
                if (installError != null) {
                    return@withContext fail(
                        installError,
                        retryable = true
                    ).map { }
                }
            }

            val dirs = directories.ensureAllCreated()
            if (dirs.isFailure) {
                return@withContext fail(
                    dirs.exceptionOrNull()?.message ?: "目录重建失败",
                    retryable = true
                ).map { }
            }

            val startResult = start { }
            if (startResult.isFailure) {
                return@withContext fail(
                    startResult.exceptionOrNull()?.message ?: "修复后启动失败",
                    retryable = true
                ).map { }
            }
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "Runtime 修复完成"
            )
            Result.success(Unit)
        } catch (e: Exception) {
            fail(e.message ?: "修复异常", retryable = true, cause = e).map { }
        }
    }

    private suspend fun collectInstall(progress: (Float, String) -> Unit): String? {
        var failed: String? = null
        try {
            rootfsManager.install().collect { p ->
                progress(p.percent, p.message)
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "RootFS 安装: phase=${p.phase} pct=${p.percent} msg=${p.message}"
                )
                if (p.phase == RootfsInstallPhase.FAILED) {
                    failed = p.error ?: p.message
                }
            }
        } catch (e: Exception) {
            failed = e.message ?: "RootFS 安装异常"
        }
        return failed
    }

    private suspend fun startSurrealdb(): Result<Int> {
        val workDir = directories.surrealdbDir().apply { mkdirs() }
        val dataPath = File(workDir, "graph.db")
        val command = buildSurrealdbCommand(workDir, dataPath)
        val env = mapOf(
            "HOME" to workDir.absolutePath,
            "TMPDIR" to directories.tmpDir().absolutePath
        )
        return processManager.start(
            name = SURREALDB_PROC_NAME,
            command = command,
            env = env,
            workDir = workDir,
            restartPolicy = RestartPolicy.ALWAYS,
            timeoutMs = null
        )
    }

    private suspend fun startQdrant(): Result<Int> {
        val workDir = directories.qdrantDir().apply { mkdirs() }
        val dataPath = File(workDir, "storage")
        val configPath = File(workDir, "config.yaml")
        if (!configPath.exists()) {
            configPath.writeText(buildQdrantConfig())
        }
        val command = buildQdrantCommand(workDir, dataPath)
        val env = mapOf(
            "HOME" to workDir.absolutePath,
            "TMPDIR" to directories.tmpDir().absolutePath,
            "QDRANT__SERVICE__HTTP_PORT" to Constants.QDRANT_PORT.toString(),
            "QDRANT__SERVICE__GRPC_PORT" to (Constants.QDRANT_PORT + 1).toString()
        )
        return processManager.start(
            name = QDRANT_PROC_NAME,
            command = command,
            env = env,
            workDir = workDir,
            restartPolicy = RestartPolicy.ALWAYS,
            timeoutMs = null
        )
    }

    private suspend fun startBackend(): Result<Int> {
        val workDir = directories.amitiaDataRoot().apply { mkdirs() }
        val configPath = File(directories.configDir(), CONFIG_FILE_NAME)
        val command = buildBackendCommand(directories.binDir(), configPath, workDir)
        val env = buildBackendEnv(configPath)
        return processManager.start(
            name = BACKEND_PROC_NAME,
            command = command,
            env = env,
            workDir = workDir,
            restartPolicy = RestartPolicy.ON_FAILURE,
            timeoutMs = null
        )
    }

    private fun checkRuntimeBinaries(): Result<Unit> {
        val bin = directories.binDir()
        val backend = File(bin, BACKEND_BINARY)
        if (!backend.exists()) {
            return Result.failure(IllegalStateException("后端二进制缺失: ${backend.absolutePath}"))
        }
        if (!backend.canExecute()) {
            backend.setExecutable(true, false)
        }
        val qdrant = File(bin, QDRANT_BINARY)
        if (!qdrant.exists()) {
            return Result.failure(IllegalStateException("Qdrant 二进制缺失: ${qdrant.absolutePath}"))
        }
        if (!qdrant.canExecute()) {
            qdrant.setExecutable(true, false)
        }
        val surreal = File(bin, SURREAL_BINARY)
        if (!surreal.exists()) {
            return Result.failure(IllegalStateException("SurrealDB 二进制缺失: ${surreal.absolutePath}"))
        }
        if (!surreal.canExecute()) {
            surreal.setExecutable(true, false)
        }
        return Result.success(Unit)
    }

    private fun buildSurrealdbCommand(workDir: File, dataPath: File): List<String> {
        val binary = File(directories.binDir(), SURREAL_BINARY).absolutePath
        return listOf(
            binary,
            "start",
            "--log", "info",
            "--user", "root",
            "--password", "root",
            "--bind", "${Constants.LOCAL_HOST}:${Constants.SURREALDB_PORT}",
            "file:${dataPath.absolutePath}"
        )
    }

    private fun buildQdrantCommand(workDir: File, dataPath: File): List<String> {
        val binary = File(directories.binDir(), QDRANT_BINARY).absolutePath
        val configPath = File(workDir, "config.yaml")
        return listOf(
            binary,
            "--config-path", configPath.absolutePath
        )
    }

    private fun buildBackendCommand(binDir: File, configPath: File, dataDir: File): List<String> {
        val binary = File(binDir, BACKEND_BINARY).absolutePath
        return listOf(binary)
    }

    private fun buildBackendEnv(configPath: File): Map<String, String> {
        return mapOf(
            "CONFIG_PATH" to configPath.parentFile.absolutePath,
            "STORAGE_DATADIR" to directories.sqliteDir().absolutePath,
            "AMITIA_DATA_DIR" to directories.amitiaDataRoot().absolutePath,
            "AMITIA_SQLITE_DIR" to directories.sqliteDir().absolutePath,
            "QDRANT_HOST" to Constants.LOCAL_HOST,
            "QDRANT_PORT" to Constants.QDRANT_PORT.toString(),
            "SURREALDB_HOST" to Constants.LOCAL_HOST,
            "SURREALDB_PORT" to Constants.SURREALDB_PORT.toString(),
            "SURREALDB_DATAPATH" to directories.surrealdbDir().absolutePath,
            "AMITIA_EMBEDDED" to "android",
            "LOCAL_AUTH_TOKEN" to localAuthToken,
            "HOME" to directories.amitiaDataRoot().absolutePath,
            "TMPDIR" to directories.tmpDir().absolutePath,
            "GOGC" to "50"
        )
    }

    private fun buildQdrantConfig(): String {
        return buildString {
            appendLine("service:")
            appendLine("  host: ${Constants.LOCAL_HOST}")
            appendLine("  http_port: ${Constants.QDRANT_PORT}")
            appendLine("  grpc_port: ${Constants.QDRANT_PORT + 1}")
            appendLine("storage:")
            appendLine("  storage_path: ./storage")
            appendLine("telemetry_disabled: true")
        }
    }

    private fun buildDefaultConfig(): String {
        val dataDir = directories.amitiaDataRoot().absolutePath
        return buildString {
            appendLine("server:")
            appendLine("  port: ${Constants.BACKEND_PORT}")
            appendLine("  host: \"${Constants.LOCAL_HOST}\"")
            appendLine("  mode: \"release\"")
            appendLine("storage:")
            appendLine("  dataDir: \"$dataDir\"")
            appendLine("jwt:")
            appendLine("  secret: \"AMITIA_LOCAL_AUTH_SECRET\"")
            appendLine("  expireDays: 7")
            appendLine("app:")
            appendLine("  name: \"Amitia\"")
            appendLine("  version: \"android-local\"")
            appendLine("  deployMode: \"android-embedded\"")
            appendLine("qdrant:")
            appendLine("  host: \"${Constants.LOCAL_HOST}\"")
            appendLine("  port: ${Constants.QDRANT_PORT}")
            appendLine("surrealdb:")
            appendLine("  host: \"${Constants.LOCAL_HOST}\"")
            appendLine("  port: ${Constants.SURREALDB_PORT}")
            appendLine("  namespace: \"uai\"")
            appendLine("  database: \"memory_graph\"")
            appendLine("  username: \"root\"")
            appendLine("  password: \"root\"")
            appendLine("  dataPath: \"${directories.surrealdbDir().absolutePath}/graph.db\"")
        }
    }

    private fun surrealdbHealthUrl(): String =
        "http://${Constants.LOCAL_HOST}:${Constants.SURREALDB_PORT}/health"

    private fun qdrantHealthUrl(): String =
        "http://${Constants.LOCAL_HOST}:${Constants.QDRANT_PORT}/healthz"

    private fun backendHealthUrl(): String =
        "http://${Constants.LOCAL_HOST}:${Constants.BACKEND_PORT}/api/health"

    private fun stageOf(stage: String, progress: Float, message: String): RuntimeStage =
        RuntimeStage(
            stage = stage,
            progress = progress,
            message = message,
            error = null,
            retryable = true,
            requiresUserAction = false
        )

    private suspend fun fail(
        message: String,
        retryable: Boolean,
        requiresUserAction: Boolean = false,
        cause: Throwable? = null
    ): Result<RuntimeServices> {
        stateMachine.transition(
            RuntimeState.Failed(
                errorMessage = message,
                retryable = retryable,
                requiresUserAction = requiresUserAction
            )
        )
        stateMachine.emitError(message, retryable, requiresUserAction, cause)
        return Result.failure(cause ?: IllegalStateException(message))
    }

    companion object {
        private const val TAG = "Bootstrap"
        private const val CONFIG_FILE_NAME = "config.yml"
        private const val BACKEND_BINARY = "amitia-backend-arm64"
        private const val QDRANT_BINARY = "qdrant_linux_aarch64"
        private const val SURREAL_BINARY = "surreal_linux_aarch64"
        private const val BACKEND_PROC_NAME = "amitia-backend"
        private const val QDRANT_PROC_NAME = "qdrant"
        private const val SURREALDB_PROC_NAME = "surrealdb"
        private const val HEALTH_CHECK_TIMEOUT_MS = 2000L
        private const val HEALTH_CHECK_INTERVAL_MS = 500L
        private const val SURREALDB_READY_TIMEOUT_MS = 30_000L
        private const val QDRANT_READY_TIMEOUT_MS = 30_000L
        private const val BACKEND_READY_TIMEOUT_MS = 60_000L
    }
}
