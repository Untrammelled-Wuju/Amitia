package com.amitia.runtime.bootstrap

import com.amitia.core.common.Constants
import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.amitia.runtime.health.HealthChecker
import com.amitia.runtime.linux.LinuxRootfsManager
import com.amitia.runtime.linux.ProotBinaryManager
import com.amitia.runtime.linux.ProotInstallPhase
import com.amitia.runtime.linux.ProotInstallProgress
import com.amitia.runtime.linux.RootfsInstallPhase
import com.amitia.runtime.linux.RootfsInstallProgress
import com.amitia.runtime.manager.RuntimeDirectories
import com.amitia.runtime.manager.RuntimeStateMachine
import com.amitia.runtime.process.LinuxProcessManager
import com.amitia.runtime.process.ProotCommandWrapper
import com.amitia.runtime.process.RestartPolicy
import com.google.common.truth.Truth.assertThat
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.coVerifyOrder
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.Test
import java.io.File

class BootstrapSequenceImplTest {

    private val directories: RuntimeDirectories = mockk(relaxed = true)
    private val rootfsManager: LinuxRootfsManager = mockk(relaxed = true)
    private val processManager: LinuxProcessManager = mockk(relaxed = true)
    private val healthChecker: HealthChecker = mockk(relaxed = true)
    private val stateMachine: RuntimeStateMachine = mockk(relaxed = true)
    private val prootBinaryManager: ProotBinaryManager = mockk(relaxed = true)
    private val prootCommandWrapper: ProotCommandWrapper = mockk(relaxed = true)

    private val bootstrap = BootstrapSequenceImpl(
        directories, rootfsManager, processManager, healthChecker, stateMachine,
        prootBinaryManager, prootCommandWrapper
    )

    private val noopStage: (com.amitia.runtime.api.RuntimeStage) -> Unit = {}

    private fun completedInstallFlow(message: String = "RootFS 安装完成") = flowOf(
        RootfsInstallProgress(
            phase = RootfsInstallPhase.COMPLETED,
            currentFile = "",
            bytesCopied = 0L,
            totalBytes = 0L,
            percent = 1f,
            message = message
        )
    )

    private fun failedInstallFlow(error: String) = flow {
        emit(
            RootfsInstallProgress(
                phase = RootfsInstallPhase.FAILED,
                currentFile = "",
                bytesCopied = 0L,
                totalBytes = 0L,
                percent = 0f,
                message = error,
                error = error
            )
        )
    }

    private fun stubCommonDirs(tempDir: File) {
        every { directories.binDir() } returns File(tempDir, "bin").apply { mkdirs() }
        every { directories.surrealdbDir() } returns File(tempDir, "surrealdb").apply { mkdirs() }
        every { directories.qdrantDir() } returns File(tempDir, "qdrant").apply { mkdirs() }
        every { directories.amitiaDataRoot() } returns File(tempDir, "data").apply { mkdirs() }
        every { directories.sqliteDir() } returns File(tempDir, "sqlite").apply { mkdirs() }
        every { directories.tmpDir() } returns File(tempDir, "tmp").apply { mkdirs() }
        every { directories.configDir() } returns File(tempDir, "config").apply { mkdirs() }
        coEvery { directories.ensureAllCreated() } returns Result.success(Unit)
        every { directories.validateIsolation() } returns true
        every { rootfsManager.minimalRootfsDir() } returns File(tempDir, "rootfs/minimal").apply { mkdirs() }
        coEvery { rootfsManager.ensureMinimalRootfs() } returns Result.success(Unit)
        every { prootBinaryManager.isAvailable() } returns true
        every { prootBinaryManager.version() } returns "proot-test-0.1.0"
        every { prootBinaryManager.binaryPath() } returns File(tempDir, "bin/proot").apply {
            writeText("dummy")
            setExecutable(true, false)
        }
        every { prootBinaryManager.unavailableReason() } returns null
        every { prootBinaryManager.install() } returns flowOf(
            ProotInstallProgress(
                phase = ProotInstallPhase.COMPLETED,
                percent = 1f,
                message = "PRoot 安装完成"
            )
        )
        every { prootCommandWrapper.isProotAvailable() } returns true
        every { prootCommandWrapper.fallbackReason() } returns null
        every { prootCommandWrapper.wrap(any(), any(), any()) } answers {
            firstArg<List<String>>()
        }
    }

    private fun stubBinaries(tempDir: File) {
        val bin = File(tempDir, "bin").apply { mkdirs() }
        listOf("amitia-backend-arm64", "qdrant_linux_aarch64", "surreal_linux_aarch64").forEach { name ->
            File(bin, name).apply {
                writeText("dummy")
                setExecutable(true, false)
            }
        }
    }

    private fun stubConfig(tempDir: File) {
        val configDir = File(tempDir, "config").apply { mkdirs() }
        val configFile = File(configDir, "config.yml")
        configFile.writeText("service: amitia")
        every { directories.configDir() } returns configDir
    }

    private fun stubHealthyFlow() {
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { rootfsManager.install() } returns completedInstallFlow()
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery {
            healthChecker.waitForHealthy(
                name = any(),
                check = any(),
                intervalMs = any(),
                timeoutMs = any()
            )
        } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
    }

    @Test
    fun start_returns_healthy_services_when_all_components_start() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-healthy")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        stubHealthyFlow()

        val result = bootstrap.start(noopStage)

        assertThat(result.isSuccess).isTrue()
        val services = result.getOrNull()
        assertThat(services).isNotNull()
        assertThat(services!!.surrealDb).isInstanceOf(ServiceState.Healthy::class.java)
        assertThat(services.qdrant).isInstanceOf(ServiceState.Healthy::class.java)
        assertThat(services.backend).isInstanceOf(ServiceState.Healthy::class.java)
        tempDir.deleteRecursively()
    }

    @Test
    fun start_installs_rootfs_when_not_installed() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-install")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        coEvery { rootfsManager.isInstalled() } returns false
        coEvery { rootfsManager.install() } returns completedInstallFlow()
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery { healthChecker.waitForHealthy(any(), any(), any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)

        val result = bootstrap.start(noopStage)

        assertThat(result.isSuccess).isTrue()
        coVerify(atLeast = 1) { rootfsManager.install() }
        tempDir.deleteRecursively()
    }

    @Test
    fun start_returns_failure_when_rootfs_install_fails() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-rootfs-fail")
        stubCommonDirs(tempDir)
        coEvery { rootfsManager.isInstalled() } returns false
        coEvery { rootfsManager.install() } returns failedInstallFlow("install failed")
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isFailure).isTrue()
        tempDir.deleteRecursively()
    }

    @Test
    fun start_returns_failure_when_backend_binary_missing() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-missing-binary")
        stubCommonDirs(tempDir)
        stubConfig(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isFailure).isTrue()
        val error = result.exceptionOrNull()
        assertThat(error?.message).contains("二进制")
        tempDir.deleteRecursively()
    }

    @Test
    fun start_auto_generates_config_when_missing() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-auto-config")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        val configDir = File(tempDir, "config").apply { mkdirs() }
        every { directories.configDir() } returns configDir
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery { healthChecker.waitForHealthy(any(), any(), any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isSuccess).isTrue()
        val generatedConfig = File(configDir, "config.yml")
        assertThat(generatedConfig.exists()).isTrue()
        assertThat(generatedConfig.readText()).contains("port: ${Constants.BACKEND_PORT}")
        tempDir.deleteRecursively()
    }

    @Test
    fun start_returns_failure_when_directory_creation_fails() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-dir-fail")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { directories.ensureAllCreated() } returns Result.failure(IllegalStateException("mkdir denied"))
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isFailure).isTrue()
        tempDir.deleteRecursively()
    }

    @Test
    fun start_returns_failure_when_directory_isolation_violated() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-isolation")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        every { directories.validateIsolation() } returns false
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isFailure).isTrue()
        assertThat(result.exceptionOrNull()?.message).contains("隔离")
        tempDir.deleteRecursively()
    }

    @Test
    fun start_returns_failure_when_backend_health_check_times_out() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-backend-timeout")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery { healthChecker.waitForHealthy(name = "surrealdb", any(), any(), any()) } returns Result.success(Unit)
        coEvery { healthChecker.waitForHealthy(name = "qdrant", any(), any(), any()) } returns Result.success(Unit)
        coEvery { healthChecker.waitForHealthy(name = "backend", any(), any(), any()) } returns Result.failure(java.util.concurrent.TimeoutException("backend health check timeout"))
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isFailure).isTrue()
        assertThat(result.exceptionOrNull()?.message).contains("健康检查超时")
        tempDir.deleteRecursively()
    }

    @Test
    fun start_transitions_to_degraded_when_surrealdb_unhealthy() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-degraded")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery { processManager.start(name = "surrealdb", any(), any(), any(), any(), any()) } returns Result.failure(IllegalStateException("surrealdb failed"))
        coEvery { healthChecker.waitForHealthy(name = "qdrant", any(), any(), any()) } returns Result.success(Unit)
        coEvery { healthChecker.waitForHealthy(name = "backend", any(), any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs

        val result = bootstrap.start(noopStage)

        assertThat(result.isSuccess).isTrue()
        val services = result.getOrNull()
        assertThat(services).isNotNull()
        assertThat(services!!.surrealDb).isInstanceOf(ServiceState.Unhealthy::class.java)
        assertThat(services.qdrant).isInstanceOf(ServiceState.Healthy::class.java)
        assertThat(services.backend).isInstanceOf(ServiceState.Healthy::class.java)
        tempDir.deleteRecursively()
    }

    @Test
    fun stop_invokes_stop_on_all_processes_in_reverse_order() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-stop")
        stubCommonDirs(tempDir)
        coEvery { processManager.stop(any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs

        val result = bootstrap.stop(noopStage)

        assertThat(result.isSuccess).isTrue()
        coVerifyOrder {
            processManager.stop("amitia-backend", any())
            processManager.stop("qdrant", any())
            processManager.stop("surrealdb", any())
        }
        tempDir.deleteRecursively()
    }

    @Test
    fun stop_transitions_state_machine_to_stopping_then_stopped() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-stop-state")
        stubCommonDirs(tempDir)
        coEvery { processManager.stop(any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs

        bootstrap.stop(noopStage)

        coVerify {
            stateMachine.transition(match { it is RuntimeState.Stopping })
            stateMachine.transition(RuntimeState.Stopped)
        }
        tempDir.deleteRecursively()
    }

    @Test
    fun restart_calls_stop_then_start() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-restart")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        stubHealthyFlow()

        val result = bootstrap.restart()

        assertThat(result.isSuccess).isTrue()
        coVerify(atLeast = 1) { processManager.stop(any(), any()) }
        coVerify(atLeast = 1) { processManager.start(any(), any(), any(), any(), any(), any()) }
        tempDir.deleteRecursively()
    }

    @Test
    fun repair_reinstalls_rootfs_when_verify_fails() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-repair")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        coEvery { rootfsManager.isInstalled() } returns true
        coEvery { rootfsManager.verify() } returns Result.success(
            com.amitia.runtime.linux.RootfsIntegrity(
                valid = false,
                missingFiles = listOf("bin/qdrant"),
                corruptedFiles = emptyList()
            )
        )
        coEvery { rootfsManager.install() } returns completedInstallFlow()
        coEvery { processManager.start(any(), any(), any(), any(), any(), any()) } returns Result.success(1234)
        coEvery { healthChecker.waitForHealthy(any(), any(), any(), any()) } returns Result.success(Unit)
        coEvery { stateMachine.transition(any()) } returns Result.success(RuntimeState.NotInstalled)
        every { stateMachine.emitLog(any(), any(), any()) } just Runs
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs
        coEvery { processManager.releaseAll() } returns Unit

        val result = bootstrap.repair()

        assertThat(result.isSuccess).isTrue()
        coVerify(atLeast = 1) { rootfsManager.install() }
        tempDir.deleteRecursively()
    }

    @Test
    fun start_uses_local_auth_token_for_backend_env() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-token")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        stubHealthyFlow()

        bootstrap.start(noopStage)

        coVerify {
            processManager.start(
                name = "amitia-backend",
                command = any(),
                env = match { env -> env.containsKey("LOCAL_AUTH_TOKEN") && env["LOCAL_AUTH_TOKEN"]!!.length == 32 },
                workDir = any(),
                restartPolicy = RestartPolicy.ON_FAILURE,
                timeoutMs = null
            )
        }
        tempDir.deleteRecursively()
    }

    @Test
    fun start_sets_restart_policy_ALWAYS_for_surrealdb_and_qdrant() = runTest {
        val tempDir = createTempDir(prefix = "bootstrap-policy")
        stubCommonDirs(tempDir)
        stubBinaries(tempDir)
        stubConfig(tempDir)
        stubHealthyFlow()

        bootstrap.start(noopStage)

        coVerify {
            processManager.start(
                name = "surrealdb",
                command = any(),
                env = any(),
                workDir = any(),
                restartPolicy = RestartPolicy.ALWAYS,
                timeoutMs = any()
            )
            processManager.start(
                name = "qdrant",
                command = any(),
                env = any(),
                workDir = any(),
                restartPolicy = RestartPolicy.ALWAYS,
                timeoutMs = any()
            )
        }
        tempDir.deleteRecursively()
    }
}

private fun createTempDir(prefix: String): File =
    java.nio.file.Files.createTempDirectory(prefix).toFile()
