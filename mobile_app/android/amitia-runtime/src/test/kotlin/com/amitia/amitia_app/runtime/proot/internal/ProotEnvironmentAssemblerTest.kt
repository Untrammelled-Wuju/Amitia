package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class ProotEnvironmentAssemblerTest {

    private fun createLayout(): DefaultRuntimeHostLayout {
        return DefaultRuntimeHostLayout.fromContext(
            noBackupFilesDir = File("/data/user/0/com.amitia.amitia_app/no_backup"),
            filesDir = File("/data/user/0/com.amitia.amitia_app/files"),
        )
    }

    private fun fakeBuilder(): RuntimeEnvironmentBuilder {
        return RuntimeEnvironmentBuilder { request ->
            RuntimeEnvironmentResult.Success(
                com.amitia.amitia_app.runtime.proot.RuntimeEnvironment(
                    hostProcess = mapOf("TMPDIR" to request.hostLayout.runRoot.absolutePath + "/tmp"),
                    guestRuntime = mapOf(
                        "AMITIA_RUNTIME_ROOT" to "/opt/amitia",
                        "AMITIA_SERVER_HOST" to "127.0.0.1",
                        "AMITIA_SERVER_PORT" to "18899",
                    ),
                )
            )
        }
    }

    @Test
    fun assembleRootfsProbeReturnsValidSpec() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleRootfsProbe()
        assertNotNull(spec)
        assertEquals("/opt/amitia", spec.workingDirectory)
        assertEquals(listOf("/usr/bin/env"), spec.command)
    }

    @Test
    fun assembleBackendLaunchReturnsCommand() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch()
        assertEquals(listOf("/opt/amitia/backend/amitia-server"), spec.command)
    }

    @Test
    fun toProotLaunchRequestConvertsCorrectly() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch()
        val request = assembler.toProotLaunchRequest(spec)
        assertEquals(listOf("/opt/amitia/backend/amitia-server"), request.command)
        assertEquals("/opt/amitia", request.workingDirectory)
    }

    @Test
    fun assembleHasBindMounts() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch()
        assertTrue(spec.bindMounts.isNotEmpty())
    }
}
