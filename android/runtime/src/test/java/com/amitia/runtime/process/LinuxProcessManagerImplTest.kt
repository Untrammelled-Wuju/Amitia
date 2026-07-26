package com.amitia.runtime.process

import android.content.Context
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.manager.RuntimeStateMachine
import com.google.common.truth.Truth.assertThat
import io.mockk.Runs
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Before
import org.junit.Test
import java.io.File
import java.nio.file.Files
import java.util.concurrent.TimeUnit

class LinuxProcessManagerImplTest {

    private lateinit var tempRoot: File
    private lateinit var context: Context
    private lateinit var stateMachine: RuntimeStateMachine
    private lateinit var logRotator: LogRotator
    private lateinit var crashDataStore: ProcessCrashDataStore
    private lateinit var manager: LinuxProcessManagerImpl

    @Before
    fun setUp() {
        tempRoot = Files.createTempDirectory("process-test").toFile()
        context = mockk(relaxed = true)
        stateMachine = mockk(relaxed = true)
        logRotator = LogRotator()
        crashDataStore = mockk(relaxed = true)
        every { context.filesDir } returns tempRoot
        every { stateMachine.emitLog(any(), any(), any()) } just Runs
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs
        coEvery { crashDataStore.load(any()) } returns ProcessCrashData()
        every { crashDataStore.cachedCrashCount(any()) } returns 0
        every { crashDataStore.cachedLastExitReason(any()) } returns null
        every { crashDataStore.cachedLastExitCode(any()) } returns null
        every { crashDataStore.cachedLastStartedAt(any()) } returns null
        manager = LinuxProcessManagerImpl(context, logRotator, stateMachine, crashDataStore)
    }

    @After
    fun tearDown() {
        runBlocking { manager.releaseAll() }
        tempRoot.deleteRecursively()
    }

    @Test
    fun start_returns_pid_for_simple_echo_command(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "echo hello") else listOf("echo", "hello")

        val result = manager.start(
            name = "echo-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )

        assertThat(result.isSuccess).isTrue()
        val pid = result.getOrNull()
        assertThat(pid).isNotNull()
        assertThat(pid!!).isGreaterThan(0)
        manager.stop("echo-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun start_writes_stdout_to_log_file(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "echo stdout-line") else listOf("echo", "stdout-line")

        manager.start(
            name = "log-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(500)

        val logs = manager.logs("log-test", tailLines = 50)
        assertThat(logs).isNotEmpty()
        assertThat(logs.joinToString("\n")).contains("stdout-line")
        manager.stop("log-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun status_returns_UNKNOWN_for_nonexistent_process() {
        val status = manager.status("does-not-exist")

        assertThat(status).isEqualTo(LinuxProcessManager.ProcessStatus.UNKNOWN)
    }

    @Test
    fun status_returns_RUNNING_for_alive_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "sleep-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(300)

        val status = manager.status("sleep-test")
        assertThat(status).isEqualTo(LinuxProcessManager.ProcessStatus.RUNNING)
        manager.stop("sleep-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun stop_terminates_running_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "stop-target",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(300)

        val result = manager.stop("stop-target", timeoutMs = 3000L)

        assertThat(result.isSuccess).isTrue()
        delay(200)
        assertThat(manager.status("stop-target")).isEqualTo(LinuxProcessManager.ProcessStatus.STOPPED)
    }

    @Test
    fun stop_returns_failure_for_nonexistent_process(): Unit = runBlocking {
        val result = manager.stop("no-such-process", timeoutMs = 1000L)

        assertThat(result.isFailure).isTrue()
    }

    @Test
    fun forceStop_kills_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "force-target",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(300)

        val result = manager.forceStop("force-target")

        assertThat(result.isSuccess).isTrue()
        delay(200)
        assertThat(manager.status("force-target")).isEqualTo(LinuxProcessManager.ProcessStatus.STOPPED)
        assertThat(manager.lastExitReason("force-target")).isEqualTo("force-stopped")
    }

    @Test
    fun start_rejects_duplicate_running_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "dup",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(200)

        val secondResult = manager.start(
            name = "dup",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )

        assertThat(secondResult.isFailure).isTrue()
        manager.stop("dup", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun crashCount_starts_at_zero() {
        assertThat(manager.crashCount("any")).isEqualTo(0)
    }

    @Test
    fun crashCount_increments_when_process_exits_non_zero(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "exit /b 1") else listOf("sh", "-c", "exit 1")

        manager.start(
            name = "crash-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(800)

        assertThat(manager.crashCount("crash-test")).isAtLeast(1)
        assertThat(manager.lastExitReason("crash-test")).isNotNull()
    }

    @Test
    fun lastStartedAt_returns_null_for_nonexistent_process() {
        assertThat(manager.lastStartedAt("does-not-exist")).isNull()
    }

    @Test
    fun lastStartedAt_returns_timestamp_for_existing_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "echo hello") else listOf("echo", "hello")

        val before = System.currentTimeMillis()
        manager.start(
            name = "ts-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )

        val startedAt = manager.lastStartedAt("ts-test")
        assertThat(startedAt).isNotNull()
        assertThat(startedAt!!).isAtLeast(before)
        manager.stop("ts-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun releaseAll_stops_all_managed_processes(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start("p1", command, emptyMap(), tempRoot, RestartPolicy.NEVER)
        manager.start("p2", command, emptyMap(), tempRoot, RestartPolicy.NEVER)
        delay(300)

        manager.releaseAll()
        delay(300)

        assertThat(manager.status("p1")).isEqualTo(LinuxProcessManager.ProcessStatus.STOPPED)
        assertThat(manager.status("p2")).isEqualTo(LinuxProcessManager.ProcessStatus.STOPPED)
    }

    @Test
    fun observeStatus_returns_UNKNOWN_for_nonexistent_process(): Unit = runBlocking {
        val first = manager.observeStatus("no-such").first()
        assertThat(first).isEqualTo(LinuxProcessManager.ProcessStatus.UNKNOWN)
    }

    @Test
    fun timeout_triggers_destroy_for_long_running_process(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "timeout-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER,
            timeoutMs = 500L
        )

        delay(1200)
        assertThat(manager.status("timeout-test")).isEqualTo(LinuxProcessManager.ProcessStatus.STOPPED)
    }

    @Test
    fun start_loads_persisted_crash_data_from_datastore(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "echo hello") else listOf("echo", "hello")
        val persisted = ProcessCrashData(
            crashCount = 5,
            lastExitReason = "previous-crash",
            lastExitCode = 137,
            lastStartedAt = 1000L
        )
        coEvery { crashDataStore.load("persisted-test") } returns persisted

        manager.start(
            name = "persisted-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )

        assertThat(manager.crashCount("persisted-test")).isEqualTo(5)
        assertThat(manager.lastExitReason("persisted-test")).isEqualTo("previous-crash")
        assertThat(manager.lastExitCode("persisted-test")).isEqualTo(137)
        manager.stop("persisted-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun start_persists_lastStartedAt_to_datastore(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "echo hello") else listOf("echo", "hello")

        manager.start(
            name = "started-at-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )

        coVerify {
            crashDataStore.save(
                name = "started-at-test",
                data = match { it.lastStartedAt != null && it.crashCount == 0 }
            )
        }
        manager.stop("started-at-test", timeoutMs = 3000L)
        Unit
    }

    @Test
    fun crash_persists_incremented_crash_count_to_datastore(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "exit /b 1") else listOf("sh", "-c", "exit 1")

        manager.start(
            name = "crash-persist-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(800)

        coVerify {
            crashDataStore.save(
                name = "crash-persist-test",
                data = match { it.crashCount >= 1 && it.lastExitCode == 1 }
            )
        }
    }

    @Test
    fun stop_persists_exit_data_to_datastore(): Unit = runBlocking {
        val isWindows = (System.getProperty("os.name") ?: "").lowercase().contains("windows")
        val command = if (isWindows) listOf("cmd.exe", "/c", "ping -n 30 127.0.0.1") else listOf("sleep", "30")

        manager.start(
            name = "stop-persist-test",
            command = command,
            env = emptyMap(),
            workDir = tempRoot,
            restartPolicy = RestartPolicy.NEVER
        )
        delay(300)
        manager.stop("stop-persist-test", timeoutMs = 3000L)

        coVerify {
            crashDataStore.save(
                name = "stop-persist-test",
                data = match { it.lastExitReason == "stopped" }
            )
        }
    }

    @Test
    fun crashCount_returns_cached_value_when_process_not_in_memory() {
        every { crashDataStore.cachedCrashCount("cached-process") } returns 7

        assertThat(manager.crashCount("cached-process")).isEqualTo(7)
    }

    @Test
    fun lastExitReason_returns_cached_value_when_process_not_in_memory() {
        every { crashDataStore.cachedLastExitReason("cached-reason") } returns "cached-failure"

        assertThat(manager.lastExitReason("cached-reason")).isEqualTo("cached-failure")
    }
}
