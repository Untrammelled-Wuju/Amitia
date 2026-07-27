package com.amitia.runtime.process

import android.content.Context
import android.system.Os
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.manager.RuntimeStateMachine
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class LinuxProcessManagerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val logRotator: LogRotator,
    private val stateMachine: RuntimeStateMachine,
    private val crashDataStore: ProcessCrashDataStore
) : LinuxProcessManager {

    private val filesDir: File = context.filesDir
    private val logsDir: File = File(filesDir, "runtime/logs")

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val mutex = Mutex()

    private data class ManagedProcess(
        val name: String,
        val command: List<String>,
        val process: Process,
        val pid: Int,
        val startedAt: Long,
        val env: Map<String, String>,
        val workDir: File,
        val restartPolicy: RestartPolicy,
        val statusFlow: MutableStateFlow<LinuxProcessManager.ProcessStatus>,
        val stdoutFlow: MutableSharedFlow<String>,
        val stderrFlow: MutableSharedFlow<String>,
        val monitorJob: Job,
        val outLog: File,
        val errLog: File,
        var crashCount: Int,
        var lastExitReason: String?,
        var lastExitCode: Int?,
        @Volatile var stopRequested: Boolean = false
    )

    private val processes = ConcurrentHashMap<String, ManagedProcess>()

    override suspend fun start(
        name: String,
        command: List<String>,
        env: Map<String, String>,
        workDir: File,
        restartPolicy: RestartPolicy,
        timeoutMs: Long?
    ): Result<Int> = withContext(Dispatchers.IO) {
        try {
            mutex.withLock {
                val existing = processes[name]
                if (existing != null && existing.process.isAlive) {
                    return@withContext Result.failure<Int>(
                        IllegalStateException("进程 $name 已在运行 pid=${existing.pid}")
                    )
                }
                if (existing != null) {
                    existing.monitorJob.cancel()
                    processes.remove(name)
                }
            }

            if (!workDir.exists()) workDir.mkdirs()
            logsDir.mkdirs()
            val timestamp = SimpleDateFormat(LOG_TIMESTAMP_PATTERN, Locale.US).format(Date())
            val outLog = File(logsDir, "$name-$timestamp.log")
            val errLog = File(logsDir, "$name-err-$timestamp.log")

            val builder = ProcessBuilder(command)
                .directory(workDir)
                .redirectErrorStream(false)
            builder.environment().putAll(env)

            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "启动进程 $name: ${command.joinToString(" ")} workDir=${workDir.absolutePath}"
            )

            val process = try {
                builder.start()
            } catch (ioe: java.io.IOException) {
                val msg = ioe.message ?: ""
                if (msg.contains("error=13") || msg.contains("Permission denied")) {
                    val linker = resolveLinker()
                    if (linker != null) {
                        val retryCommand = listOf(linker) + command
                        val retryBuilder = ProcessBuilder(retryCommand)
                            .directory(workDir)
                            .redirectErrorStream(false)
                        retryBuilder.environment().putAll(env)
                        stateMachine.emitLog(
                            RuntimeEvent.LogEmitted.Level.WARN,
                            TAG,
                            "进程 $name 直接执行失败(noexec),通过 $linker 重试"
                        )
                        try {
                            retryBuilder.start()
                        } catch (ioe2: java.io.IOException) {
                            stateMachine.emitLog(
                                RuntimeEvent.LogEmitted.Level.WARN,
                                TAG,
                                "进程 $name linker 重试也失败: ${ioe2.message}, 尝试可执行副本"
                            )
                            val tmpExec = tryTempExec(name, command, env, workDir)
                            if (tmpExec != null) {
                                tmpExec
                            } else {
                                throw ioe
                            }
                        }
                    } else {
                        val tmpExec = tryTempExec(name, command, env, workDir)
                        if (tmpExec != null) {
                            tmpExec
                        } else {
                            throw ioe
                        }
                    }
                } else {
                    throw ioe
                }
            }

            val pid = try {
                val method = Process::class.java.getMethod("pid")
                method.isAccessible = true
                (method.invoke(process) as Number).toInt()
            } catch (_: Exception) {
                try {
                    val field = process.javaClass.getDeclaredField("pid")
                    field.isAccessible = true
                    (field.get(process) as Number).toInt()
                } catch (_: Exception) {
                    -1
                }
            }

            val statusFlow = MutableStateFlow(LinuxProcessManager.ProcessStatus.RUNNING)
            val stdoutFlow = MutableSharedFlow<String>(
                replay = 0,
                extraBufferCapacity = STDOUT_BUFFER,
                onBufferOverflow = BufferOverflow.DROP_OLDEST
            )
            val stderrFlow = MutableSharedFlow<String>(
                replay = 0,
                extraBufferCapacity = STDOUT_BUFFER,
                onBufferOverflow = BufferOverflow.DROP_OLDEST
            )
            val startedAt = System.currentTimeMillis()
            val previousCrashData = crashDataStore.load(name)

            val outReaderJob = scope.launch {
                readStreamToLogAndFlow(process.inputStream, outLog, name, "stdout", stdoutFlow)
            }
            val errReaderJob = scope.launch {
                readStreamToLogAndFlow(process.errorStream, errLog, name, "stderr", stderrFlow)
            }

            val managed = ManagedProcess(
                name = name,
                command = command,
                process = process,
                pid = pid,
                startedAt = startedAt,
                env = env,
                workDir = workDir,
                restartPolicy = restartPolicy,
                statusFlow = statusFlow,
                stdoutFlow = stdoutFlow,
                stderrFlow = stderrFlow,
                monitorJob = Job(),
                outLog = outLog,
                errLog = errLog,
                crashCount = previousCrashData.crashCount,
                lastExitReason = previousCrashData.lastExitReason,
                lastExitCode = previousCrashData.lastExitCode
            )
            processes[name] = managed
            crashDataStore.save(
                name = name,
                data = previousCrashData.copy(lastStartedAt = startedAt)
            )

            val monitorJob = scope.launch {
                try {
                    outReaderJob.join()
                    errReaderJob.join()
                    val exitCode = process.waitFor()
                    handleProcessExit(name, exitCode)
                } catch (e: Exception) {
                    handleProcessExit(name, -1, e.message ?: "监控异常")
                }
            }
            assignMonitorJob(name, monitorJob)

            if (timeoutMs != null && timeoutMs > 0L) {
                scope.launch {
                    delay(timeoutMs)
                    val current = processes[name]
                    if (current != null && current.process.isAlive) {
                        stateMachine.emitLog(
                            RuntimeEvent.LogEmitted.Level.WARN,
                            TAG,
                            "进程 $name 启动超时 ${timeoutMs}ms,执行 destroy"
                        )
                        current.process.destroy()
                    }
                }
            }

            Result.success(pid)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "启动进程 $name 失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    override suspend fun stop(name: String, timeoutMs: Long): Result<Unit> = withContext(Dispatchers.IO) {
        val managed = processes[name]
        if (managed == null) {
            return@withContext Result.failure(IllegalStateException("进程 $name 不存在"))
        }
        try {
            managed.stopRequested = true
            managed.monitorJob.cancel()
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "停止进程 $name pid=${managed.pid} timeout=${timeoutMs}ms"
            )
            managed.process.destroy()
            val exitCode = withTimeoutOrNull(timeoutMs) {
                managed.process.waitFor()
            } ?: -1
            if (managed.process.isAlive) {
                stateMachine.emitLog(
                    RuntimeEvent.LogEmitted.Level.WARN,
                    TAG,
                    "进程 $name 优雅停止超时,执行 destroyForcibly"
                )
                managed.process.destroyForcibly()
                managed.process.waitFor()
            }
            managed.statusFlow.value = LinuxProcessManager.ProcessStatus.STOPPED
            managed.lastExitReason = "stopped"
            managed.lastExitCode = exitCode
            persistCrashData(name, managed)
            Result.success(Unit)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "停止进程 $name 失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
            Result.failure(e)
        }
    }

    override suspend fun forceStop(name: String): Result<Unit> = withContext(Dispatchers.IO) {
        val managed = processes[name]
        if (managed == null) {
            return@withContext Result.failure(IllegalStateException("进程 $name 不存在"))
        }
        try {
            managed.stopRequested = true
            managed.monitorJob.cancel()
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "强制停止进程 $name pid=${managed.pid}"
            )
            managed.process.destroyForcibly()
            val exitCode = managed.process.waitFor()
            managed.statusFlow.value = LinuxProcessManager.ProcessStatus.STOPPED
            managed.lastExitReason = "force-stopped"
            managed.lastExitCode = exitCode
            persistCrashData(name, managed)
            Result.success(Unit)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    override fun status(name: String): LinuxProcessManager.ProcessStatus {
        val managed = processes[name] ?: return LinuxProcessManager.ProcessStatus.UNKNOWN
        val current = managed.statusFlow.value
        if (current == LinuxProcessManager.ProcessStatus.RUNNING && !managed.process.isAlive) {
            return LinuxProcessManager.ProcessStatus.STOPPED
        }
        return current
    }

    override fun isAlive(name: String): Boolean {
        val managed = processes[name] ?: return false
        return managed.process.isAlive
    }

    override fun crashCount(name: String): Int {
        val managed = processes[name]
        return managed?.crashCount ?: crashDataStore.cachedCrashCount(name)
    }

    override fun lastStartedAt(name: String): Long? {
        val managed = processes[name]
        return managed?.startedAt ?: crashDataStore.cachedLastStartedAt(name)
    }

    override fun lastExitReason(name: String): String? {
        val managed = processes[name]
        return managed?.lastExitReason ?: crashDataStore.cachedLastExitReason(name)
    }

    override fun lastExitCode(name: String): Int? {
        val managed = processes[name]
        return managed?.lastExitCode ?: crashDataStore.cachedLastExitCode(name)
    }

    override fun logs(name: String, tailLines: Int): List<String> {
        val managed = processes[name] ?: return emptyList()
        val out = logRotator.readTail(managed.outLog, tailLines)
        val err = logRotator.readTail(managed.errLog, tailLines)
        val combined = ArrayList<String>(out.size + err.size)
        combined.addAll(out)
        combined.addAll(err)
        return combined
    }

    override fun logFiles(name: String): List<File> {
        if (!logsDir.exists()) return emptyList()
        val files = logsDir.listFiles { f ->
            f.isFile && (f.name.startsWith("$name-") || f.name.startsWith("$name.err-") || f.name == "$name.log" || f.name == "$name.err.log")
        }?.toList() ?: emptyList()
        return files.sortedByDescending { it.lastModified() }
    }

    override suspend fun releaseAll() = withContext(Dispatchers.IO) {
        val names = processes.keys.toList()
        for (name in names) {
            try {
                stop(name, timeoutMs = 3000L)
            } catch (_: Exception) {
                try {
                    forceStop(name)
                } catch (_: Exception) {
                }
            }
        }
        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.INFO,
            TAG,
            "已释放全部 ${names.size} 个进程"
        )
    }

    override fun observeStatus(name: String): Flow<LinuxProcessManager.ProcessStatus> {
        val managed = processes[name]
        return if (managed != null) {
            managed.statusFlow
        } else {
            flowOf(LinuxProcessManager.ProcessStatus.UNKNOWN)
        }
    }

    override fun observeStdout(name: String): Flow<String> {
        val managed = processes[name]
        return if (managed != null) {
            managed.stdoutFlow.asSharedFlow()
        } else {
            flowOf()
        }
    }

    override fun observeStderr(name: String): Flow<String> {
        val managed = processes[name]
        return if (managed != null) {
            managed.stderrFlow.asSharedFlow()
        } else {
            flowOf()
        }
    }

    private fun assignMonitorJob(name: String, job: Job) {
        val managed = processes[name] ?: return
        val updated = managed.copy(monitorJob = job)
        processes[name] = updated
    }

    private suspend fun persistCrashData(name: String, managed: ManagedProcess) {
        try {
            crashDataStore.save(
                name = name,
                data = ProcessCrashData(
                    crashCount = managed.crashCount,
                    lastExitReason = managed.lastExitReason,
                    lastExitCode = managed.lastExitCode,
                    lastStartedAt = managed.startedAt
                )
            )
        } catch (e: Exception) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "持久化进程 $name 崩溃数据失败: ${e.message}"
            )
        }
    }

    private suspend fun handleProcessExit(name: String, exitCode: Int, reason: String? = null) {
        val managed = processes[name] ?: return
        if (managed.stopRequested) {
            managed.statusFlow.value = LinuxProcessManager.ProcessStatus.STOPPED
            persistCrashData(name, managed)
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "进程 $name 因停止请求退出 code=$exitCode"
            )
            return
        }
        managed.lastExitCode = exitCode
        if (exitCode == 0) {
            managed.statusFlow.value = LinuxProcessManager.ProcessStatus.STOPPED
            managed.lastExitReason = reason ?: "exit=0"
            persistCrashData(name, managed)
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                TAG,
                "进程 $name 正常退出 code=0"
            )
            return
        }

        managed.crashCount = managed.crashCount + 1
        managed.lastExitReason = reason ?: "exit=$exitCode"
        managed.statusFlow.value = LinuxProcessManager.ProcessStatus.CRASHED
        persistCrashData(name, managed)
        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.WARN,
            TAG,
            "进程 $name 崩溃 code=$exitCode crashCount=${managed.crashCount} reason=${managed.lastExitReason}"
        )

        val policy = managed.restartPolicy
        val shouldRestart = policy.shouldRestart(managed.crashCount)
        if (!shouldRestart) return

        stateMachine.emitLog(
            RuntimeEvent.LogEmitted.Level.INFO,
            TAG,
            "进程 $name 触发重启策略=$policy attempt=${managed.crashCount}"
        )
        try {
            delay(RESTART_DELAY_MS)
            restartInternal(name)
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "进程 $name 重启失败: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
        }
    }

    private suspend fun restartInternal(name: String) {
        val managed = processes[name] ?: return
        processes.remove(name)
        try {
            start(
                name = name,
                command = managed.command,
                env = managed.env,
                workDir = managed.workDir,
                restartPolicy = managed.restartPolicy,
                timeoutMs = null
            )
        } catch (e: Exception) {
            stateMachine.emitError(
                error = "进程 $name 重启异常: ${e.message}",
                retryable = true,
                requiresUserAction = false,
                cause = e
            )
        }
    }

    private suspend fun readStreamToLogAndFlow(
        stream: java.io.InputStream,
        logFile: File,
        name: String,
        tag: String,
        flow: MutableSharedFlow<String>
    ) {
        try {
            BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { reader ->
                while (true) {
                    val line = reader.readLine() ?: break
                    logRotator.writeLine(logFile, line)
                    flow.tryEmit(line)
                }
            }
        } catch (e: Exception) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.WARN,
                TAG,
                "读取 $name/$tag 流结束: ${e.message}"
            )
        }
    }

    companion object {
        private const val TAG = "ProcessManager"
        private const val RESTART_DELAY_MS = 1000L
        private const val STDOUT_BUFFER = 256
        private const val LOG_TIMESTAMP_PATTERN = "yyyyMMdd-HHmmss"
        private const val LINKER64 = "/system/bin/linker64"
        private const val LINKER = "/system/bin/linker"
        private const val APEX_LINKER64 = "/apex/com.android.runtime/bin/linker64"
        private const val APEX_LINKER = "/apex/com.android.runtime/bin/linker"

        fun resolveLinker(): String? {
            val linker64 = File(LINKER64)
            if (linker64.exists() && linker64.canExecute()) return LINKER64
            val linker = File(LINKER)
            if (linker.exists() && linker.canExecute()) return LINKER
            val apexLinker64 = File(APEX_LINKER64)
            if (apexLinker64.exists() && apexLinker64.canExecute()) return APEX_LINKER64
            val apexLinker = File(APEX_LINKER)
            if (apexLinker.exists() && apexLinker.canExecute()) return APEX_LINKER
            return null
        }

    }

    private fun tryTempExec(
        name: String,
        command: List<String>,
        env: Map<String, String>,
        workDir: File
    ): Process? {
        if (command.isEmpty()) return null
        val origBin = File(command[0])
        val nativeLibDir = File(context.applicationInfo.nativeLibraryDir)
        val tmpDir = File("/data/local/tmp/amitia/bin").apply { mkdirs() }
        val tmpBin = File(tmpDir, origBin.name)

        val execAt = fun(dest: File): Process? {
            return try {
                origBin.copyTo(dest, overwrite = true)
                dest.setExecutable(true, false)
                dest.setReadable(true, false)
                if (!dest.canExecute()) {
                    try { Os.chmod(dest.absolutePath, 493) } catch (_: Exception) {}
                }
                val tmpCommand = listOf(dest.absolutePath) + command.drop(1)
                val tmpBuilder = ProcessBuilder(tmpCommand)
                    .directory(workDir)
                    .redirectErrorStream(false)
                tmpBuilder.environment().putAll(env)
                tmpBuilder.start()
            } catch (_: Exception) {
                null
            }
        }

        val nativeCandidate = File(nativeLibDir, "libproot_exec.so")
        if (nativeCandidate.exists() && nativeCandidate.canExecute()) {
            val nativeCommand = listOf(nativeCandidate.absolutePath) + command.drop(1)
            return try {
                val nb = ProcessBuilder(nativeCommand)
                    .directory(workDir)
                    .redirectErrorStream(false)
                nb.environment().putAll(env)
                nb.start()
            } catch (_: Exception) {
                null
            }
        }

        return execAt(tmpBin)
    }
}
