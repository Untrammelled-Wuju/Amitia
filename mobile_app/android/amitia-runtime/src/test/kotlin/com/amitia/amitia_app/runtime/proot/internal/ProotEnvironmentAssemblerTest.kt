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
                    hostProcess = mapOf(
                        "TMPDIR" to request.hostLayout.runRoot.absolutePath + "/tmp",
                        "PROOT_TMP_DIR" to request.hostLayout.runRoot.absolutePath + "/proot-tmp",
                        "PROOT_NO_SECCOMP" to "1",
                        "ANDROID_ROOT" to "/system",
                        "ANDROID_DATA" to "/data",
                    ),
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
        assertEquals(listOf("/usr/bin/true"), spec.command)
    }

    @Test
    fun assembleBackendLaunchReturnsCommand() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertEquals(listOf(GuestLayout.BACKEND_SERVER, "--runtime-profile=local"), spec.command)
    }

    @Test
    fun toProotLaunchRequestConvertsCorrectly() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        val request = assembler.toProotLaunchRequest(spec)
        assertEquals(listOf(GuestLayout.BACKEND_SERVER, "--runtime-profile=local"), request.command)
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
    @Test
    fun hostProcessEnvironmentIsMergedIntoLaunchEnvironment() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        val spec = assembler.assembleBackendLaunch(createProgramSource(layout))
        assertEquals(layout.runRoot.absolutePath + "/proot-tmp", spec.environment.toMap()["PROOT_TMP_DIR"])
        assertEquals("1", spec.environment.toMap()["PROOT_NO_SECCOMP"])
        assertEquals("/system", spec.environment.toMap()["ANDROID_ROOT"])
        assertEquals("/data", spec.environment.toMap()["ANDROID_DATA"])
        assertEquals(GuestLayout.PROGRAM, spec.environment.toMap()["AMITIA_RUNTIME_ROOT"])
    }

    @Test
    fun assembleCreatesAllWritableHostMountSources() {
        val layout = createLayout()
        val assembler = ProotEnvironmentAssembler(layout = layout, environmentBuilder = fakeBuilder())
        assembler.assembleBackendLaunch(createProgramSource(layout))

        for (directory in listOf(layout.configRoot, layout.dataRoot, layout.cacheRoot, layout.logRoot, layout.runRoot, layout.homeRoot)) {
            assertTrue("expected host directory: ${directory.absolutePath}", directory.isDirectory)
        }
        assertTrue(File(layout.runRoot, "tmp").isDirectory)
        assertTrue(File(layout.runRoot, "proot-tmp").isDirectory)
        assertTrue(File(layout.dataRoot, "security").isDirectory)
        assertTrue(File(layout.dataRoot, "workspaces").isDirectory)
    }

}
