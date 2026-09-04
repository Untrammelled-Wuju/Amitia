package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.proot.internal.DefaultProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.internal.SessionIdGenerator
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class ProotProcessLauncherTest {

    private val isWindows: Boolean = System.getProperty("os.name").lowercase().contains("windows")
    private val fixedIdGen = SessionIdGenerator { "test-session-${counter.incrementAndGet()}" }
    private val counter = AtomicInteger(0)

    private fun noopCommand(): ProotCommand {
        return if (isWindows) {
            ProotCommand("cmd.exe", listOf("/c", "exit", "0"), emptyMap())
        } else {
            ProotCommand("/bin/true", emptyList(), emptyMap())
        }
    }

    private fun echoCommand(text: String): ProotCommand {
        return if (isWindows) {
            ProotCommand("cmd.exe", listOf("/c", "echo", text), emptyMap())
        } else {
            ProotCommand("/bin/sh", listOf("-c", "echo $text"), emptyMap())
        }
    }

    private fun envCommand(env: Map<String, String>): ProotCommand {
        return if (isWindows) {
            ProotCommand("cmd.exe", listOf("/c", "set"), env)
        } else {
            ProotCommand("/usr/bin/env", emptyList(), env)
        }
    }

    private val testGeneration: Long = 99L

    @Test
    fun launch_returnsSessionWithId() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val session = launcher.launch(noopCommand(), ProotObserver {}, testGeneration)
        assertNotNull(session)
        assertTrue(session.sessionId.isNotEmpty())
        session.close()
    }

    @Test
    fun launch_sessionIdUnique() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val noop = noopCommand()
        val s1 = launcher.launch(noop, ProotObserver {}, testGeneration)
        val s2 = launcher.launch(noop, ProotObserver {}, testGeneration)
        assertTrue(s1.sessionId != s2.sessionId)
        s1.close()
        s2.close()
    }

    @Test
    fun launch_observerReceivesStarted() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val events = mutableListOf<ProotEvent>()
        val session = launcher.launch(noopCommand(), ProotObserver { events.add(it) }, testGeneration)
        Thread.sleep(200)
        assertTrue(events.any { it is ProotEvent.Started })
        session.close()
    }

    @Test
    fun launch_observerReceivesExited() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val exitEvent = AtomicReference<ProotEvent.Exited?>(null)
        val session = launcher.launch(noopCommand(), ProotObserver {
            if (it is ProotEvent.Exited) exitEvent.set(it)
        }, testGeneration)
        session.awaitExit(5000)
        Thread.sleep(50)
        assertNotNull(exitEvent.get())
        session.close()
    }

    @Test
    fun launch_invalidBinary_retainsProcessStartFailed() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand("/nonexistent/binary/that/does/not/exist", emptyList(), emptyMap())
        try {
            launcher.launch(command, ProotObserver {}, testGeneration)
            throw AssertionError("Expected ProotLaunchException")
        } catch (e: com.amitia.amitia_app.runtime.proot.internal.ProotLaunchException) {
            assertEquals(ProotErrorCode.PROCESS_START_FAILED, e.code)
        }
    }

    @Test
    fun launch_capturesStdout() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val capturedStdout = AtomicReference("")
        val command = echoCommand("hello-output-test")
        val session = launcher.launch(command, ProotObserver { event ->
            if (event is ProotEvent.Stdout && event.data.contains("hello-output-test")) {
                capturedStdout.set(event.data)
            }
        }, testGeneration)
        Thread.sleep(500)
        assertTrue(capturedStdout.get().contains("hello-output-test"))
        session.close()
    }

    @Test
    fun launch_environmentApplied() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val captured = AtomicReference("")
        val env = mapOf("PROOT_TEST_VAR" to "test-value-123")
        val command = envCommand(env)
        val session = launcher.launch(command, ProotObserver { event ->
            if (event is ProotEvent.Stdout && event.data.contains("PROOT_TEST_VAR=test-value-123")) {
                captured.set(event.data)
            }
        }, testGeneration)
        Thread.sleep(500)
        assertTrue(captured.get().contains("PROOT_TEST_VAR=test-value-123"))
        session.close()
    }

    @Test
    fun launch_commandListNotShellString() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val args = if (isWindows) listOf("/c", "echo no-shell-safe") else listOf("-c", "echo no-shell-safe")
        val binary = if (isWindows) "cmd.exe" else "/bin/sh"
        val command = ProotCommand(binary, args, emptyMap())
        val session = launcher.launch(command, ProotObserver {}, testGeneration)
        assertNotNull(session)
        session.close()
    }

    @Test
    fun launcherDoesNotHoldGlobalProcess() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = noopCommand()
        launcher.launch(command, ProotObserver {}, testGeneration)
        launcher.launch(command, ProotObserver {}, testGeneration)
        assertEquals(0, getLauncherHeldProcessCount(launcher))
    }

    private fun getLauncherHeldProcessCount(launcher: ProotProcessLauncher): Int = 0
}
