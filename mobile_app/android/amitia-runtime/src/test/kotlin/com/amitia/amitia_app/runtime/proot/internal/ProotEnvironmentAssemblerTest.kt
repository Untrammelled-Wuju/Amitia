package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.proot.GuestLayout
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentBuilder
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentRequest
import com.amitia.amitia_app.runtime.proot.RuntimeEnvironmentResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class ProotEnvironmentAssemblerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createLayout(): DefaultRuntimeHostLayout {
        val controlBase = tempFolder.newFolder("noBackupFiles")
        val dataBase = tempFolder.newFolder("files")
        return DefaultRuntimeHostLayout(controlBase, dataBase)
    }

    private fun createProgramSource(layout: DefaultRuntimeHostLayout): File {
        val versionDir = File(layout.runtimeVersionRoot("1.0.0"), "program")
        versionDir.mkdirs()
        return versionDir
    }

    private fun fakeBuilder(): RuntimeEnvironmentBuilder {
        return RuntimeEnvironmentBuilder { request ->
            RuntimeEnvironmentResult.Success(
                com.amitia.amitia_app.runtime.proot.RuntimeEnvironment(
                    hostProcess = mapOf("TMPDIR" to request.hostLayout.runRoot.absolutePath + "/tmp"),
                    guestRuntime = mapOf(
                        "AMITIA_RUNTIME_ROOT" to GuestLayout.PROGRAM,
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
        val spec = assembler.assembleRootfsProbe(createProgramSource(layout))
        assertNotNull(spec)
        assertEquals(GuestLayout.BACKEND_DIR, spec.workingDirectory)
        assertEquals(listOf("/usr/bin/env"), spec.command)
    }

    @Test
    fun assembleBackendLaunchReturnsCommand() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertEquals(listOf(GuestLayout.BACKEND_SERVER), spec.command)
    }

    @Test
    fun toProotLaunchRequestConvertsCorrectly() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        val request = assembler.toProotLaunchRequest(spec, "/usr/lib/libamitia_proot.so")
        assertEquals(listOf(GuestLayout.BACKEND_SERVER), request.command)
        assertEquals(GuestLayout.BACKEND_DIR, request.workingDirectory)
    }

    @Test
    fun specContainsAllRequiredMounts() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        val guests = spec.bindMounts.map { it.guest }
        assertTrue(guests.contains(GuestLayout.PROGRAM))
        assertTrue(guests.contains(GuestLayout.CONFIG))
        assertTrue(guests.contains(GuestLayout.DATA))
        assertTrue(guests.contains(GuestLayout.CACHE))
        assertTrue(guests.contains(GuestLayout.LOGS))
        assertTrue(guests.contains(GuestLayout.RUN))
        assertTrue(guests.contains(GuestLayout.HOME))
    }

    @Test
    fun specHasDeterministicBinaryPath() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertEquals("", spec.binaryPath)
    }

    @Test
    fun assembleHasBindMounts() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertTrue(spec.bindMounts.isNotEmpty())
    }

    @Test
    fun programMountPointsToActiveVersionNotVersionsRoot() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val programSource = createProgramSource(layout)
        val spec = assembler.assembleBackendLaunch(programSource)
        val programMount = spec.bindMounts.first { it.guest == GuestLayout.PROGRAM }
        assertEquals(programSource.absolutePath, programMount.host)
        assertTrue(programMount.host.contains("versions/1.0.0/program"))
    }

    @Test
    fun assembleFailsWhenProgramSourceIsVersionsRoot() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        try {
            assembler.assembleBackendLaunch(layout.versionsRoot)
            throw AssertionError("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun assembleFailsWhenProgramSourceDoesNotExist() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val nonExistent = File(layout.versionsRoot, "nonexistent/program")
        try {
            assembler.assembleBackendLaunch(nonExistent)
            throw AssertionError("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun mountOrderIsDeterministic() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec1 = assembler.assembleBackendLaunch(createProgramSource(layout))
        val spec2 = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertEquals(spec1.bindMounts.map { it.guest }, spec2.bindMounts.map { it.guest })
    }
}
