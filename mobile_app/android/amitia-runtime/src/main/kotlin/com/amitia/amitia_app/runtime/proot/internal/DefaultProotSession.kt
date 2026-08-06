package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import java.util.concurrent.atomic.AtomicBoolean

internal class DefaultProotSession(override val sessionId: String, private val process: Process, private val observer: ProotObserver) : ProotSession {
    private val closed = AtomicBoolean(false)
    private var cachedExitCode: Int? = null
    private val stdoutPump: StreamPump
    private val stderrPump: StreamPump
    private val started = AtomicBoolean(false)

    init {
        stdoutPump = InputStreamPump(process.inputStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stdout(sessionId, data, seq)) }
        stderrPump = InputStreamPump(process.errorStream) { data, seq -> if (!closed.get()) safeNotify(ProotEvent.Stderr(sessionId, data, seq)) }
    }

    override fun isAlive(): Boolean {
        if (closed.get()) return false
        return try { process.exitValue(); cachedExitCode = process.exitValue(); false } catch (e: IllegalThreadStateException) { true }
    }

    override fun awaitExit(timeoutMillis: Long): Int? {
        if (closed.get()) return cachedExitCode
        val deadline = System.currentTimeMillis() + timeoutMillis
        while (System.currentTimeMillis() < deadline) {
            if (!isAlive()) {
                val code = try { process.exitValue() } catch (e: Exception) { return cachedExitCode }
                cachedExitCode = code; safeNotify(ProotEvent.Exited(sessionId, code, false)); close(); return code
            }
            Thread.sleep(50)
        }
        return null
    }

    override fun stop(graceMillis: Long): ProotStopResult {
        if (closed.get()) return ProotStopResult.AlreadyStopped(sessionId, cachedExitCode)
        if (!isAlive()) { val code = cachedExitCode; close(); return ProotStopResult.AlreadyStopped(sessionId, code) }
        return try {
            process.destroy()
            if (process.waitFor(graceMillis, java.util.concurrent.TimeUnit.MILLISECONDS)) {
                val code = process.exitValue(); cachedExitCode = code; safeNotify(ProotEvent.Exited(sessionId, code, false)); close()
                ProotStopResult.Graceful(sessionId, code)
            } else {
                process.destroyForcibly()
                val exited = process.waitFor(5, java.util.concurrent.TimeUnit.SECONDS)
                val code = if (exited) process.exitValue() else null; cachedExitCode = code; safeNotify(ProotEvent.Exited(sessionId, code ?: -1, true)); close()
                ProotStopResult.Forced(sessionId, code)
            }
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt(); process.destroyForcibly(); close()
            ProotStopResult.Failed(sessionId, com.amitia.amitia_app.runtime.proot.ProotErrorCode.PROCESS_STOP_FAILED, e.message ?: "interrupted")
        } catch (e: Exception) {
            close()
            ProotStopResult.Failed(sessionId, com.amitia.amitia_app.runtime.proot.ProotErrorCode.PROCESS_STOP_FAILED, e.message ?: "error")
        }
    }

    override fun close() {
        if (closed.compareAndSet(false, true)) {
            try { stdoutPump.stop() } catch (_: Throwable) {}
            try { stderrPump.stop() } catch (_: Throwable) {}
            try { if (process.isAlive) process.destroyForcibly() } catch (_: Throwable) {}
        }
    }

    internal fun markStarted() {
        if (started.compareAndSet(false, true)) {
            safeNotify(ProotEvent.Started(sessionId, System.currentTimeMillis())); stdoutPump.start(); stderrPump.start()
        }
    }

    private fun safeNotify(event: ProotEvent) { try { observer.onEvent(event) } catch (_: Throwable) {} }
}

internal interface StreamPump { fun start(); fun stop() }
internal class InputStreamPump(private val inputStream: java.io.InputStream, private val onData: (String, Long) -> Unit) : StreamPump {
    private val sequence = java.util.concurrent.atomic.AtomicLong(0)
    private var thread: Thread? = null
    private val running = AtomicBoolean(false)
    override fun start() {
        if (running.compareAndSet(false, true)) {
            thread = Thread {
                val reader = inputStream.bufferedReader()
                try { while (running.get()) { val line = reader.readLine() ?: break; onData(line, sequence.incrementAndGet()) } } catch (_: Throwable) {} finally { running.set(false) }
            }.apply { isDaemon = true; name = "proot-stream-pump"; start() }
        }
    }
    override fun stop() { running.set(false); thread?.interrupt() }
}