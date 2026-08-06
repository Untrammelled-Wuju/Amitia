package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.abi.UnsupportedReason
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotCommand
import com.amitia.amitia_app.runtime.proot.ProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotEnvironment
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class AndroidProotComponentAbiTest {

    private fun arm64Snapshot(): RuntimeAbiSnapshot = RuntimeAbiSnapshot(
        supportedAbis = listOf("arm64-v8a"),
        supported64BitAbis = listOf("arm64-v8a"),
        supported32BitAbis = emptyList(),
        processIs64Bit = true,
        osArchitecture = "aarch64"
    )

    private fun x86Snapshot(): RuntimeAbiSnapshot = RuntimeAbiSnapshot(
        supportedAbis = listOf("x86_64"),
        supported64BitAbis = listOf("x86_64"),
        supported32BitAbis = listOf("x86"),
        processIs64Bit = true,
        osArchitecture = "amd64"
    )

    private fun makeComponent(gate: RuntimeAbiGate?): AndroidProotComponent {
        val locator = FakeLocator()
        val verifier = DefaultProotArtifactVerifier(locator = locator, metadataLoader = null)
        return AndroidProotComponent(
            binaryLocator = locator,
            artifactVerifier = verifier,
            commandBuilder = FakeCommandBuilder(),
            processLauncher = FakeProcessLauncher(),
            abiGate = gate
        )
    }

    private fun launchRequest(): ProotLaunchRequest = ProotLaunchRequest.create(
        rootfsPath = "/rootfs",
        workingDirectory = "/",
        command = listOf("/bin/echo"),
        bindMountsSource = emptyList(),
        environmentSource = ProotEnvironment.EMPTY
    )

    @Test
    fun `unsupported abi availability returns Unavailable UNSUPPORTED_ABI`() {
        val gate = FakeGate(RuntimeAbiStatus.Unsupported(UnsupportedReason.ARM64_ABI_MISSING, x86Snapshot()))
        val comp = makeComponent(gate)
        val avail = comp.availability()
        assertTrue(avail is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.UNSUPPORTED_ABI, (avail as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun `unsupported abi launch returns closed session`() {
        val gate = FakeGate(RuntimeAbiStatus.Unsupported(UnsupportedReason.SUPPORTED_ABIS_EMPTY, arm64Snapshot()))
        val comp = makeComponent(gate)
        val result = comp.launch(launchRequest(), ProotObserver {})
        assertFalse(result.isAlive())
    }

    @Test
    fun `supported abi continues verifier path`() {
        val gate = FakeGate(RuntimeAbiStatus.Supported("arm64-v8a", true, arm64Snapshot()))
        val comp = makeComponent(gate)
        val avail = comp.availability()
        assertTrue(avail is ProotAvailability.Unavailable)
    }

    @Test
    fun `null gate does not block availability`() {
        val comp = makeComponent(null)
        val avail = comp.availability()
        assertNotNull(avail)
    }

    @Test
    fun `null gate does not block launch`() {
        val comp = makeComponent(null)
        val result = comp.launch(launchRequest(), ProotObserver {})
        assertNotNull(result)
    }

    @Test
    fun `unsupported abi bypassed messageKey`() {
        val gate = FakeGate(RuntimeAbiStatus.Unsupported(UnsupportedReason.PROCESS_IS_32_BIT, arm64Snapshot()))
        val comp = makeComponent(gate)
        val avail = comp.availability()
        val unavailable = avail as ProotAvailability.Unavailable
        assertEquals("proot.abi.unsupported", unavailable.messageKey)
    }

    private class FakeGate(private val status: RuntimeAbiStatus) : RuntimeAbiGate {
        override fun evaluate(): RuntimeAbiStatus = status
        override fun isSupported(): Boolean = status is RuntimeAbiStatus.Supported
    }

    private class FakeLocator : ProotBinaryLocator {
        override fun locate(): File? = null
    }

    private class FakeCommandBuilder : ProotCommandBuilder {
        override fun build(binaryPath: String, request: ProotLaunchRequest): ProotCommand =
            ProotCommand(binaryPath, listOf(binaryPath) + request.command, emptyMap())
    }

    private class FakeProcessLauncher : ProotProcessLauncher {
        override fun launch(command: ProotCommand, observer: ProotObserver): ProotSession {
            return object : ProotSession {
                override val sessionId = "fake"
                override fun isAlive() = false
                override fun awaitExit(timeoutMillis: Long) = 0
                override fun stop(graceMillis: Long) = ProotStopResult.AlreadyStopped("fake", null)
                override fun close() {}
            }
        }
    }
}
