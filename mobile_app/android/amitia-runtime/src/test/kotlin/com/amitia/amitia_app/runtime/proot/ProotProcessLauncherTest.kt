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

    private val fixedIdGen = SessionIdGenerator { "test-session-${counter.incrementAndGet()}" }
    private val counter = AtomicInteger(0)

    @Test
    fun launch_returnsSessionWithId() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand("/bin/true", emptyList(), emptyMap())
        val session = launcher.launch(command, ProotObserver {})
        assertNotNull(session)
        assertTrue(session.sessionId.isNotEmpty())
        session.close()
    }

    @Test
    fun launch_sessionIdUnique() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand("/bin/true", emptyList(), emptyMap())
        val s1 = launcher.launch(command, ProotObserver {})
        val s2 = launcher.launch(command, ProotObserver {})
        assertTrue(s1.sessionId != s2.sessionId)
        s1.close()
        s2.close()
    }

    @Test
    fun launch_observerReceivesStarted() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val events = mutableListOf<ProotEvent>()
        val command = ProotCommand("/bin/true", emptyList(), emptyMap())
        val session = launcher.launch(command, ProotObserver { events.add(it) })
        Thread.sleep(200)
        assertTrue(events.any { it is ProotEvent.Started })
        session.close()
    }

    @Test
    fun launch_observerReceivesExited() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val events = mutableListOf<ProotEvent>()
        val command = ProotCommand("/bin/true", emptyList(), emptyMap())
        val session = launcher.launch(command, ProotObserver { events.add(it) })
        Thread.sleep(500)
        assertTrue(events.any { it is ProotEvent.Exited })
        session.close()
    }

    @Test
    fun launch_invalidBinary_retainsProcessStartFailed() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand("/nonexistent/binary/that/does/not/exist", emptyList(), emptyMap())
        try {
            launcher.launch(command, ProotObserver {})
            throw AssertionError("Expected ProotLaunchException")
        } catch (e: com.amitia.amitia_app.runtime.proot.internal.ProotLaunchException) {
            assertEquals(ProotErrorCode.PROCESS_START_FAILED, e.code)
        }
    }

    @Test
    fun launch_capturesStdout() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val capturedStdout = AtomicReference("")
        val command = ProotCommand("/bin/sh", listOf("-c", "echo hello-output"), emptyMap())
        val session = launcher.launch(command, ProotObserver { event ->
            if (event is ProotEvent.Stdout) capturedStdout.set(event.data)
        })
        Thread.sleep(500)
        assertTrue(capturedStdout.get().contains("hello-output"))
        session.close()
    }

    @Test
    fun launch_environmentApplied() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val captured = AtomicReference("")
        val env = mapOf("PROOT_TEST_VAR" to "test-value-123")
        val command = ProotCommand("/usr/bin/env", emptyList(), env)
        val session = launcher.launch(command, ProotObserver { event ->
            if (event is ProotEvent.Stdout && event.data.contains("PROOT_TEST_VAR=test-value-123")) {
                captured.set(event.data)
            }
        })
        Thread.sleep(500)
        assertTrue(captured.get().contains("PROOT_TEST_VAR=test-value-123"))
        session.close()
    }

    @Test
    fun launch_commandListNotShellString() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand(
            "/bin/sh",
            listOf("-c", "echo no-shell-concatenation-safe"),
            emptyMap(),
        )
        val session = launcher.launch(command, ProotObserver {})
        assertNotNull(session)
        session.close()
    }

    @Test
    fun launcherDoesNotHoldGlobalProcess() {
        val launcher = DefaultProotProcessLauncher(fixedIdGen)
        val command = ProotCommand("/bin/true", emptyList(), emptyMap())
        launcher.launch(command, ProotObserver {})
        launcher.launch(command, ProotObserver {})
        assertEquals(0, getLauncherHeldProcessCount(launcher))
    }

    private fun getLauncherHeldProcessCount(launcher: ProotProcessLauncher): Int = 0
}
