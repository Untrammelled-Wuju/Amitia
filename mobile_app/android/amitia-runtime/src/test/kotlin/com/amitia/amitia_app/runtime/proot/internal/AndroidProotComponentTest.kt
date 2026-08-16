package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotLaunchSpec
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.proot.ProotTerminationResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

internal val testAbiSnapshot = RuntimeAbiSnapshot(
    supportedAbis = listOf("arm64-v8a"),
    supported64BitAbis = listOf("arm64-v8a"),
    supported32BitAbis = emptyList(),
    processIs64Bit = true,
    osArchitecture = "aarch64",
)

internal class AlwaysSupportedAbiGate : RuntimeAbiGate {
    override fun evaluate(): RuntimeAbiStatus = RuntimeAbiStatus.Supported(
        abi = "arm64-v8a",
        processIs64Bit = true,
        snapshot = testAbiSnapshot,
    )
    override fun isSupported(): Boolean = true
}

internal class AbiUnsupportedGate : RuntimeAbiGate {
    override fun evaluate(): RuntimeAbiStatus = RuntimeAbiStatus.Unsupported(
        reason = com.amitia.amitia_app.runtime.abi.UnsupportedReason.ARM64_ABI_MISSING,
        snapshot = testAbiSnapshot,
    )
    override fun isSupported(): Boolean = false
}

internal class CountingProotProcessLauncher : ProotProcessLauncher {
    var launchCount = 0

    override fun launch(
        command: ProotCommand,
        observer: ProotObserver,
        generation: Long,
    ): ProotSession {
        launchCount++
        return CountingProotSession(generation)
    }
}

internal class CountingProotSession(
    private val gen: Long,
) : ProotSession {
    override val sessionId: String = "counting-session-$gen"
    private val alive = java.util.concurrent.atomic.AtomicBoolean(true)
    override fun isAlive(): Boolean = alive.get()
    override fun awaitExit(timeoutMillis: Long): Int? = if (alive.get()) null else 0
    override fun activate() {}
    override fun terminateAndConfirmExit(gracefulTimeoutMs: Long, forceTimeoutMs: Long): ProotTerminationResult {
        return ProotTerminationResult.ConfirmedExited(exit?.exitCode)
    }
    override fun stop(graceMillis: Long): ProotStopResult {
        alive.set(false)
        return ProotStopResult.Graceful(sessionId, 0)
    }
    override fun close() { alive.set(false) }
    override fun requestStop() {}
    override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? get() = null
}

internal class AndroidProotComponentTest {

    private fun createComponent(
        abiGate: RuntimeAbiGate? = null,
        launcher: CountingProotProcessLauncher = CountingProotProcessLauncher(),
    ): AndroidProotComponent {
        val artifact = ProotArtifact.create("1.0.0", "a".repeat(64))
        return AndroidProotComponent(
            binaryLocator = { File("/test/proot") },
            artifactVerifier = { ProotAvailability.Available(artifact, "/test/proot") },
            commandBuilder = { spec -> ProotCommand(spec.binaryPath, emptyList(), emptyMap()) },
            processLauncher = launcher,
            abiGate = abiGate,
        )
    }

    private fun createRequest(): ProotLaunchRequest {
        return ProotLaunchRequest.create(
            rootfsPath = "/rootfs",
            workingDirectory = "/opt/amitia",
            command = listOf("/opt/amitia/server"),
            bindMounts = emptyList(),
            environmentSource = com.amitia.amitia_app.runtime.proot.ProotEnvironment.EMPTY,
        )
    }

    @Test
    fun l013_alreadyRunning_returnsClosedSession() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        val session1 = component.launch(request, observer, 1L)
        assertTrue(session1.isAlive())
        assertEquals(1, launcher.launchCount)

        val session2 = component.launch(request, observer, 2L)
        assertFalse(session2.isAlive())
        assertEquals(1, launcher.launchCount)

        assertTrue(session1.isAlive())
    }

    @Test
    fun l013_probeWhenRunning_returnsClosedSession() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        val session1 = component.launch(request, observer, 1L)
        assertTrue(session1.isAlive())
        assertEquals(1, launcher.launchCount)

        val probeSession = component.launchProbe(request, observer, 2L)
        assertFalse(probeSession.isAlive())
        assertEquals(1, launcher.launchCount)

        assertTrue(session1.isAlive())
    }

    @Test
    fun currentSession_afterDeath_allowsNewLaunch() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        val session1 = component.launch(request, observer, 1L)
        assertEquals(session1, component.currentSession())

        session1.stop(100L)
        assertTrue(session1.exit != null)

        val session2 = component.launch(request, observer, 2L)
        assertTrue(session2.isAlive())
        assertEquals(2, launcher.launchCount)
    }

    @Test
    fun availability_abiUnsupported_returnsUnavailable() {
        val component = createComponent(abiGate = AbiUnsupportedGate())
        val avail = component.availability()
        assertTrue(avail !is ProotAvailability.Available)
    }

    @Test
    fun launch_abiUnsupported_returnsClosedSession() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(abiGate = AbiUnsupportedGate(), launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        val session = component.launch(request, observer, 1L)
        assertFalse(session.isAlive())
        assertEquals(0, launcher.launchCount)
    }

    @Test
    fun stop_activeSession_stopsSuccessfully() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        component.launch(request, observer, 1L)
        val result = component.stop()

        assertTrue(result is ProotStopResult.Graceful)
        assertTrue(result.sessionId.isNotEmpty())
    }

    @Test
    fun close_cleansUpAllSessions() {
        val launcher = CountingProotProcessLauncher()
        val component = createComponent(launcher = launcher)
        val request = createRequest()
        val observer = ProotObserver { }

        component.launch(request, observer, 1L)
        component.close()

        val session = component.launch(request, observer, 2L)
        assertFalse(session.isAlive())
    }

    @Test
    fun availability_cachedAfterFirstCall() {
        val component = createComponent()
        val avail1 = component.availability()
        val avail2 = component.availability()
        assertEquals(avail1, avail2)
    }
}
