package com.amitia.runtime.bootstrap

import com.amitia.core.common.Constants
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeStage
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.amitia.runtime.health.HealthChecker
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.ProotBinaryManager
import com.amitia.runtime.linux.ProotInstallPhase
import com.amitia.runtime.linux.RootfsInstallPhase
import com.amitia.runtime.manager.RuntimeDirectories
import com.amitia.runtime.manager.RuntimeStateMachine
import com.amitia.runtime.process.LinuxProcessManager
import com.amitia.runtime.process.ProotCommandWrapper
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
    private val stateMachine: RuntimeStateMachine,
    private val prootBinaryManager: ProotBinaryManager,
    private val prootCommandWrapper: ProotCommandWrapper
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

                progress(stageOf("checking-proot", 0.31f, "检查 PRoot 二进制"))
                val prootInstallError = ensureProotReady(progress)
                if (prootInstallError != null) {
                    return@withContext fail(
                        prootInstallError,
                        retryable = false,
                        requiresUserAction = true
                    )
                }

                progress(stageOf("ensuring-rootfs", 0.33f, "准备最小化 RootFS"))
                val rootfsResult = rootfsManager.ensureMinimalRootfs()
                if (rootfsResult.isFailure) {
                    return@withContext fail(
                        rootfsResult.exceptionOrNull()?.message ?: "最小化 RootFS 创建失败",
                        retryable = true
                    )
                }

                progress(stageOf("checking-bin", 0.35f, "检查 Runtime 二进制"))
                val binCheck = checkRuntimeBinaries()
                if (binCheck.isFailure) {
                    return@withContext fail(
                        binCheck.exceptionOrNull()?.message ?: "Runtime 二进制缺失",
                        retryable = false,
                        requiresUserAction = true
                    )
                }

                progress(stageOf("checking-dirs", 0.4f, "检查数据目录"))
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

                progress(stageOf("checking-config", 0.42f, "检查配置文件"))
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

                stateMachine.transition(
                    RuntimeState.Starting(stage = "backend", progressValue = 0.45f)
                )
                progress(stageOf("starting-backend", 0.5f, "启动运行时 (PRoot)"))
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
                        "Go 后端启动超时 (SurrealDB/Qdrant/侧车由后端内部管理)",
                        retryable = true
                    )
                }
                val backendState = ServiceState.Healthy(Constants.BACKEND_PORT)

                progress(stageOf("verify-services", 0.9f, "验证子服务状态"))
                val surrealdbState = verifyService("surrealdb", surrealdbHealthUrl(), Constants.SURREALDB_PORT)
                val qdrantState = verifyService("qdrant", qdrantHealthUrl(), Constants.QDRANT_PORT)

                progress(stageOf("repository-connect", 0.95f, "重建 Repository 连接"))
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "Repository 连接已通过 ConnectionManager 重建 Endpoint (PRoot 模式 active=${prootCommandWrapper.isProotAvailable()})"
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
                    progress(stageOf("degraded", 1f, "运行时降级: ${reasons.joinToString(",")} (PRoot 模式)"))
                } else {
                    stateMachine.transition(
                        RuntimeState.Running(uptimeMs = 0L, services = services)
                    )
                    progress(stageOf("running", 1f, "运行时已启动 (PRoot 模式)"))
                }

                Result.success(services)
            } catch (e: Exception) {
                fail(e.message ?: "启动异常", retryable = true, cause = e)
            }
        }

    private suspend fun ensureProotReady(progress: (RuntimeStage) -> Unit): String? {
        if (prootBinaryManager.isAvailable()) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "PRoot 二进制已就绪 version=${prootBinaryManager.version()}"
            )
            return null
        }
        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.INFO,
            TAG,
            "PRoot 二进制未就绪,触发安装"
        )
        var installError: String? = null
        try {
            prootBinaryManager.install().collect { p ->
                progress(stageOf("installing-proot", 0.31f + p.percent * 0.02f, p.message))
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.INFO,
                    TAG,
                    "PRoot 安装: phase=${p.phase} pct=${p.percent} msg=${p.message}"
                )
                if (p.phase == ProotInstallPhase.FAILED) {
                    installError = p.error ?: p.message
                }
            }
        } catch (e: Exception) {
            installError = e.message ?: "PRoot 安装异常"
        }
        if (installError != null) return installError
        if (!prootBinaryManager.isAvailable()) {
            return prootBinaryManager.unavailableReason() ?: "PRoot 安装完成但仍不可用"
        }
        return null
    }

    override suspend fun stop(progress: (RuntimeStage) -> Unit): Result<Unit> =
        withContext(Dispatchers.IO) {
            try {
                stateMachine.transition(RuntimeState.Stopping(stage = "reject"))
                progress(stageOf("stopping-reject", 0.1f, "停止接受新请求"))

                progress(stageOf("stopping-backend", 0.5f, "停止 Go 后端 (含 SurrealDB/Qdrant/侧车)"))
                runCatching { processManager.stop(BACKEND_PROC_NAME, timeoutMs = 15000L) }

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

    private suspend fun startBackend(): Result<Int> {
        val workDir = directories.amitiaDataRoot().apply { mkdirs() }
        val configPath = File(directories.configDir(), CONFIG_FILE_NAME)
        val rawCommand = buildBackendCommand(directories.binDir(), configPath, workDir)
        val env = buildBackendEnv(configPath)
        val command = wrapWithProot(rawCommand, env, workDir)
        return processManager.start(
            name = BACKEND_PROC_NAME,
            command = command,
            env = env,
            workDir = workDir,
            restartPolicy = RestartPolicy.ON_FAILURE,
            timeoutMs = null
        )
    }

    private fun wrapWithProot(
        command: List<String>,
        env: Map<String, String>,
        workDir: File
    ): List<String> {
        if (!prootCommandWrapper.isProotAvailable()) {
            stateMachine.emitError(
                error = "PRoot 不可用: ${prootCommandWrapper.fallbackReason() ?: "未知原因"}",
                retryable = false,
                requiresUserAction = true
            )
            return command
        }
        return prootCommandWrapper.wrap(command, env, workDir)
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
        return Result.success(Unit)
    }

    private suspend fun verifyService(name: String, healthUrl: String, port: Int): ServiceState {
        val healthy = healthChecker.waitForHealthy(
            name = name,
            check = {
                healthChecker.checkHttp(
                    url = healthUrl,
                    timeoutMs = HEALTH_CHECK_TIMEOUT_MS
                )
            },
            intervalMs = HEALTH_CHECK_INTERVAL_MS,
            timeoutMs = SERVICE_VERIFY_TIMEOUT_MS
        )
        return if (healthy.isSuccess) {
            ServiceState.Healthy(port)
        } else {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "$name 未就绪 (后端已启动,子服务可能仍在初始化)"
            )
            ServiceState.Unhealthy("$name 验证超时")
        }
    }

    private fun buildBackendCommand(binDir: File, configPath: File, dataDir: File): List<String> {
        val binary = File(binDir, BACKEND_BINARY).absolutePath
        return listOf(binary)
    }

    private fun buildBackendEnv(configPath: File): Map<String, String> {
        return mapOf(
            "CONFIG_PATH" to (configPath.parentFile?.absolutePath ?: configPath.absolutePath),
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
        private const val BACKEND_PROC_NAME = "amitia-backend"
        private const val HEALTH_CHECK_TIMEOUT_MS = 2000L
        private const val HEALTH_CHECK_INTERVAL_MS = 500L
        private const val BACKEND_READY_TIMEOUT_MS = 120_000L
        private const val SERVICE_VERIFY_TIMEOUT_MS = 10_000L
    }
}
