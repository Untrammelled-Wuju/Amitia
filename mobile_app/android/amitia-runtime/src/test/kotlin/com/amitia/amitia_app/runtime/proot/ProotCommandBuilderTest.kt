package com.amitia.amitia_app.runtime.proot

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ProotCommandBuilderTest {
    private val builder = TestProotCommandBuilder()

    @Test fun basic_structure() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/bin/proot", request)
        assertEquals("/bin/proot", cmd.binaryPath)
        assertTrue(cmd.arguments.isNotEmpty())
    }

    @Test fun fakeRoot_flag_present() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY, fakeRoot = true)
        val cmd = builder.build("/bin/proot", request)
        assertTrue(cmd.arguments.contains("-0"))
    }

    private class TestProotCommandBuilder : ProotCommandBuilder {
        override fun build(binaryPath: String, request: ProotLaunchRequest): ProotCommand {
            val args = ArrayList<String>()
            if (request.fakeRoot) args.add("-0")
            args.add("-r"); args.add(request.rootfsPath)
            args.add("-w"); args.add(request.workingDirectory)
            args.add("--"); args.addAll(request.command)
            return ProotCommand(binaryPath, args.toList(), request.environmentSource.toMap())
        }
    }
}