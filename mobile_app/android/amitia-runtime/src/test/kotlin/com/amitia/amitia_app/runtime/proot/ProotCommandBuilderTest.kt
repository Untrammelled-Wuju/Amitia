package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.proot.internal.DefaultProotCommandBuilder
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.assertFalse
import org.junit.Test

class ProotCommandBuilderTest {
    private val builder: ProotCommandBuilder = DefaultProotCommandBuilder()

    @Test fun normal_rootfs() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("/rootfs"))
        assertTrue(cmd.arguments.indexOf("-r") < cmd.arguments.indexOf("/rootfs"))
    }

    @Test fun working_directory() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/opt/amitia", command = listOf("/opt/amitia/backend/amitia-server"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val wIdx = cmd.arguments.indexOf("-w")
        assertTrue(wIdx >= 0)
        assertEquals("/opt/amitia", cmd.arguments[wIdx + 1])
    }

    @Test fun single_bind() {
        val mount = ProotBindMount.create("/data/runtime", "/opt/amitia")
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/true"), bindMountsSource = listOf(mount), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val bIdx = cmd.arguments.indexOf("-b")
        assertTrue(bIdx >= 0)
        assertEquals("/data/runtime:/opt/amitia", cmd.arguments[bIdx + 1])
    }

    @Test fun multiple_binds() {
        val mounts = listOf(
            ProotBindMount.create("/data/runtime", "/opt/amitia"),
            ProotBindMount.create("/data/config", "/etc/amitia"),
            ProotBindMount.create("/data/data", "/var/lib/amitia"),
            ProotBindMount.create("/data/cache", "/var/cache/amitia"),
            ProotBindMount.create("/data/logs", "/var/log/amitia"),
            ProotBindMount.create("/data/run", "/run/amitia"),
        )
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/opt/amitia/backend/amitia-server"), bindMountsSource = mounts, environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val bindCount = cmd.arguments.count { it == "-b" }
        assertEquals(6, bindCount)
        assertTrue(cmd.arguments.contains("/data/runtime:/opt/amitia"))
        assertTrue(cmd.arguments.contains("/data/config:/etc/amitia"))
    }

    @Test fun environment_included() {
        val env = ProotEnvironment.of(mapOf("HOME" to "/root", "PATH" to "/usr/bin"))
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = env)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertEquals("/root", cmd.environment["HOME"])
        assertEquals("/usr/bin", cmd.environment["PATH"])
    }

    @Test fun guest_executable_preserved() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/opt/amitia/backend/amitia-server", "--config", "/etc/amitia/app/config.yaml"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("/opt/amitia/backend/amitia-server"))
        assertTrue(cmd.arguments.contains("--config"))
        assertTrue(cmd.arguments.contains("/etc/amitia/app/config.yaml"))
    }

    @Test fun kill_on_exit_present() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("--kill-on-exit"))
    }

    @Test fun fake_root_present() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("-0"))
    }

    @Test fun rootfs_r_correct() {
        val request = ProotLaunchRequest.create(rootfsPath = "/data/runtime/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val rIdx = cmd.arguments.indexOf("-r")
        assertTrue(rIdx >= 0)
        assertEquals("/data/runtime/rootfs", cmd.arguments[rIdx + 1])
    }

    @Test fun working_dir_w_correct() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/sh"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val wIdx = cmd.arguments.indexOf("-w")
        assertTrue(wIdx >= 0)
        assertEquals("/", cmd.arguments[wIdx + 1])
    }

    @Test fun double_dash_separator_present() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo", "hello"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("--"))
        val ddIdx = cmd.arguments.indexOf("--")
        assertTrue(cmd.arguments.indexOf("/bin/echo") > ddIdx)
    }

    @Test fun argument_order_deterministic() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo", "hello"), bindMountsSource = listOf(ProotBindMount.create("/data", "/mnt")), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        val killIdx = cmd.arguments.indexOf("--kill-on-exit")
        val zeroIdx = cmd.arguments.indexOf("-0")
        val rIdx = cmd.arguments.indexOf("-r")
        val wIdx = cmd.arguments.indexOf("-w")
        val bIdx = cmd.arguments.indexOf("-b")
        val ddIdx = cmd.arguments.indexOf("--")
        assertTrue(killIdx < zeroIdx)
        assertTrue(zeroIdx < rIdx)
        assertTrue(rIdx < wIdx)
        assertTrue(wIdx < bIdx)
        assertTrue(bIdx < ddIdx)
    }

    @Test fun no_shell_wrapper_in_command() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/opt/amitia/backend/amitia-server"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        for (arg in cmd.arguments) {
            assertFalse("Should not contain sh -c: $arg", arg.contains("sh -c") || arg.contains("bash -c") || arg.contains("/system/bin/sh"))
        }
    }

    @Test fun args_as_individual_entries() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/echo", "hello world", "--flag=value"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertTrue(cmd.arguments.contains("hello world"))
        assertTrue(cmd.arguments.contains("--flag=value"))
    }

    @Test fun binary_path_is_proot() {
        val request = ProotLaunchRequest.create(rootfsPath = "/rootfs", workingDirectory = "/", command = listOf("/bin/true"), bindMountsSource = emptyList(), environmentSource = ProotEnvironment.EMPTY)
        val cmd = builder.build("/usr/lib/libamitia_proot.so", request)
        assertEquals("/usr/lib/libamitia_proot.so", cmd.binaryPath)
    }
}