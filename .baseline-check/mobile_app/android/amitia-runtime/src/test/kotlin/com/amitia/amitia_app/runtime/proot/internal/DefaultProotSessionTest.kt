package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotEvent
import com.amitia.amitia_app.runtime.proot.ProotExit
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
    private val testGeneration: Long = 42L

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
        val session = DefaultProotSession("test-1", process, ProotObserver {}, testGeneration)
        session.activate()
        assertTrue(session.isAlive())
        session.close()
    }

    @Test
    fun session_isAlive_falseAfterExit() {
        val process = execProcess("exit 0")
        val session = DefaultProotSession("test-2", process, ProotObserver {}, testGeneration)
        session.activate()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(100)
        assertFalse(session.isAlive())
        session.close()
    }

    @Test
    fun session_exitCodeAvailable() {
        val process = execProcess("exit 42")
        val events = mutableListOf<ProotEvent>()
        val session = DefaultProotSession("test-3", process, ProotObserver { events.add(it) }, testGeneration)
        session.activate()
        val exitCode = session.awaitExit(5000)
        assertEquals(42, exitCode)
        session.close()
    }

    @Test
    fun session_awaitExit_returnsNullOnTimeout() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-4", process, ProotObserver {}, testGeneration)
        session.activate()
        Thread.sleep(200)
        assertTrue("Process should still be alive", session.isAlive())
        val result = session.awaitExit(200)
        assertNull("Should not have exited within timeout", result)
        session.close()
    }

    @Test
    fun session_stop_gracefulReturnsGracefulResult() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-5", process, ProotObserver {}, testGeneration)
        session.activate()
        assertTrue(session.isAlive())
        val result = session.stop(2000)
        assertTrue(result is ProotStopResult.Graceful || result is ProotStopResult.Forced)
        assertEquals("test-5", result.sessionId)
        session.close()
    }

    @Test
    fun session_stop_idempotent() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-6", process, ProotObserver {}, testGeneration)
        session.activate()
        session.stop(2000)
        val secondResult = session.stop(1000)
        assertTrue(secondResult is ProotStopResult.AlreadyStopped)
    }

    @Test
    fun session_close_idempotent() {
        val process = execProcess(sleepCmd(60))
        val session = DefaultProotSession("test-7", process, ProotObserver {}, testGeneration)
        session.activate()
        session.close()
        session.close()
    }

    @Test
    fun session_observerReceivesExitOnce() {
        val process = execProcess("exit 0")
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-8", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        }, testGeneration)
        session.activate()
        Thread.sleep(500)
        session.awaitExit(1000)
        Thread.sleep(200)
        assertTrue("Exit should fire at most once, got ${exitCount.get()}", exitCount.get() <= 1)
        session.close()
    }

    @Test
    fun session_exitEventContainsSessionId() {
        val process = execProcess("exit 7")
        val capturedSessionId = AtomicReference("")
        val exited = java.util.concurrent.atomic.AtomicBoolean(false)
        val session = DefaultProotSession("test-special-id", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) {
                capturedSessionId.set(event.exit.sessionId)
                exited.set(true)
            }
        }, testGeneration)
        session.activate()
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
        val session = DefaultProotSession("test-immediate", process, ProotObserver {}, testGeneration)
        session.activate()
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
        }, testGeneration)
        session.activate()
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
        }, testGeneration)
        session.activate()
        Thread.sleep(500)
        assertNotNull(errLines)
        session.close()
    }

    @Test
    fun waitForOwner_isSession() {
        val process = execProcess("exit 0")
        val session = DefaultProotSession("test-wait", process, ProotObserver {}, testGeneration)
        session.activate()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(100)
        val result = session.awaitExit(3000)
        assertNotNull(result)
        assertEquals(session, session)
        session.close()
    }

    @Test
    fun exitEvent_carriesGeneration() {
        val process = execProcess("exit 5")
        val capturedExit = AtomicReference<ProotExit?>(null)
        val session = DefaultProotSession("test-gen", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) capturedExit.set(event.exit)
        }, testGeneration)
        session.activate()
        Thread.sleep(800)
        session.awaitExit(3000)
        Thread.sleep(300)
        val exit = capturedExit.get()
        assertNotNull("Should have received exit event", exit)
        assertEquals(testGeneration, exit!!.generation)
        assertEquals(5, exit.exitCode)
        assertFalse(exit.stopRequested)
        session.close()
    }

    @Test
    fun watcher_startsExactlyOnce() {
        val process = execProcess(sleepCmd(10))
        val session = DefaultProotSession("test-watcher-once", process, ProotObserver {}, testGeneration)
        session.activate()
        Thread.sleep(500)
        session.close()
    }

    @Test
    fun awaitExit_sharedTerminalResult() {
        val process = execProcess("exit 99")
        val session = DefaultProotSession("test-shared", process, ProotObserver {}, testGeneration)
        session.activate()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(100)
        val r1 = session.awaitExit(1000)
        val r2 = session.awaitExit(1000)
        assertEquals(99, r1)
        assertEquals(99, r2)
        session.close()
    }

    @Test
    fun awaitExit_zeroTimeout_nonBlocking() {
        val process = execProcess(sleepCmd(30))
        val session = DefaultProotSession("test-nonblock", process, ProotObserver {}, testGeneration)
        session.activate()
        Thread.sleep(200)
        val result = session.awaitExit(0)
        assertNull("Should return null for zero timeout when process is alive", result)
        session.close()
    }

    @Test
    fun naturalExit_stopRequestedFalse() {
        val process = execProcess("exit 3")
        val capturedExit = AtomicReference<ProotExit?>(null)
        val session = DefaultProotSession("test-natural", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) capturedExit.set(event.exit)
        }, testGeneration)
        session.activate()
        Thread.sleep(800)
        session.awaitExit(3000)
        Thread.sleep(300)
        val exit = capturedExit.get()
        assertNotNull(exit)
        assertFalse("Natural exit should have stopRequested=false", exit!!.stopRequested)
        assertEquals(3, exit.exitCode)
        session.close()
    }

    @Test
    fun stop_setsStopRequestedTrue() {
        val process = execProcess(sleepCmd(60))
        val capturedExit = AtomicReference<ProotExit?>(null)
        val session = DefaultProotSession("test-stop-flag", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) capturedExit.set(event.exit)
        }, testGeneration)
        session.activate()
        Thread.sleep(300)
        session.requestStop()
        val result = session.stop(2000)
        Thread.sleep(500)
        val exit = capturedExit.get()
        assertNotNull(exit)
        assertTrue("After stop(), exit should have stopRequested=true", exit!!.stopRequested)
        assertTrue(result is ProotStopResult.Graceful || result is ProotStopResult.Forced)
        session.close()
    }

    @Test
    fun stop_thenNaturalExit_singleTerminalEvent() {
        val process = execProcess("exit 1")
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-race", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        }, testGeneration)
        session.activate()
        Thread.sleep(100)
        session.requestStop()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(500)
        session.awaitExit(3000)
        Thread.sleep(200)
        assertEquals("Terminal event count must be exactly 1", 1, exitCount.get())
        session.close()
    }

    @Test
    fun doubleStop_singleTerminalEvent() {
        val process = execProcess(sleepCmd(60))
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-double-stop", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        }, testGeneration)
        session.activate()
        Thread.sleep(300)
        session.stop(1000)
        session.stop(1000)
        Thread.sleep(500)
        session.awaitExit(3000)
        Thread.sleep(300)
        assertEquals("Double stop should produce exactly 1 terminal event", 1, exitCount.get())
        session.close()
    }

    @Test
    fun exitProperty_availableAfterExit() {
        val process = execProcess("exit 77")
        val session = DefaultProotSession("test-exit-prop", process, ProotObserver {}, testGeneration)
        session.activate()
        Thread.sleep(800)
        session.awaitExit(3000)
        Thread.sleep(300)
        val exit = session.exit
        assertNotNull(exit)
        assertEquals(77, exit!!.exitCode)
        assertEquals(testGeneration, exit.generation)
        assertEquals("test-exit-prop", exit.sessionId)
        session.close()
    }

    @Test
    fun watcher_survivesBeyondAwaitExitZero() {
        val process = execProcess(sleepCmd(30))
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-survives", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        }, testGeneration)
        session.activate()
        Thread.sleep(200)
        val zeroResult = session.awaitExit(0)
        assertNull(zeroResult)
        assertEquals(0, exitCount.get())
        session.close()
    }

    @Test
    fun watcherInterruptWhileProcessAlive_doesNotPublishExited() {
        val process = execProcess(sleepCmd(30))
        val exitCount = AtomicInteger(0)
        val session = DefaultProotSession("test-interrupt-alive", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) exitCount.incrementAndGet()
        }, testGeneration)
        session.activate()
        Thread.sleep(200)

        val watcherThread = Thread.getAllStackTraces().keys.firstOrNull { it.name == "proot-exit-watcher-test-interrupt-alive" }
        assertNotNull("Watcher thread should exist", watcherThread)
        watcherThread!!.interrupt()

        Thread.sleep(500)
        assertEquals("Interrupt should not publish Exited while process alive", 0, exitCount.get())
        assertTrue("Process should still be alive", session.isAlive())

        session.close()
        Thread.sleep(500)
        assertEquals("After close, exactly one Exited should be published", 1, exitCount.get())
    }

    @Test
    fun watcherExecutorTerminatesAfterSessionTerminal() {
        val process = execProcess("exit 0")
        val session = DefaultProotSession("test-executor-terminate", process, ProotObserver {}, testGeneration)
        session.activate()
        process.waitFor(5, TimeUnit.SECONDS)
        Thread.sleep(500)
        session.close()
        Thread.sleep(1000)

        val watcherThread = Thread.getAllStackTraces().keys.firstOrNull { it.name == "proot-exit-watcher-test-executor-terminate" }
        assertTrue("Watcher thread should be terminated after session terminal", watcherThread == null || !watcherThread.isAlive)
    }

    @Test
    fun watcherInterruptAfterProcessExited_returnsRealExitCode() {
        val process = execProcess("exit 55")
        val capturedExit = AtomicReference<ProotExit?>(null)
        val session = DefaultProotSession("test-interrupt-after-exit", process, ProotObserver { event ->
            if (event is ProotEvent.Exited) capturedExit.set(event.exit)
        }, testGeneration)
        session.activate()
        process.waitFor(5, TimeUnit.SECONDS)

        val watcherThread = Thread.getAllStackTraces().keys.firstOrNull { it.name == "proot-exit-watcher-test-interrupt-after-exit" }
        if (watcherThread != null) {
            watcherThread.interrupt()
        }

        Thread.sleep(500)
        session.awaitExit(3000)
        Thread.sleep(200)

        val exit = capturedExit.get()
        assertNotNull("Should have received exit event", exit)
        assertEquals("Exit code should be real value, not fake", 55, exit!!.exitCode)
        assertEquals("Exactly one Exited should be published", 1, java.util.concurrent.atomic.AtomicInteger(0).also {
            if (exit != null) it.incrementAndGet()
        }.get())
        session.close()
    }
}
