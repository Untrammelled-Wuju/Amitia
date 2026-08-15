package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotExit
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
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
    private val watcherStarted = AtomicBoolean(false)
    private val watcherFailureRef = AtomicReference<String?>(null)

    init {
        stdoutPump = InputStreamPump(process.inputStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stdout(sessionId, data, seq)) }
        stderrPump = InputStreamPump(process.errorStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stderr(sessionId, data, seq)) }
        startExitWatcher()
    }

    override fun isAlive(): Boolean {
        if (closed.get()) return false
        return try {
            process.exitValue()
            false
        } catch (e: IllegalThreadStateException) { true }
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
        } catch (e: Exception) {
            exitRef.get()?.exitCode
        }
    }

    override fun stop(graceMillis: Long): ProotStopResult {
        if (closed.get()) {
            val code = exitRef.get()?.exitCode
            return ProotStopResult.AlreadyStopped(sessionId, code)
        }
        stopRequested.set(true)
        return try {
            process.destroy()
            val graceful = try {
                exitDeferred.get(graceMillis, java.util.concurrent.TimeUnit.MILLISECONDS)
            } catch (_: Exception) {
                null
            }
            if (graceful != null) {
                val code = exitRef.get()?.exitCode
                ProotStopResult.Graceful(sessionId, code)
            } else {
                process.destroyForcibly()
                try { exitDeferred.get(5, java.util.concurrent.TimeUnit.SECONDS) } catch (_: Exception) {}
                val code = exitRef.get()?.exitCode
                ProotStopResult.Forced(sessionId, code)
            }
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            process.destroyForcibly()
            try { exitDeferred.get(5, java.util.concurrent.TimeUnit.SECONDS) } catch (_: Exception) {}
            val code = exitRef.get()?.exitCode
            ProotStopResult.Forced(sessionId, code)
        } catch (e: Exception) {
            process.destroyForcibly()
            ProotStopResult.Failed(sessionId, com.amitia.amitia_app.runtime.proot.ProotErrorCode.PROCESS_STOP_FAILED, e.message ?: "error")
        }
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            try { stdoutPump.stop() } catch (_: Throwable) {}
            try { stderrPump.stop() } catch (_: Throwable) {}
            try { if (process.isAlive) process.destroyForcibly() } catch (_: Throwable) {}
            shutdownWatcherExecutor()
        }
    }

    internal fun markStarted() {
        if (started.compareAndSet(false, true)) {
            safeNotify(ProotEvent.Started(sessionId, System.currentTimeMillis()))
            stdoutPump.start()
            stderrPump.start()
        }
    }

    private fun shutdownWatcherExecutor() {
        try {
            watcherExecutor.shutdown()
        } catch (_: Throwable) {}
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
            } catch (_: InterruptedException) {
                interrupted = true

                if (!process.isAlive) {
                    val exitCode = process.exitValue()

                    Thread.currentThread().interrupt()
                    return exitCode
                }

                retries++
                if (retries >= MAX_WATCHER_RETRY) {
                    throw InterruptedException("watcher retry exhausted after $retries attempts")
                }
            }
        }
    }

    private fun isProcessDead(): Boolean {
        return try {
            process.exitValue()
            true
        } catch (_: IllegalThreadStateException) {
            false
        }
    }

    private fun startExitWatcher() {
        if (!watcherStarted.compareAndSet(false, true)) return
        watcherExecutor.execute {
            try {
                val code = waitForRealExit()
                val exit = ProotExit(
                    generation = generation,
                    sessionId = sessionId,
                    exitCode = code,
                    stopRequested = stopRequested.get(),
                )
                exitRef.set(exit)
                publishTerminalOnce(exit)
                exitDeferred.complete(exit)
            } catch (e: InterruptedException) {
                if (isProcessDead()) {
                    val exitCode = process.exitValue()
                    val exit = ProotExit(
                        generation = generation,
                        sessionId = sessionId,
                        exitCode = exitCode,
                        stopRequested = stopRequested.get(),
                    )
                    exitRef.set(exit)
                    publishTerminalOnce(exit)
                    exitDeferred.complete(exit)
                } else {
                    watcherFailureRef.set(e.message ?: "interrupted")
                    publishWatcherFailure(e.message ?: "interrupted")
                }
            } catch (e: Exception) {
                if (isProcessDead()) {
                    val exitCode = process.exitValue()
                    val exit = ProotExit(
                        generation = generation,
                        sessionId = sessionId,
                        exitCode = exitCode,
                        stopRequested = stopRequested.get(),
                    )
                    exitRef.set(exit)
                    publishTerminalOnce(exit)
                    exitDeferred.complete(exit)
                } else {
                    watcherFailureRef.set(e.message ?: "unknown watcher error")
                    publishWatcherFailure(e.message ?: "unknown watcher error")
                }
            } finally {
                watcherExecutor.shutdown()
            }
        }
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
        try { observer.onEvent(event) } catch (_: Throwable) {}
    }

    private companion object {
        const val MAX_WATCHER_RETRY = 3
    }
}

internal interface StreamPump {
    fun start()
    fun stop()
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
                } catch (_: Throwable) {}
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
}
