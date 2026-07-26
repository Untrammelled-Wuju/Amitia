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
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.After
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import java.io.ByteArrayInputStream
import java.io.File
import java.nio.file.Files

class LinuxRootfsManagerImplTest {

    private lateinit var tempRoot: File
    private lateinit var context: Context
    private lateinit var stateMachine: RuntimeStateMachine
    private lateinit var integrityChecker: RootfsIntegrityChecker
    private lateinit var manager: LinuxRootfsManagerImpl

    private val json = Json {
        prettyPrint = true
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    @Before
    fun setUp() {
        tempRoot = Files.createTempDirectory("rootfs-test").toFile()
        context = mockk(relaxed = true)
        stateMachine = mockk(relaxed = true)
        integrityChecker = spyk(RootfsIntegrityChecker())

        every { context.filesDir } returns tempRoot
        every { stateMachine.emitLog(any(), any(), any()) } just Runs
        every { stateMachine.emitError(any(), any(), any(), any()) } just Runs
        every { context.assets.openFd(any()) } throws java.io.IOException("no fd in test")

        manager = LinuxRootfsManagerImpl(context, stateMachine, integrityChecker)
    }

    @After
    fun tearDown() {
        tempRoot.deleteRecursively()
    }

    private fun sha256Hex(bytes: ByteArray): String {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        return digest.digest(bytes).joinToString("") { "%02x".format(it) }
    }

    private fun buildManifestJson(
        version: String,
        components: List<RootfsComponent>
    ): String {
        val manifest = RootfsManifest(
            version = version,
            createdAt = "2026-07-26",
            description = "test manifest",
            components = components,
            totalSize = components.sumOf { it.size },
            checksumAlgorithm = "SHA-256"
        )
        return json.encodeToString(RootfsManifest.serializer(), manifest)
    }

    private fun stubManifest(manifestJson: String) {
        every { context.assets.open("rootfs-manifest.json") } answers {
            ByteArrayInputStream(manifestJson.toByteArray())
        }
    }

    private fun stubAsset(name: String, bytes: ByteArray) {
        every { context.assets.open(name) } answers { ByteArrayInputStream(bytes) }
    }

    private fun makeComponent(name: String, content: ByteArray, type: String = "binary"): RootfsComponent {
        return RootfsComponent(
            name = name,
            file = name,
            size = content.size.toLong(),
            sha256 = sha256Hex(content),
            type = type,
            target = "linux/arm64"
        )
    }

    private fun stubComponents(
        components: List<RootfsComponent>,
        contents: Map<String, ByteArray>
    ) {
        components.forEach { c ->
            val content = contents[c.file] ?: ByteArray(c.size.toInt().coerceAtLeast(1)) { it.toByte() }
            stubAsset(c.file, content)
        }
    }

    @Test
    fun isInstalled_returns_false_when_rootfs_dir_missing() = runTest {
        assertThat(manager.isInstalled()).isFalse()
    }

    @Test
    fun isInstalled_returns_false_when_version_file_missing() = runTest {
        manager.rootfsDir().mkdirs()

        assertThat(manager.isInstalled()).isFalse()
    }

    @Test
    fun isInstalled_returns_true_after_successful_install() = runTest {
        val content = ByteArray(64) { it.toByte() }
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))

        val progresses = manager.install().toList()

        assertThat(progresses.last().phase).isEqualTo(RootfsInstallPhase.COMPLETED)
        assertThat(manager.isInstalled()).isTrue()
    }

    @Test
    fun install_creates_rootfs_dir_and_version_file() = runTest {
        val qdrantContent = ByteArray(32) { 0xAB.toByte() }
        val backendContent = ByteArray(32) { 0xCD.toByte() }
        val components = listOf(
            makeComponent("qdrant_linux_aarch64", qdrantContent),
            makeComponent("amitia-backend-arm64", backendContent)
        )
        stubManifest(buildManifestJson("2.0.0", components))
        stubComponents(
            components,
            mapOf(
                "qdrant_linux_aarch64" to qdrantContent,
                "amitia-backend-arm64" to backendContent
            )
        )

        manager.install().toList()

        assertThat(manager.rootfsDir().exists()).isTrue()
        assertThat(manager.versionFile().exists()).isTrue()
        assertThat(manager.getCurrentVersion()).isEqualTo("2.0.0")
    }

    @Test
    fun install_emits_progress_with_increasing_values() = runTest {
        val content = ByteArray(16) { 0.toByte() }
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("3.1.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))

        val progresses = manager.install().toList()
        val percents = progresses.map { it.percent }

        assertThat(percents).isNotEmpty()
        assertThat(percents.first()).isAtMost(percents.last())
        assertThat(percents.last()).isEqualTo(1f)
    }

    @Test
    fun install_replaces_existing_rootfs_directory() = runTest {
        val oldContent = ByteArray(8) { 1.toByte() }
        val oldComponent = makeComponent("qdrant_linux_aarch64", oldContent)
        stubManifest(buildManifestJson("1.0.0", listOf(oldComponent)))
        stubComponents(listOf(oldComponent), mapOf("qdrant_linux_aarch64" to oldContent))
        manager.install().toList()

        val staleFile = File(manager.rootfsDir(), "stale-file.txt")
        staleFile.writeText("old")

        val newContent = ByteArray(8) { 2.toByte() }
        val newComponent = makeComponent("qdrant_linux_aarch64", newContent)
        stubManifest(buildManifestJson("1.1.0", listOf(newComponent)))
        stubComponents(listOf(newComponent), mapOf("qdrant_linux_aarch64" to newContent))
        manager.install().toList()

        assertThat(staleFile.exists()).isFalse()
        assertThat(manager.getCurrentVersion()).isEqualTo("1.1.0")
    }

    @Test
    fun install_emits_log_on_success() = runTest {
        val content = ByteArray(8)
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))

        manager.install().toList()

        verify(atLeast = 1) {
            stateMachine.emitLog(
                RuntimeEvent.LogEmitted.Level.INFO,
                "RootfsManager",
                match { it.contains("RootFS 安装完成") }
            )
        }
    }

    @Test
    fun install_emits_error_on_failure() = runTest {
        every { context.assets.open("rootfs-manifest.json") } throws java.io.IOException("asset not found")

        val progresses = mutableListOf<RootfsInstallProgress>()
        val error = runCatching {
            manager.install().collect { progresses.add(it) }
        }

        assertThat(error.isFailure).isTrue()
        assertThat(progresses.last().phase).isEqualTo(RootfsInstallPhase.FAILED)
        verify(atLeast = 1) {
            stateMachine.emitError(
                error = match { it.contains("RootFS 安装失败") },
                retryable = true,
                requiresUserAction = false,
                cause = any()
            )
        }
    }

    @Test
    fun install_sets_executable_bit_for_binaries() = runTest {
        assumeTrue("可执行位检测仅在 Linux/macOS 有效", !isWindows())
        val content = ByteArray(8)
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))

        manager.install().toList()

        val binary = File(manager.binDir(), "qdrant_linux_aarch64")
        assertThat(binary.canExecute()).isTrue()
    }

    @Test
    fun verify_returns_invalid_when_manifest_missing() = runTest {
        manager.rootfsDir().mkdirs()
        every { context.assets.open("rootfs-manifest.json") } throws java.io.IOException("not found")

        val result = manager.verify()

        assertThat(result.isSuccess).isTrue()
        val integrity = result.getOrNull()
        assertThat(integrity).isNotNull()
        assertThat(integrity!!.valid).isFalse()
        assertThat(integrity.missingFiles).contains("rootfs-manifest.json")
    }

    @Test
    fun verify_returns_valid_after_install() = runTest {
        val content = ByteArray(8) { 0xAB.toByte() }
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))
        manager.install().toList()

        val result = manager.verify()

        assertThat(result.isSuccess).isTrue()
        assertThat(result.getOrNull()?.valid).isTrue()
    }

    @Test
    fun verify_detects_corrupted_files_after_install() = runTest {
        val content = ByteArray(8) { 0xAB.toByte() }
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))
        manager.install().toList()

        File(manager.binDir(), "qdrant_linux_aarch64").writeBytes(ByteArray(8) { 0xCD.toByte() })

        val result = manager.verify()

        assertThat(result.isSuccess).isTrue()
        val integrity = result.getOrNull()
        assertThat(integrity).isNotNull()
        assertThat(integrity!!.valid).isFalse()
        assertThat(integrity.corruptedFiles).contains("qdrant_linux_aarch64")
    }

    @Test
    fun upgrade_replaces_rootfs_atomically() = runTest {
        val oldContent = ByteArray(8) { 0x01.toByte() }
        val oldComponent = makeComponent("qdrant_linux_aarch64", oldContent)
        stubManifest(buildManifestJson("1.0.0", listOf(oldComponent)))
        stubComponents(listOf(oldComponent), mapOf("qdrant_linux_aarch64" to oldContent))
        manager.install().toList()

        val newContent = ByteArray(16) { 0x02.toByte() }
        val newComponent = makeComponent("qdrant_linux_aarch64", newContent)
        stubManifest(buildManifestJson("1.1.0", listOf(newComponent)))
        stubComponents(listOf(newComponent), mapOf("qdrant_linux_aarch64" to newContent))

        val progresses = manager.upgrade().toList()

        assertThat(progresses.last().phase).isEqualTo(RootfsInstallPhase.COMPLETED)
        assertThat(manager.getCurrentVersion()).isEqualTo("1.1.0")
    }

    @Test
    fun upgrade_fails_when_integrity_check_fails() = runTest {
        val oldContent = ByteArray(8)
        val oldComponent = makeComponent("qdrant_linux_aarch64", oldContent)
        stubManifest(buildManifestJson("1.0.0", listOf(oldComponent)))
        stubComponents(listOf(oldComponent), mapOf("qdrant_linux_aarch64" to oldContent))
        manager.install().toList()

        val newContent = ByteArray(8) { 0x02.toByte() }
        val wrongSha = "0000000000000000000000000000000000000000000000000000000000000000"
        val newComponent = RootfsComponent(
            name = "qdrant_linux_aarch64",
            file = "qdrant_linux_aarch64",
            size = newContent.size.toLong(),
            sha256 = wrongSha,
            type = "binary",
            target = "linux/arm64"
        )
        stubManifest(buildManifestJson("1.1.0", listOf(newComponent)))
        stubComponents(listOf(newComponent), mapOf("qdrant_linux_aarch64" to newContent))

        val progresses = mutableListOf<RootfsInstallProgress>()
        val error = runCatching {
            manager.upgrade().collect { progresses.add(it) }
        }

        assertThat(error.isFailure).isTrue()
        assertThat(progresses.last().phase).isEqualTo(RootfsInstallPhase.FAILED)
        verify(atLeast = 1) {
            stateMachine.emitError(
                error = match { it.contains("RootFS 升级失败") },
                retryable = true,
                requiresUserAction = false,
                cause = any()
            )
        }
    }

    @Test
    fun cleanup_with_requireConfirmation_returns_success_without_deleting() = runTest {
        val content = ByteArray(8)
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))
        manager.install().toList()

        val result = manager.cleanup(requireConfirmation = true)

        assertThat(result.isSuccess).isTrue()
        assertThat(manager.rootfsDir().exists()).isTrue()
    }

    @Test
    fun cleanup_without_confirmation_deletes_rootfs() = runTest {
        val content = ByteArray(8)
        val component = makeComponent("qdrant_linux_aarch64", content)
        stubManifest(buildManifestJson("1.0.0", listOf(component)))
        stubComponents(listOf(component), mapOf("qdrant_linux_aarch64" to content))
        manager.install().toList()

        val result = manager.cleanup(requireConfirmation = false)

        assertThat(result.isSuccess).isTrue()
        assertThat(manager.rootfsDir().exists()).isFalse()
    }

    @Test
    fun info_returns_null_when_not_installed() = runTest {
        assertThat(manager.info()).isNull()
    }

    @Test
    fun info_returns_version_size_and_count_when_installed() = runTest {
        val qdrantContent = ByteArray(64) { 0x01.toByte() }
        val surrealContent = ByteArray(32) { 0x02.toByte() }
        val components = listOf(
            makeComponent("qdrant_linux_aarch64", qdrantContent),
            makeComponent("surreal_linux_aarch64", surrealContent)
        )
        stubManifest(buildManifestJson("1.0.0", components))
        stubComponents(
            components,
            mapOf(
                "qdrant_linux_aarch64" to qdrantContent,
                "surreal_linux_aarch64" to surrealContent
            )
        )
        manager.install().toList()

        val info = manager.info()

        assertThat(info).isNotNull()
        assertThat(info!!.version).isEqualTo("1.0.0")
        assertThat(info.fileCount).isGreaterThan(0)
        assertThat(info.sizeBytes).isGreaterThan(0L)
    }

    @Test
    fun getCurrentVersion_returns_null_when_not_installed() = runTest {
        assertThat(manager.getCurrentVersion()).isNull()
    }

    private fun isWindows(): Boolean =
        (System.getProperty("os.name") ?: "").lowercase().contains("windows")
}
