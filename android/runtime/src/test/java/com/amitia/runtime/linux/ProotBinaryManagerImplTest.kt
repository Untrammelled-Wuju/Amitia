package com.amitia.runtime.linux

import android.content.Context
import com.amitia.runtime.api.RuntimeEvent
import com.amitia.runtime.manager.RuntimeStateMachine
import com.google.common.truth.Truth.assertThat
import io.mockk.Runs
import io.mockk.every
import io.mockk.just
import io.mockk.mockk
import io.mockk.spyk
import io.mockk.verify
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Before
import org.junit.Test
import java.io.ByteArrayInputStream
import java.io.File
import java.nio.file.Files

class ProotBinaryManagerImplTest {

    private lateinit var tempRoot: File
    private lateinit var context: Context
    private lateinit var stateMachine: RuntimeStateMachine
    private lateinit var integrityChecker: RootfsIntegrityChecker
    private lateinit var manager: ProotBinaryManagerImpl

    @Before
    fun setUp() {
        tempRoot = Files.createTempDirectory("proot-binary-test").toFile()
        context = mockk(relaxed = true)
        stateMachine = mockk(relaxed = true)
        integrityChecker = spyk(RootfsIntegrityChecker())

        every { context.filesDir } returns tempRoot
        val appInfo = android.content.pm.ApplicationInfo()
        appInfo.nativeLibraryDir = tempRoot.absolutePath
        every { context.applicationInfo } returns appInfo
        every { stateMachine.emitLog(any(), any(), any()) } just Runs
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs
        every { context.assets.openFd(any()) } throws java.io.IOException("no fd in test")

        manager = ProotBinaryManagerImpl(context, stateMachine, integrityChecker)
    }

    @After
    fun tearDown() {
        tempRoot.deleteRecursively()
    }

    private fun sha256Hex(bytes: ByteArray): String {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        return digest.digest(bytes).joinToString("") { "%02x".format(it) }
    }

    private fun stubAsset(name: String, bytes: ByteArray) {
        every { context.assets.open(name) } answers { ByteArrayInputStream(bytes) }
    }

    private fun stubAssetMissing(name: String) {
        every { context.assets.open(name) } throws java.io.IOException("asset $name not found")
    }

    private fun stubNativeExecAvailable() {
        val nativeExecFile = File(tempRoot, "libproot_exec.so")
        nativeExecFile.writeBytes(byteArrayOf(0))
        nativeExecFile.setExecutable(true, false)
    }

    private fun stubSha256Asset(sha256: String) {
        stubAsset(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME, sha256.toByteArray())
    }

    private suspend fun collectInstall(): MutableList<ProotInstallProgress> {
        val progresses = mutableListOf<ProotInstallProgress>()
        runCatching { manager.install().collect { progresses.add(it) } }
        return progresses
    }

    @Test
    fun isAvailable_returns_false_when_binary_not_installed() {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)
        assertThat(manager.isAvailable()).isFalse()
    }

    @Test
    fun isAvailable_returns_true_after_install_with_matching_sha256() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        val sha = sha256Hex(binaryContent)
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubSha256Asset(sha)

        collectInstall()

        assertThat(manager.isAvailable()).isTrue()
    }

    @Test
    fun isAvailable_returns_false_after_install_with_mismatched_sha256() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubSha256Asset("0000000000000000000000000000000000000000000000000000000000000000")

        collectInstall()

        assertThat(manager.isAvailable()).isFalse()
    }

    @Test
    fun isAvailable_returns_true_when_sha256_asset_uses_sha256sum_format_with_filename() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        val sha = sha256Hex(binaryContent)
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubSha256Asset("$sha  proot_linux_aarch64")

        collectInstall()

        assertThat(manager.isAvailable()).isTrue()
    }

    @Test
    fun isAvailable_returns_true_when_sha256_asset_uses_sha256sum_binary_format_with_star() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        val sha = sha256Hex(binaryContent)
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubSha256Asset("$sha *proot_linux_aarch64")

        collectInstall()

        assertThat(manager.isAvailable()).isTrue()
    }

    @Test
    fun isAvailable_returns_true_when_no_sha256_asset_present() = runTest {
        val binaryContent = "no-sha-proot".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        assertThat(manager.isAvailable()).isTrue()
    }

    @Test
    fun binaryPath_returns_null_when_not_available() {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)
        assertThat(manager.binaryPath()).isNull()
    }

    @Test
    fun binaryPath_returns_file_path_when_available() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        val path = manager.binaryPath()
        assertThat(path).isNotNull()
        assertThat(path!!.exists()).isTrue()
        assertThat(path.canExecute()).isTrue()
        assertThat(path.name).isEqualTo("proot")
    }

    @Test
    fun version_returns_null_when_not_installed() {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)
        assertThat(manager.version()).isNull()
    }

    @Test
    fun version_returns_default_version_after_install() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        assertThat(manager.version()).isEqualTo(ProotBinaryManagerImpl.PROOT_DEFAULT_VERSION)
    }

    @Test
    fun verify_returns_failure_when_asset_not_present() {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)
        val result = manager.verify()
        assertThat(result.isFailure).isTrue()
        val err = result.exceptionOrNull()?.message ?: ""
        assertThat(err).contains("PRoot 二进制未预置")
    }

    @Test
    fun verify_returns_success_after_install() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()
        stubNativeExecAvailable()

        val result = manager.verify()
        assertThat(result.isSuccess).isTrue()
    }

    @Test
    fun unavailableReason_returns_asset_missing_message_when_no_asset() {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)
        val reason = manager.unavailableReason()
        assertThat(reason).isNotNull()
        assertThat(reason!!).contains("PRoot 二进制未预置")
        assertThat(reason).contains("proot-rs")
    }

    @Test
    fun unavailableReason_returns_not_installed_message_when_asset_present_but_binary_missing() {
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, "dummy".toByteArray())
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)
        val reason = manager.unavailableReason()
        assertThat(reason).isNotNull()
        assertThat(reason!!).contains("尚未安装")
    }

    @Test
    fun unavailableReason_returns_null_when_available() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()
        stubNativeExecAvailable()

        assertThat(manager.unavailableReason()).isNull()
    }

    @Test
    fun install_emits_started_and_completed_phases_when_successful() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        val progress = collectInstall()

        assertThat(progress).isNotEmpty()
        assertThat(progress.first().phase).isEqualTo(ProotInstallPhase.STARTED)
        assertThat(progress.last().phase).isEqualTo(ProotInstallPhase.COMPLETED)
        assertThat(progress.last().percent).isEqualTo(1f)
    }

    @Test
    fun install_emits_copying_phase_with_bytes_copied_message() = runTest {
        val binaryContent = ByteArray(200) { it.toByte() }
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        val progress = collectInstall()

        val copyingPhases = progress.filter { it.phase == ProotInstallPhase.COPYING }
        assertThat(copyingPhases).isNotEmpty()
        assertThat(copyingPhases.first().message).contains("复制 PRoot")
    }

    @Test
    fun install_emits_failed_phase_when_asset_missing() = runTest {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)

        val progress = collectInstall()

        val failedPhases = progress.filter { it.phase == ProotInstallPhase.FAILED }
        assertThat(failedPhases).isNotEmpty()
        assertThat(failedPhases.last().error).contains("PRoot 二进制未预置")
    }

    @Test
    fun install_emits_failed_phase_when_sha256_mismatches() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubSha256Asset("deadbeef")

        val progress = collectInstall()

        val failedPhases = progress.filter { it.phase == ProotInstallPhase.FAILED }
        assertThat(failedPhases).isNotEmpty()
        assertThat(failedPhases.last().error ?: "").contains("SHA-256")
    }

    @Test
    fun install_sets_executable_permission_on_target_binary() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        val target = File(tempRoot, "runtime/bin/${ProotBinaryManagerImpl.PROOT_BINARY_NAME}")
        assertThat(target.exists()).isTrue()
        assertThat(target.canExecute()).isTrue()
    }

    @Test
    fun install_writes_version_file_after_success() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        val versionFile = File(tempRoot, "runtime/bin/${ProotBinaryManagerImpl.PROOT_VERSION_FILE_NAME}")
        assertThat(versionFile.exists()).isTrue()
        assertThat(versionFile.readText()).isEqualTo(ProotBinaryManagerImpl.PROOT_DEFAULT_VERSION)
    }

    @Test
    fun install_emits_error_to_state_machine_on_failure() = runTest {
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_ASSET_NAME)

        collectInstall()

        verify(atLeast = 1) {
            stateMachine.emitError(
                error = match { it.contains("PRoot") },
                retryable = any(),
                requiresUserAction = any(),
                cause = any()
            )
        }
    }

    @Test
    fun install_logs_info_to_state_machine_on_success() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()

        verify(atLeast = 1) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                "ProotBinaryManager",
                match { it.contains("PRoot 二进制安装完成") }
            )
        }
    }

    @Test
    fun install_can_be_repeated_idempotently() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        collectInstall()
        val firstVersion = manager.version()
        assertThat(firstVersion).isNotNull()

        collectInstall()
        val secondVersion = manager.version()
        assertThat(secondVersion).isEqualTo(firstVersion)
        assertThat(manager.isAvailable()).isTrue()
    }

    @Test
    fun install_emits_started_phase_with_zero_percent() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        val progress = collectInstall()

        val started = progress.first()
        assertThat(started.phase).isEqualTo(ProotInstallPhase.STARTED)
        assertThat(started.percent).isEqualTo(0f)
        assertThat(started.message).contains("开始安装 PRoot")
    }

    @Test
    fun install_emits_verify_phase_before_completion() = runTest {
        val binaryContent = "fake-proot-binary-content".toByteArray()
        stubAsset(ProotBinaryManagerImpl.PROOT_ASSET_NAME, binaryContent)
        stubAssetMissing(ProotBinaryManagerImpl.PROOT_SHA256_ASSET_NAME)

        val progress = collectInstall()

        val verifyPhases = progress.filter { it.phase == ProotInstallPhase.VERIFYING }
        assertThat(verifyPhases).isNotEmpty()
        assertThat(verifyPhases.first().message).contains("校验")
        assertThat(verifyPhases.first().percent).isAtLeast(0.8f)
    }
}
