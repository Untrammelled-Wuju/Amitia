package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotExit
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.proot.ProotTerminationResult
import java.io.File
import java.util.concurrent.CompletableFuture
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal class DefaultProotSession(
    override val sessionId: String,
    private val process: Process,
    private val observer: ProotObserver,
    private val generation: Long,
    private val watcherExecutor: java.util.concurrent.ExecutorService = java.util.concurrent.Executors.newSingleThreadExecutor { r ->
        Thread(r, "proot-exit-watcher-$sessionId").apply { isDaemon = true }
    },
) : ProotSession {

    private val closed = AtomicBoolean(false)
    private val stopRequested = AtomicBoolean(false)
    private val terminalPublished = AtomicBoolean(false)
    private val watcherFailurePublished = AtomicBoolean(false)
    private val exitDeferred = CompletableFuture<ProotExit>()
    private val exitRef = AtomicReference<ProotExit?>(null)
    private val stdoutPump: StreamPump
    private val stderrPump: StreamPump
    private val started = AtomicBoolean(false)
    private val activated = AtomicBoolean(false)
    private val watcherStarted = AtomicBoolean(false)
    private val watcherFailureRef = AtomicReference<String?>(null)

    init {
        stdoutPump = InputStreamPump(process.inputStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stdout(sessionId, data, seq)) }
        stderrPump = InputStreamPump(process.errorStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stderr(sessionId, data, seq)) }
    }

    override fun isAlive(): Boolean {
        if (closed.get()) return false
        return try {
            process.exitValue()
            false
        } catch (e: IllegalThreadStateException) {
            true
        }
    }

    override fun requestStop() {
        stopRequested.set(true)
    }

    override val exit: ProotExit?
        get() = exitRef.get()

    override fun awaitExit(timeoutMillis: Long): Int? {
        val existing = exitRef.get()
        if (existing != null) return existing.exitCode
        if (timeoutMillis <= 0L) return exitRef.get()?.exitCode
        return try {
            exitDeferred.get(timeoutMillis, java.util.concurrent.TimeUnit.MILLISECONDS)?.exitCode
        } catch (e: java.util.concurrent.TimeoutException) {
            exitRef.get()?.exitCode
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            exitRef.get()?.exitCode
        } catch (e: java.util.concurrent.ExecutionException) {
            throw IllegalStateException(e.cause?.message ?: e.message, e.cause ?: e)
        }
    }

    override fun stop(graceMillis: Long): ProotStopResult {
        val existingExit = exitRef.get()
        if (closed.get() || existingExit != null || !process.isAlive) {
            val code = existingExit?.exitCode ?: try { process.exitValue() } catch (originalError: IllegalThreadStateException) { null }
            return ProotStopResult.AlreadyStopped(sessionId, code)
        }
        stopRequested.set(true)
        val processTreeSnapshot = snapshotProcessTree()
        return try {
            process.destroy()
            val graceful = try {
                exitDeferred.get(graceMillis, java.util.concurrent.TimeUnit.MILLISECONDS)
            } catch (originalError77: java.util.concurrent.TimeoutException) {
                null
            }
            if (graceful != null) {
                terminateSnapshotProcesses(processTreeSnapshot)
                val code = exitRef.get()?.exitCode
                ProotStopResult.Graceful(sessionId, code)
            } else {
                forceTerminateProcessTree(processTreeSnapshot)
                try { exitDeferred.get(5, java.util.concurrent.TimeUnit.SECONDS) } catch (originalError85: java.util.concurrent.TimeoutException) { }
                val code = exitRef.get()?.exitCode
                ProotStopResult.Forced(sessionId, code)
            }
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            forceTerminateProcessTree(processTreeSnapshot)
            try { exitDeferred.get(5, java.util.concurrent.TimeUnit.SECONDS) } catch (originalError92: java.util.concurrent.TimeoutException) { }
            val code = exitRef.get()?.exitCode
            ProotStopResult.Forced(sessionId, code)
        } catch (e: Exception) {
            forceTerminateProcessTree(processTreeSnapshot)
            ProotStopResult.Failed(sessionId, com.amitia.amitia_app.runtime.proot.ProotErrorCode.PROCESS_STOP_FAILED, e.message ?: "error")
        }
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            try { stdoutPump.stop() } catch (originalError103: Throwable) {
              originalError103.printStackTrace()
            }
            try { stderrPump.stop() } catch (originalError104: Throwable) {
              originalError104.printStackTrace()
            }
            val processTreeSnapshot = snapshotProcessTree()
            try { if (process.isAlive) forceTerminateProcessTree(processTreeSnapshot) } catch (originalError105: Throwable) {
              originalError105.printStackTrace()
            }
            shutdownWatcherExecutor()
        }
    }

    override fun terminateAndConfirmExit(
        gracefulTimeoutMs: Long,
        forceTimeoutMs: Long,
    ): ProotTerminationResult {
        if (!process.isAlive) {
            val code = try { process.exitValue() } catch (originalError115: IllegalThreadStateException) {
              originalError115.printStackTrace()
             null }
            return ProotTerminationResult.ConfirmedExited(code)
        }

        val processTreeSnapshot = snapshotProcessTree()
        try { process.destroy() } catch (originalError119: Throwable) {
          originalError119.printStackTrace()
        }
        val gracefulExit = try {
            exitDeferred.get(gracefulTimeoutMs, java.util.concurrent.TimeUnit.MILLISECONDS)?.exitCode
        } catch (originalError122: Exception) {
          originalError122.printStackTrace()

            null
        }
        if (gracefulExit != null) {
            terminateSnapshotProcesses(processTreeSnapshot)
            return ProotTerminationResult.ConfirmedExited(gracefulExit)
        }

        if (!process.isAlive) {
            terminateSnapshotProcesses(processTreeSnapshot)
            val code = try { process.exitValue() } catch (originalError130: IllegalThreadStateException) {
              originalError130.printStackTrace()
             null }
            return ProotTerminationResult.ConfirmedExited(code)
        }

        try { forceTerminateProcessTree(processTreeSnapshot) } catch (originalError134: Throwable) {
          originalError134.printStackTrace()
        }
        val forcedExit = try {
            exitDeferred.get(forceTimeoutMs, java.util.concurrent.TimeUnit.MILLISECONDS)?.exitCode
        } catch (originalError137: Exception) {
          originalError137.printStackTrace()

            null
        }
        if (forcedExit != null) {
            terminateSnapshotProcesses(processTreeSnapshot)
            return ProotTerminationResult.ConfirmedExited(forcedExit)
        }

        if (!process.isAlive) {
            terminateSnapshotProcesses(processTreeSnapshot)
            val code = try { process.exitValue() } catch (originalError145: IllegalThreadStateException) {
              originalError145.printStackTrace()
             null }
            return ProotTerminationResult.ConfirmedExited(code)
        }

        return ProotTerminationResult.StillAlive
    }

    override fun activate() {
        if (!activated.compareAndSet(false, true)) return
        stdoutPump.start()
        stderrPump.start()
        if (isProcessDead()) {
            awaitOutputDrain()
            val exitCode = process.exitValue()
            val exit = ProotExit(
                generation = generation,
                sessionId = sessionId,
                exitCode = exitCode,
                stopRequested = stopRequested.get(),
            )
            publishExit(exit)
        } else {
            startExitWatcher()
            if (started.compareAndSet(false, true) && !terminalPublished.get()) {
                safeNotify(ProotEvent.Started(sessionId, System.currentTimeMillis()))
            }
        }
    }

    private fun shutdownWatcherExecutor() {
        try {
            watcherExecutor.shutdown()
        } catch (originalError179: Throwable) {
          originalError179.printStackTrace()
        }
    }

    private fun forceTerminateProcessTree(
        snapshot: Collection<ProcessIdentity> = emptyList(),
    ) {
        val rootPid = processPid()
        val targetPids = linkedSetOf<Int>()
        if (rootPid != null) {
            val descendants = descendantPids(rootPid, mutableSetOf())
            for (pid in descendants) {
                targetPids.add(pid)
                if (!isSameProcess(pid)) continue
                try {
                    android.os.Process.sendSignal(pid, android.os.Process.SIGNAL_KILL)
                } catch (error: Throwable) {
                    System.err.println("sendSignal failed for pid=$pid: ${error.message}")
                    error.printStackTrace()
                }
            }
            targetPids.add(rootPid)
            if (isSameProcess(rootPid)) {
                try {
                    android.os.Process.sendSignal(rootPid, android.os.Process.SIGNAL_KILL)
                } catch (error: Throwable) {
                    System.err.println("sendSignal failed for root pid=$rootPid: ${error.message}")
                    error.printStackTrace()
                }
            }
        }
        val snapshotPids = snapshot
            .filter(::isSameProcess)
            .map(ProcessIdentity::pid)
            .filterNot(targetPids::contains)
        if (snapshotPids.isNotEmpty()) {
            killWithSystemTool(snapshotPids)
        }
        if (targetPids.isNotEmpty()) killWithSystemTool(targetPids)
        process.destroyForcibly()
        val retryPids = (targetPids + snapshotPids).filter { isSameProcess(it) && isProcessAlive(it) }
        if (retryPids.isNotEmpty()) {
            killWithSystemTool(retryPids)
        }
    }

    private data class ProcessIdentity(
        val pid: Int,
        val startTime: String,
    )

    private fun snapshotProcessTree(): List<ProcessIdentity> {
        val rootPid = processPid() ?: return emptyList()
        val pids = descendantPids(rootPid, mutableSetOf()) + rootPid
        return pids.mapNotNull(::processIdentity)
    }

    private fun processIdentity(pid: Int): ProcessIdentity? {
        val stat = try {
            File("/proc/$pid/stat").readText().trim()
        } catch (originalError: Throwable) {
            return processIdentityFromPs(pid)
        }
        val fields = stat.substringAfterLast(") ", "")
            .split(Regex("\\s+"))
            .filter(String::isNotEmpty)
        val startTime = fields.getOrNull(19) ?: return null
        return ProcessIdentity(pid, startTime)
    }

    private fun processIdentityFromPs(pid: Int): ProcessIdentity? {
        return try {
            val process = ProcessBuilder("ps", "-p", pid.toString(), "-o", "lstart=")
                .redirectErrorStream(true)
                .start()
            val output = process.inputStream.bufferedReader().use { it.readText().trim() }
            if (process.waitFor() != 0 || output.isEmpty()) return null
            ProcessIdentity(pid, output)
        } catch (originalError: Throwable) {
            null
        }
    }

    private fun isSameProcess(identity: ProcessIdentity): Boolean {
        return processIdentity(identity.pid)?.startTime == identity.startTime
    }

    private fun isSameProcess(pid: Int): Boolean {
        return processIdentity(pid) != null && isProcessAlive(pid)
    }

    private fun terminateSnapshotProcesses(snapshot: Collection<ProcessIdentity>) {
        val pids = snapshot
            .filter(::isSameProcess)
            .map(ProcessIdentity::pid)
            .filter(::isProcessAlive)
        if (pids.isNotEmpty()) {
            killWithSystemTool(pids)
        }
    }

    private fun killWithSystemTool(pids: Collection<Int>) {
        val kill = File("/system/bin/kill")
        if (pids.isEmpty()) return
        val command = if (kill.isFile) {
            listOf(kill.absolutePath, "-9") + pids.map(Int::toString)
        } else {
            listOf("kill", "-9") + pids.map(Int::toString)
        }
        val process = try {
            ProcessBuilder(command)
                .redirectErrorStream(true)
                .start()
        } catch (error: Throwable) {
            System.err.println("system kill failed for pids=$pids: ${error.message}")
            error.printStackTrace()
            return
        }
        try {
            if (!process.waitFor(1, java.util.concurrent.TimeUnit.SECONDS)) {
                process.destroyForcibly()
            }
        } catch (error: Throwable) {
            process.destroyForcibly()
            System.err.println("system kill wait failed for pids=$pids: ${error.message}")
            error.printStackTrace()
        }
    }

    private fun isProcessAlive(pid: Int): Boolean {
        val stat = File("/proc/$pid/stat")
        val raw = try {
            stat.readText()
        } catch (error: Throwable) {
            return try {
                val process = ProcessBuilder("kill", "-0", pid.toString())
                    .redirectErrorStream(true)
                    .start()
                process.inputStream.close()
                process.waitFor() == 0
            } catch (originalError: Throwable) {
                false
            }
        }
        val state = raw.substringAfterLast(") ", "").firstOrNull()
        return state != null && state != 'Z' && state != 'X'
    }

    private fun processPid(): Int? {
        val method = runCatching {
            process.javaClass.getDeclaredMethod("pid").apply { isAccessible = true }
        }.getOrNull()
        if (method != null) {
            try {
                return (method.invoke(process) as? Number)?.toInt()
            } catch (originalError166: Throwable) {
                originalError166.printStackTrace()
            }
        }
        var type: Class<*>? = process.javaClass
        while (type != null) {
            val field = runCatching { type.getDeclaredField("pid").apply { isAccessible = true } }.getOrNull()
            if (field != null) {
                try {
                    return (field.get(process) as? Number)?.toInt()
                } catch (originalError177: Throwable) {
                    originalError177.printStackTrace()
                    return null
                }
            }
            type = type.superclass
        }
        return null
    }

    private fun descendantPids(pid: Int, seen: MutableSet<Int>): List<Int> {
        if (!seen.add(pid)) return emptyList()
        val children = try {
            File("/proc/$pid/task/$pid/children")
                .readText()
                .trim()
                .split(Regex("\\s+"))
                .mapNotNull(String::toIntOrNull)
        } catch (originalError194: Throwable) {
            if (!File("/proc").isDirectory) {
                return descendantPidsFromPs(pid, seen)
            }
            originalError194.printStackTrace()
            emptyList()
        }
        val result = mutableListOf<Int>()
        for (child in children) {
            result.addAll(descendantPids(child, seen))
            result.add(child)
        }
        return result
    }

    private fun descendantPidsFromPs(pid: Int, seen: MutableSet<Int>): List<Int> {
        val childrenByParent = try {
            val process = ProcessBuilder("ps", "-axo", "pid=,ppid=")
                .redirectErrorStream(true)
                .start()
            val lines = process.inputStream.bufferedReader().use { it.readLines() }
            if (process.waitFor() != 0) return emptyList()
            lines.mapNotNull { line ->
                val parts = line.trim().split(Regex("\\s+"))
                if (parts.size < 2) return@mapNotNull null
                val childPid = parts[0].toIntOrNull() ?: return@mapNotNull null
                val parentPid = parts[1].toIntOrNull() ?: return@mapNotNull null
                parentPid to childPid
            }.groupBy({ it.first }, { it.second })
        } catch (originalError: Throwable) {
            originalError.printStackTrace()
            emptyMap()
        }
        val result = mutableListOf<Int>()
        for (child in childrenByParent[pid].orEmpty()) {
            if (!seen.add(child)) continue
            result.addAll(descendantPidsFromPs(child, seen))
            result.add(child)
        }
        return result
    }

    private fun waitForRealExit(): Int {
        var retries = 0
        var interrupted = false

        while (true) {
            try {
                val exitCode = process.waitFor()

                if (interrupted) {
                    Thread.currentThread().interrupt()
                }

                return exitCode
            } catch (originalError195: InterruptedException) {
              originalError195.printStackTrace()

                interrupted = true

                if (!process.isAlive) {
                    val exitCode = process.exitValue()

                    Thread.currentThread().interrupt()
                    return exitCode
                }

                retries++
                if (retries >= MAX_WATCHER_RETRY) {
                    throw InterruptedException("watcher retry exhausted after $retries attempts: ${originalError195.message}")
                }
            }
        }
    }

    private fun isProcessDead(): Boolean {
        return try {
            process.exitValue()
            true
        } catch (originalError217: IllegalThreadStateException) {
            false
        }
    }

    private fun startExitWatcher() {
        if (!watcherStarted.compareAndSet(false, true)) return
        watcherExecutor.execute {
            try {
                val code = waitForRealExit()
                awaitOutputDrain()
                val exit = ProotExit(
                    generation = generation,
                    sessionId = sessionId,
                    exitCode = code,
                    stopRequested = stopRequested.get(),
                )
                publishExit(exit)
            } catch (e: InterruptedException) {
                if (isProcessDead()) {
                    awaitOutputDrain()
                    val exitCode = process.exitValue()
                    val exit = ProotExit(
                        generation = generation,
                        sessionId = sessionId,
                        exitCode = exitCode,
                        stopRequested = stopRequested.get(),
                    )
                    publishExit(exit)
                } else {
                    watcherFailureRef.set(e.message ?: "interrupted")
                    publishWatcherFailure(e.message ?: "interrupted")
                }
            } catch (e: Exception) {
                if (isProcessDead()) {
                    awaitOutputDrain()
                    val exitCode = process.exitValue()
                    val exit = ProotExit(
                        generation = generation,
                        sessionId = sessionId,
                        exitCode = exitCode,
                        stopRequested = stopRequested.get(),
                    )
                    publishExit(exit)
                } else {
                    watcherFailureRef.set(e.message ?: "unknown watcher error")
                    publishWatcherFailure(e.message ?: "unknown watcher error")
                }
            } finally {
                watcherExecutor.shutdown()
            }
        }
    }

    private fun publishExit(exit: ProotExit) {
        exitRef.set(exit)
        exitDeferred.complete(exit)
        publishTerminalOnce(exit)
    }

    private fun awaitOutputDrain() {
        stdoutPump.awaitStopped(IMMEDIATE_EXIT_OUTPUT_DRAIN_MS)
        stderrPump.awaitStopped(IMMEDIATE_EXIT_OUTPUT_DRAIN_MS)
    }

    private fun publishWatcherFailure(message: String) {
        if (watcherFailurePublished.compareAndSet(false, true)) {
            safeNotify(
                ProotEvent.ExitWatcherFailed(
                    sessionId = sessionId,
                    generation = generation,
                    message = message,
                )
            )
        }
    }

    private fun publishTerminalOnce(exit: ProotExit) {
        if (terminalPublished.compareAndSet(false, true)) {
            safeNotify(ProotEvent.Exited(exit))
        }
    }

    private fun safeNotify(event: ProotEvent) {
        try { observer.onEvent(event) } catch (originalError301: Throwable) {
          originalError301.printStackTrace()
        }
    }

    private companion object {
        const val MAX_WATCHER_RETRY = 3
        const val IMMEDIATE_EXIT_OUTPUT_DRAIN_MS = 75L
    }
}

internal interface StreamPump {
    fun start()
    fun stop()
    fun awaitStopped(timeoutMillis: Long)
}

internal class InputStreamPump(
    private val inputStream: java.io.InputStream,
    private val onData: (String, Long) -> Unit
) : StreamPump {
    private val sequence = java.util.concurrent.atomic.AtomicLong(0)
    private var thread: Thread? = null
    private val running = AtomicBoolean(false)

    override fun start() {
        if (running.compareAndSet(false, true)) {
            thread = Thread {
                val reader = inputStream.bufferedReader()
                try {
                    while (running.get()) {
                        val line = reader.readLine() ?: break
                        onData(line, sequence.incrementAndGet())
                    }
                } catch (originalError333: Throwable) {
                  originalError333.printStackTrace()
                }
                finally { running.set(false) }
            }.apply {
                isDaemon = true
                name = "proot-stream-pump"
                start()
            }
        }
    }

    override fun stop() {
        running.set(false)
        thread?.interrupt()
    }

    override fun awaitStopped(timeoutMillis: Long) {
        if (timeoutMillis <= 0L) return
        val pumpThread = thread ?: return
        if (pumpThread === Thread.currentThread()) return
        try {
            pumpThread.join(timeoutMillis)
        } catch (originalError354: InterruptedException) {
          originalError354.printStackTrace()

            Thread.currentThread().interrupt()
        }
    }
}
