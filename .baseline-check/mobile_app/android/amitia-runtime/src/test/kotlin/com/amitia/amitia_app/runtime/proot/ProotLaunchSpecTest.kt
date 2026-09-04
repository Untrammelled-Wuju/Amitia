package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Test

class ProotLaunchSpecTest {

    @Test fun create_fullSpec() {
        val env = ProotEnvironment.of(mapOf("HOME" to "/root"))
        val mounts = listOf(ProotBindMount.create("/data/runtime", "/opt/amitia"))
        val spec = ProotLaunchSpec(
            binaryPath = "/usr/lib/libamitia_proot.so",
            rootfsPath = "/data/user/0/app/files/rootfs",
            workingDirectory = "/opt/amitia",
            command = listOf("/opt/amitia/backend/amitia-server"),
            bindMounts = mounts,
            environment = env,
            fakeRoot = true,
            killOnExit = true,
        )
        assertEquals("/usr/lib/libamitia_proot.so", spec.binaryPath)
        assertEquals("/data/user/0/app/files/rootfs", spec.rootfsPath)
        assertEquals("/opt/amitia", spec.workingDirectory)
        assertEquals(listOf("/opt/amitia/backend/amitia-server"), spec.command)
        assertEquals(1, spec.bindMounts.size)
        assertEquals("/root", spec.environment.toMap()["HOME"])
    }

    @Test fun from_request_copiesAllFields() {
        val request = ProotLaunchRequest.create(
            rootfsPath = "/rootfs",
            workingDirectory = "/opt/amitia",
            command = listOf("/opt/amitia/backend/amitia-server"),
            bindMountsSource = listOf(ProotBindMount.create("/data", "/mnt")),
            environmentSource = ProotEnvironment.of(mapOf("FOO" to "bar")),
            fakeRoot = true,
            killOnExit = false,
        )
        val spec = ProotLaunchSpec.from(request, "/usr/lib/libamitia_proot.so")
        assertEquals("/usr/lib/libamitia_proot.so", spec.binaryPath)
        assertEquals("/rootfs", spec.rootfsPath)
        assertEquals("/opt/amitia", spec.workingDirectory)
        assertEquals(listOf("/opt/amitia/backend/amitia-server"), spec.command)
        assertEquals(1, spec.bindMounts.size)
        assertEquals("bar", spec.environment.toMap()["FOO"])
        assertEquals(true, spec.fakeRoot)
        assertEquals(false, spec.killOnExit)
    }

    @Test fun defaults() {
        val spec = ProotLaunchSpec(
            binaryPath = "/usr/lib/libamitia_proot.so",
            rootfsPath = "/rootfs",
            workingDirectory = "/",
            command = listOf("/bin/true"),
            bindMounts = emptyList(),
            environment = ProotEnvironment.EMPTY,
        )
        assertEquals(true, spec.fakeRoot)
        assertEquals(true, spec.killOnExit)
    }

    @Test(expected = IllegalArgumentException::class)
    fun from_blankBinaryPath_fails() {
        val request = ProotLaunchRequest.create(
            rootfsPath = "/rootfs",
            workingDirectory = "/",
            command = listOf("/bin/true"),
            bindMountsSource = emptyList(),
            environmentSource = ProotEnvironment.EMPTY,
        )
        ProotLaunchSpec.from(request, "")
    }

    @Test fun defaultWorkdir_isOptAmitia() {
        assertEquals("/opt/amitia", ProotLaunchSpec.DEFAULT_WORKDIR)
    }

    @Test fun spec_isDataClass() {
        val spec1 = ProotLaunchSpec("/bin/p", "/r", "/", listOf("/bin/x"), emptyList(), ProotEnvironment.EMPTY)
        val spec2 = ProotLaunchSpec("/bin/p", "/r", "/", listOf("/bin/x"), emptyList(), ProotEnvironment.EMPTY)
        assertEquals(spec1, spec2)
    }
}
