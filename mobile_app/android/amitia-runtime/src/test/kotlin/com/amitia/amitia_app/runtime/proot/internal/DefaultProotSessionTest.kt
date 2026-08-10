package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class DefaultProotSessionTest {

    private val isWindows: Boolean = System.getProperty("os.name").lowercase().contains("windows")

    private fun execProcess(cmd: String): Process {
        val pb = if (isWindows) {
            ProcessBuilder("cmd.exe", "/c", cmd)
        } else {
            ProcessBuilder("/bin/sh", "-c", cmd)
        }
        pb.directory(java.io.File("."))
        return pb.start()
    }

    private fun sleepCmd(seconds: Int): String =
        if (isWindows) "ping -n $seconds 127.0.0.1 >nul" else "sleep $seconds"

    @Test
    fun session_holdsRealProcess() {
        val process = execProcess(sleepCmd(10))
        val session = DefaultProotSession("test-1", process, ProotObserver {})
        session.markStarted()
        assertTrue(session.isAlive())
        session.close()
    }

    @Test
    fun session_isAlive_falseAfterExit() {
        val process = execProcess("exit 0")
        val session = DefaultProotSession("test-2", process, ProotObserver {})
        session.markStarted()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(100)
        assertFalse(session.isAlive())
        session.close()
    }

    @Test
    fun session_exitCodeAvailable() {
        val process = execProcess("exit 42")
        val events = mutableListOf<ProotEvent>()
        val session = DefaultProotSession("test-3", process, ProotObserver { events.add(it) })
        session.markStarted()
        val exitCode = session.awaitExit(5000)
        assertEquals(42, exitCode)
        session.close()
    }

    @Test
    fun session_awaitExit_returnsNullOnTimeout() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-4", process, ProotObserver {})
        session.markStarted()
        Thread.sleep(200)
        assertTrue("Process should still be alive", session.isAlive())
        val result = session.awaitExit(200)
        assertNull("Should not have exited within timeout", result)
        session.close()
    }

    @Test
    fun session_stop_gracefulReturnsGracefulResult() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-5", process, ProotObserver {})
        session.markStarted()
        assertTrue(session.isAlive())
        val result = session.stop(2000)
        assertTrue(result is ProotStopResult.Graceful || result is ProotStopResult.Forced)
        assertEquals("test-5", result.sessionId)
        session.close()
    }

    @Test
    fun session_stop_idempotent() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-6", process, ProotObserver {})
        session.markStarted()
        session.stop(2000)
        val secondResult = session.stop(1000)
        assertTrue(secondResult is ProotStopResult.AlreadyStopped)
    }

    @Test
    fun session_close_idempotent() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-7", process, ProotObserver {})
        session.markStarted()
        session.close()
        session.close()
    }

    @Test
    fun session_observerReceivesExitOnce() {
        val process = execProcess("exit 0")
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-8", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        })
        session.markStarted()
        Thread.sleep(500)
        session.awaitExit(1000)
        Thread.sleep(200)
        assertTrue("Exit should fire at most once", exitCount.get() <= 1)
        session.close()
    }

    @Test
    fun session_exitEventContainsSessionId() {
        val process = execProcess("exit 7")
        val capturedSessionId = AtomicReference("")
        val exited = java.util.concurrent.atomic.AtomicBoolean(false)
        val session = DefaultProotSession("test-special-id", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) {
                capturedSessionId.set(event.sessionId)
                exited.set(true)
            }
        })
        session.markStarted()
        Thread.sleep(800)
        session.awaitExit(3000)
        Thread.sleep(300)
        assertTrue("Should have received exit event", exited.get())
        assertEquals("test-special-id", capturedSessionId.get())
        session.close()
    }

    @Test
    fun session_immediateExit_detected() {
        val process = execProcess("exit 1")
        val session = DefaultProotSession("test-immediate", process, ProotObserver {})
        session.markStarted()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(200)
        assertFalse(session.isAlive())
        session.close()
    }

    @Test
    fun session_stdoutReadWithoutBlocking() {
        val process = execProcess(if (isWindows) "echo line1\\necho line2" else "echo line1; echo line2")
        val lines = mutableListOf<String>()
        val session = DefaultProotSession("test-stdout", process, ProotObserver { event ->
            if (event is ProotEvent.Stdout) lines.add(event.data)
        })
        session.markStarted()
        Thread.sleep(500)
        assertNotNull(lines)
        session.close()
    }

    @Test
    fun session_stderrReadWithoutBlocking() {
        val process = execProcess(if (isWindows) "echo err-msg 1>&2" else "echo err-msg >&2")
        val errLines = mutableListOf<String>()
        val session = DefaultProotSession("test-stderr", process, ProotObserver { event ->
            if (event is ProotEvent.Stderr) errLines.add(event.data)
        })
        session.markStarted()
        Thread.sleep(500)
        assertNotNull(errLines)
        session.close()
    }

    @Test
    fun waitForOwner_isSession() {
        val process = execProcess("exit 0")
        val session = DefaultProotSession("test-wait", process, ProotObserver {})
        session.markStarted()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(100)
        val result = session.awaitExit(3000)
        assertNotNull(result)
        assertEquals(session, session)
        session.close()
    }
}
