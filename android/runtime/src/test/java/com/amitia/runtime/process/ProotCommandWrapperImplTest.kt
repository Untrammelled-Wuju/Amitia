package com.amitia.runtime.process

import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.ProotBinaryManager
import com.amitia.runtime.manager.RuntimeStateMachine
import com.google.common.truth.Truth.assertThat
import io.mockk.Runs
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import io.mockk.slot
import io.mockk.verify
import org.junit.After
import org.junit.Before
import org.junit.Test
import java.io.File
import java.nio.file.Files

class ProotCommandWrapperImplTest {

    private lateinit var tempRoot: File
    private lateinit var prootBinaryManager: ProotBinaryManager
    private lateinit var rootfsManager: LinuxRootfsManager
    private lateinit var stateMachine: RuntimeStateMachine
    private lateinit var wrapper: ProotCommandWrapperImpl

    @Before
    fun setUp() {
        tempRoot = Files.createTempDirectory("proot-wrapper-test").toFile()
        prootBinaryManager = mockk(relaxed = true)
        rootfsManager = mockk(relaxed = true)
        stateMachine = mockk(relaxed = true)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        wrapper = ProotCommandWrapperImpl(prootBinaryManager, rootfsManager, stateMachine)
    }

    @After
    fun tearDown() {
        tempRoot.deleteRecursively()
    }

    private fun makeProotBinary(): File {
        val binDir = File(tempRoot, "bin").apply { mkdirs() }
        val prootPath = File(binDir, "proot")
        prootPath.writeText("dummy")
        prootPath.setExecutable(true, false)
        return prootPath
    }

    private fun stubProotAvailable(prootPath: File) {
        every { prootBinaryManager.isAvailable() } returns true
        every { prootBinaryManager.binaryPath() } returns prootPath
        every { prootBinaryManager.unavailableReason() } returns null
    }

    private fun stubRootfsDir(): File {
        val rootfsDir = File(tempRoot, "rootfs/minimal").apply { mkdirs() }
        every { rootfsManager.minimalRootfsDir() } returns rootfsDir
        return rootfsDir
    }

    @Test
    fun isProotAvailable_returns_true_when_proot_binary_available() {
        stubProotAvailable(makeProotBinary())
        assertThat(wrapper.isProotAvailable()).isTrue()
    }

    @Test
    fun isProotAvailable_returns_false_when_proot_binary_unavailable() {
        every { prootBinaryManager.isAvailable() } returns false
        assertThat(wrapper.isProotAvailable()).isFalse()
    }

    @Test
    fun fallbackReason_returns_null_when_proot_available() {
        every { prootBinaryManager.isAvailable() } returns true
        every { prootBinaryManager.unavailableReason() } returns null
        assertThat(wrapper.fallbackReason()).isNull()
    }

    @Test
    fun fallbackReason_returns_reason_from_manager() {
        every { prootBinaryManager.isAvailable() } returns false
        every { prootBinaryManager.unavailableReason() } returns "assets/proot 缺失"
        assertThat(wrapper.fallbackReason()).contains("assets/proot 缺失")
    }

    @Test
    fun wrap_returns_original_command_when_proot_unavailable() {
        every { prootBinaryManager.isAvailable() } returns false
        every { prootBinaryManager.unavailableReason() } returns "PRoot 不可用"
        val command = listOf("/data/bin/surreal", "start")
        val env = mapOf("HOME" to "/data")
        val workDir = File(tempRoot, "work").apply { mkdirs() }

        val result = wrapper.wrap(command, env, workDir)

        assertThat(result).isEqualTo(command)
        verify { stateMachine.emitError(any(), any(), any(), any()) }
    }

    @Test
    fun wrap_returns_original_command_for_empty_input() {
        stubProotAvailable(makeProotBinary())
        stubRootfsDir()
        val result = wrapper.wrap(emptyList(), emptyMap(), tempRoot)
        assertThat(result).isEmpty()
    }

    @Test
    fun wrap_produces_proot_prefix_with_rootfs_and_root_id() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        val rootfsDir = stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/surreal", "start", "--bind", "0.0.0.0:18000")
        val env = mapOf("HOME" to workDir.absolutePath, "TMPDIR" to "/tmp")

        val result = wrapper.wrap(command, env, workDir)

        assertThat(result).isNotEmpty()
        assertThat(result.first()).isEqualTo(prootPath.absolutePath)
        assertThat(result).contains("--rootfs=${rootfsDir.absolutePath}")
        assertThat(result).contains("--root-id")
        assertThat(result).contains("--cwd=${workDir.absolutePath}")
        assertThat(result).contains("--bind=/dev")
        assertThat(result).contains("--bind=/proc")
        assertThat(result).contains("--bind=/sys")
        assertThat(result).contains("--")
        val dashIndex = result.indexOf("--")
        assertThat(dashIndex).isGreaterThan(0)
        assertThat(result.subList(dashIndex + 1, result.size)).isEqualTo(command)
    }

    @Test
    fun wrap_includes_env_vars_when_env_non_empty() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/qdrant", "--config-path", "/data/qdrant/config.yaml")
        val env = mapOf(
            "HOME" to "/data/qdrant",
            "QDRANT__SERVICE__HTTP_PORT" to "19178"
        )

        val result = wrapper.wrap(command, env, workDir)

        assertThat(result).contains("--env")
        val envIndex = result.indexOf("--env")
        assertThat(envIndex).isGreaterThan(0)
        assertThat(result).contains("HOME=/data/qdrant")
        assertThat(result).contains("QDRANT__SERVICE__HTTP_PORT=19178")
        val dashIndex = result.indexOf("--")
        assertThat(dashIndex).isGreaterThan(envIndex)
    }

    @Test
    fun wrap_omits_env_block_when_env_empty() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/backend")

        val result = wrapper.wrap(command, emptyMap(), workDir)

        assertThat(result).doesNotContain("--env")
        assertThat(result).contains("--")
    }

    @Test
    fun wrap_binds_data_directory_when_exists() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        val rootfsDir = File(tempRoot, "runtime/rootfs/minimal").apply { mkdirs() }
        every { rootfsManager.minimalRootfsDir() } returns rootfsDir
        val workDir = File(tempRoot, "runtime/amitia-data").apply { mkdirs() }
        val command = listOf("/bin/backend")
        val env = mapOf("HOME" to workDir.absolutePath)

        val result = wrapper.wrap(command, env, workDir)

        val dataBindCount = result.count { it.startsWith("--bind=") }
        assertThat(dataBindCount).isAtLeast(4)
    }

    @Test
    fun wrap_logs_info_with_rootfs_and_workDir_paths() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        val rootfsDir = stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/backend")
        val env = mapOf("HOME" to workDir.absolutePath)

        wrapper.wrap(command, env, workDir)

        val logSlot = slot<String>()
        verify {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                "ProotWrapper",
                capture(logSlot)
            )
        }
        val logMsg = logSlot.captured
        assertThat(logMsg).contains("rootfs=${rootfsDir.absolutePath}")
        assertThat(logMsg).contains("workDir=${workDir.absolutePath}")
    }

    @Test
    fun wrap_returns_original_command_when_binary_path_null() {
        every { prootBinaryManager.isAvailable() } returns true
        every { prootBinaryManager.binaryPath() } returns null
        stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/backend")

        val result = wrapper.wrap(command, emptyMap(), workDir)

        assertThat(result).isEqualTo(command)
    }

    @Test
    fun wrap_handles_multiple_commands_and_preserves_argument_order() {
        val prootPath = makeProotBinary()
        stubProotAvailable(prootPath)
        stubRootfsDir()
        val workDir = File(tempRoot, "work").apply { mkdirs() }
        val command = listOf("/bin/surreal", "start", "--user", "root", "--password", "secret", "--bind", "127.0.0.1:18000")

        val result = wrapper.wrap(command, emptyMap(), workDir)

        val dashIndex = result.indexOf("--")
        assertThat(dashIndex).isGreaterThan(0)
        assertThat(result.subList(dashIndex + 1, result.size)).isEqualTo(command)
    }
}
