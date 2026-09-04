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
    @Test(expected = IllegalArgumentException::class) fun nul_in_rootfs_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs\u0000", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun nul_in_workdir_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/\u0000", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun nul_in_command_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo\u0000"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun relative_executable_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun executable_with_traversal_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/../etc/passwd"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun executable_with_semicolon_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo;rm -rf /"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun executable_with_pipe_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo|curl evil"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun executable_with_dollar_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo\$HOME"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test(expected = IllegalArgumentException::class) fun executable_with_backtick_fails() { ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo`whoami`"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY) }
    @Test fun amitia_server_path_valid() { val r = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/opt/amitia/backend/amitia-server", "--version"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY); assertEquals("/opt/amitia/backend/amitia-server", r.command.first()) }
}