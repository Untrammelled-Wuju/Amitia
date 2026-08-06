package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Test

class ProotLaunchRequestTest {
    @Test fun valid_request() {
        val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo", "hello"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        assertEquals("/rootfs", r.rootfsPath); assertEquals("/", r.workingDirectory); assertEquals(listOf("/bin/echo", "hello"), r.command)
    }
    @Test(expected = IllegalArgumentException::class) fun empty_rootfs_fails() { ProotLaunchRequest.create(rootfsPath = "", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun relative_rootfs_fails() { ProotLaunchRequest.create(rootfsPath = "rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun empty_command_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = emptyList(), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test fun defaults() { val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); assertEquals(true, r.fakeRoot); assertEquals(true, r.killOnExit) }
}