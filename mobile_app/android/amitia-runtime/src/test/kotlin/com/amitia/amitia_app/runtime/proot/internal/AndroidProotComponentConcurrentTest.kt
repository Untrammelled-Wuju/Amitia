package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

internal class FakeProotArtifactVerifier(
    private val result: ProotAvailability
) : ProotArtifactVerifier {
    override fun verify(): ProotAvailability = result
}

class AndroidProotComponentConcurrentTest {

    private fun makeComponent(verifierResult: ProotAvailability): AndroidProotComponent {
        val locator = FakeLocator()
        val verifier = FakeProotArtifactVerifier(verifierResult)
        return AndroidProotComponent(
            binaryLocator = locator,
            artifactVerifier = verifier,
            commandBuilder = FakeCommandBuilder(),
            processLauncher = AliveProcessLauncher(),
            abiGate = null
        )
    }

    private fun launchRequest(): ProotLaunchRequest = ProotLaunchRequest.create(
        rootfsPath = "/rootfs",
        workingDirectory = "/",
        command = listOf("/opt/amitia/backend/amitia-server"),
        bindMountsSource = emptyList(),
        environmentSource = ProotEnvironment.EMPTY
    )

    @Test
    fun `concurrent launch returns already running when main session alive`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        val first = comp.launch(launchRequest(), ProotObserver {})
        assertTrue(first.isAlive())
        assertEquals(first, comp.currentSession())
        val second = comp.launch(launchRequest(), ProotObserver {})
        assertFalse(second.isAlive())
        assertTrue(second.sessionId.startsWith("already-running"))
    }

    @Test
    fun `second launch accepted after first session exits`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        val first = comp.launch(launchRequest(), ProotObserver {})
        first.close()
        val second = comp.launch(launchRequest(), ProotObserver {})
        assertTrue(second.isAlive())
    }

    @Test
    fun `current session null after stop`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        comp.launch(launchRequest(), ProotObserver {})
        val result = comp.stop()
        assertTrue(result is ProotStopResult.Graceful || result is ProotStopResult.Forced)
        assertNull(comp.currentSession())
    }

    @Test
    fun `stop idempotent after no session`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        val r1 = comp.stop()
        val r2 = comp.stop()
        assertTrue(r1 is ProotStopResult.AlreadyStopped)
        assertTrue(r2 is ProotStopResult.AlreadyStopped)
    }

    @Test
    fun `probe does not block concurrent launch check`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        val probe = comp.launchProbe(launchRequest(), ProotObserver {})
        assertTrue(probe.isAlive())
        assertEquals(null, comp.currentSession())
        val main = comp.launch(launchRequest(), ProotObserver {})
        assertTrue(main.isAlive())
    }

    @Test
    fun `two thread concurrent launch only one succeeds`() {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        val comp = makeComponent(avail)
        val latch = CountDownLatch(2)
        val successCount = AtomicInteger(0)
        val alreadyRunningCount = AtomicInteger(0)
        val threads = List(2) {
            Thread {
                try {
                    val session = comp.launch(launchRequest(), ProotObserver {})
                    if (session.isAlive()) successCount.incrementAndGet()
                    else if (session.sessionId.startsWith("already-running")) alreadyRunningCount.incrementAndGet()
                } finally { latch.countDown() }
            }
        }
        threads.forEach { it.start() }
        latch.await(5, TimeUnit.SECONDS)
        assertEquals(1, successCount.get())
        assertEquals(1, alreadyRunningCount.get())
    }

    private class FakeLocator : ProotBinaryLocator {
        override fun locate(): File? = File("/usr/lib/libamitia_proot.so").apply { parentFile?.mkdirs() }
    }

    private class FakeCommandBuilder : ProotCommandBuilder {
        override fun build(binaryPath: String, request: ProotLaunchRequest): ProotCommand =
            ProotCommand(binaryPath, listOf(binaryPath) + request.command, emptyMap())
    }

    private class AliveProcessLauncher : ProotProcessLauncher {
        override fun launch(command: ProotCommand, observer: ProotObserver): ProotSession =
            object : ProotSession {
                private var closed = false
                override val sessionId = "alive-" + System.nanoTime()
                override fun isAlive() = !closed
                override fun awaitExit(timeoutMillis: Long) = 0
                override fun stop(graceMillis: Long) = ProotStopResult.Graceful(sessionId, 0)
                override fun close() { closed = true }
            }
    }
}
