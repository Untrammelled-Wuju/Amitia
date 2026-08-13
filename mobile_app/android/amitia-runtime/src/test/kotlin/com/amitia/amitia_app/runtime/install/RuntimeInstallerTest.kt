package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.abi.UnsupportedReason
import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class RuntimeInstallerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun createSupportedAbiGate(): RuntimeAbiGate {
        return object : RuntimeAbiGate {
            override fun evaluate(): RuntimeAbiStatus {
                return RuntimeAbiStatus.Supported(
                    abi = "arm64-v8a",
                    processIs64Bit = true,
                    snapshot = RuntimeAbiSnapshot(
                        supportedAbis = listOf("arm64-v8a"),
                        supported64BitAbis = listOf("arm64-v8a"),
                        supported32BitAbis = emptyList(),
                        processIs64Bit = true,
                        osArchitecture = "aarch64",
                    )
                )
            }

            override fun isSupported(): Boolean = true
        }
    }

    private fun createUnsupportedAbiGate(): RuntimeAbiGate {
        return object : RuntimeAbiGate {
            override fun evaluate(): RuntimeAbiStatus {
                return RuntimeAbiStatus.Unsupported(
                    reason = UnsupportedReason.ARM64_ABI_MISSING,
                    snapshot = RuntimeAbiSnapshot(
                        supportedAbis = listOf("x86_64"),
                        supported64BitAbis = listOf("x86_64"),
                        supported32BitAbis = emptyList(),
                        processIs64Bit = true,
                        osArchitecture = "x86_64",
                    )
                )
            }

            override fun isSupported(): Boolean = false
        }
    }

    private fun createInstaller(
        baseDir: File,
        abiGate: RuntimeAbiGate = createSupportedAbiGate(),
        packageVerifier: PackageVerifier = DefaultPackageVerifier(),
        activeRuntimeManager: ActiveRuntimeManager? = null,
    ): DefaultRuntimeInstaller {
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        return DefaultRuntimeInstaller(
            layout = layout,
            abiGate = abiGate,
            packageVerifier = DefaultPackageVerifier(),
            archiveExtractor = DefaultSafeArchiveExtractor(),
            rootfsManager = DefaultRootfsManager(layout.controlRoot, DefaultSafeArchiveExtractor()),
            runtimeVerifier = com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier(treeHasher = { "" }),
            receiptStore = DefaultInstallReceiptStore(layout),
            activeRuntimeManager = activeRuntimeManager,
        )
    }

    @Test
    fun install_returnsUnsupportedAbi_whenDeviceNotArm64() {
        val baseDir = tempFolder.newFolder("install-unsupported-abi")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content")

        val installer = createInstaller(baseDir, createUnsupportedAbiGate())
        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)
        val failure = result as RuntimeInstallResult.Failure
        assertEquals(RuntimeInstallErrorCode.UNSUPPORTED_ABI, failure.code)
        assertEquals(RuntimeInstallPhase.ABI_GATE, failure.phase)
    }

    @Test
    fun install_returnsPackageNotFound_whenFileMissing() {
        val baseDir = tempFolder.newFolder("install-missing-file")
        val packageFile = File(baseDir, "nonexistent.zip")

        val installer = createInstaller(baseDir)
        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)
        val failure = result as RuntimeInstallResult.Failure
        assertEquals(RuntimeInstallErrorCode.PACKAGE_NOT_FOUND, failure.code)
    }

    @Test
    fun install_returnsPackageInvalid_whenZipMalformed() {
        val baseDir = tempFolder.newFolder("install-malformed-zip")
        val packageFile = File(baseDir, "malformed.zip")
        packageFile.writeText("this is not a zip file content that is long enough to be invalid")

        val installer = createInstaller(baseDir)
        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)
        val failure = result as RuntimeInstallResult.Failure
        assertEquals(RuntimeInstallErrorCode.PACKAGE_INVALID, failure.code)
    }

    @Test
    fun install_cleansStagingOnFailure() {
        val baseDir = tempFolder.newFolder("install-cleanup-staging")
        val packageFile = File(baseDir, "invalid.zip")
        packageFile.writeText("dummy invalid content for testing cleanup")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = object : PackageVerifier {
                override fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult {
                    return PackageVerificationResult.Failure(
                        RuntimeInstallErrorCode.PACKAGE_INVALID,
                        "test failure"
                    )
                }
            },
            archiveExtractor = DefaultSafeArchiveExtractor(),
            rootfsManager = DefaultRootfsManager(layout.controlRoot, DefaultSafeArchiveExtractor()),
            runtimeVerifier = com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier(treeHasher = { "" }),
            receiptStore = DefaultInstallReceiptStore(layout),
            activeRuntimeManager = null,
        )

        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)

        val stagingDir = layout.stagingRoot
        assertFalse(
            "Staging directory should not contain failed transaction artifacts",
            stagingDir.exists() && stagingDir.listFiles()?.isNotEmpty() == true
        )
    }

    @Test
    fun paths_returnsCorrectDirectoryStructure() {
        val baseDir = tempFolder.newFolder("paths-structure")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val up = { f: File -> f.absolutePath.replace('\\', '/') }
        assertTrue(up(layout.controlRoot).endsWith("amitia-runtime"))
        assertTrue(up(layout.rootfsRoot).endsWith("rootfs"))
        assertTrue(up(layout.versionsRoot).endsWith("versions"))
        assertTrue(up(layout.stagingRoot).endsWith("staging"))
        assertTrue(up(layout.metadataRoot).endsWith("metadata"))
        assertTrue(up(layout.transactionsRoot).endsWith("transactions"))
        assertTrue(up(layout.locksRoot).endsWith("locks"))
        assertTrue(up(layout.installReceiptFile("1.0.0")).endsWith("install-receipts/1.0.0.json"))
    }

    @Test
    fun alreadyInstalled_returnsConflict_whenVersionExistsWithDifferentContent() {
        val baseDir = tempFolder.newFolder("install-version-conflict")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for conflict test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val receiptStore = DefaultInstallReceiptStore(layout)
        receiptStore.save(
            RuntimeInstallReceipt(
                schemaVersion = 1,
                runtimeVersion = "1.0.0",
                packageId = "test",
                packageSha256 = "different-sha-that-does-not-match-the-temporary-test-file-1234567890abcdef",
                rootfsId = "rootfs-1",
                rootfsPayloadSha256 = "rootfs-sha-1",
                runtimePayloadSha256 = "runtime-sha-1",
                runtimeRootTreeSha256 = "tree-sha-1",
            )
        )

        val fakeVerifier = object : PackageVerifier {
            override fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult {
                val metadataDir = File(packageFile.parentFile, ".meta-tmp").apply { mkdirs() }
                return PackageVerificationResult.Success(
                    VerifiedPackage(
                        packageFile = packageFile,
                        packageSha256 = "test-package-sha256-abcdef1234567890abcdef1234567890abcdef123456",
                        packageIndex = PackageIndex(
                            runtimeVersion = "1.0.0",
                            packageId = "test-package",
                            target = PackageTarget("android", "arm64-v8a", "proot", "linux", "arm64"),
                            guestLayout = PayloadRef("guest.layout", "guest-layout-sha", 100),
                            mountContract = PayloadRef("mount.contract", "mount-contract-sha", 100),
                            rootfsPayload = PayloadRef("rootfs.tar", "rootfs-sha", 1000),
                            runtimePayload = PayloadRef("runtime.tar", "runtime-sha", 1000),
                            sha256sums = PayloadRef("sums.txt", "sums-sha", 50),
                            licenses = null,
                        ),
                        componentLock = ComponentLock(
                            runtimeVersion = "1.0.0",
                            packageId = "test-package",
                            components = emptyList(),
                        ),
                        guestLayout = GuestLayout("backend", emptyList()),
                        mountContract = MountContract(emptyList()),
                        rootfsPayloadFile = File(baseDir, "rootfs.tar").apply { writeText("rootfs") },
                        runtimePayloadFile = File(baseDir, "runtime.tar").apply { writeText("runtime") },
                        sha256sumsFile = File(baseDir, "sums.txt").apply { writeText("sums") },
                        metadataDir = metadataDir,
                    )
                )
            }
        }

        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = fakeVerifier,
            archiveExtractor = DefaultSafeArchiveExtractor(),
            rootfsManager = DefaultRootfsManager(layout.controlRoot, DefaultSafeArchiveExtractor()),
            runtimeVerifier = com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier(treeHasher = { "" }),
            receiptStore = receiptStore,
            activeRuntimeManager = null,
        )

        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)
        val failure = result as RuntimeInstallResult.Failure
        assertEquals(RuntimeInstallErrorCode.RUNTIME_VERSION_CONFLICT, failure.code)
    }

    @Test
    fun install_preservesOldActiveRuntime_onFailure() {
        val baseDir = tempFolder.newFolder("install-preserve-active")
        val packageFile = File(baseDir, "invalid.zip")
        packageFile.writeText("dummy content")

        val activeVersions = mutableListOf<String?>()
        val activeRuntimeManager = object : ActiveRuntimeManager {
            private var current: String? = "1.0.0"
            override fun current(): ActiveRuntimeResult {
                return if (current != null) {
                    ActiveRuntimeResult.Active(ActiveRuntimeInfo(current!!, 0L))
                } else {
                    ActiveRuntimeResult.NoActiveRuntime
                }
            }

            override fun activate(version: String): ActiveRuntimeResult {
                current = version
                return ActiveRuntimeResult.Active(ActiveRuntimeInfo(version, 0L))
            }

            override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                return ActiveProgramRootResult.NoActiveRuntime
            }
        }

        val installer = createInstaller(
            baseDir = baseDir,
            activeRuntimeManager = activeRuntimeManager
        )

        val result = installer.install(
            RuntimeInstallRequest(packageFile = packageFile)
        )

        assertTrue(result is RuntimeInstallResult.Failure)

        val currentAfterFailure = activeRuntimeManager.current()
        assertTrue(currentAfterFailure is ActiveRuntimeResult.Active)
        assertEquals("1.0.0", (currentAfterFailure as ActiveRuntimeResult.Active).info.version)
    }

    @Test
    fun receiptStore_persistAndLoad() {
        val baseDir = tempFolder.newFolder("receipt-store")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val store = DefaultInstallReceiptStore(layout)

        val receipt = RuntimeInstallReceipt(
            schemaVersion = 1,
            runtimeVersion = "1.0.0",
            packageId = "test-package",
            packageSha256 = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
            rootfsId = "rootfs-001",
            rootfsPayloadSha256 = "rootfs-sha-abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
            runtimePayloadSha256 = "runtime-sha-abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
            runtimeRootTreeSha256 = "tree-sha-abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
        )

        val saveResult = store.save(receipt)
        assertTrue(saveResult is InstallReceiptResult.Success)
        assertTrue(store.exists("1.0.0"))

        val loadResult = store.load("1.0.0")
        assertTrue(loadResult is InstallReceiptResult.Success)
        val loaded = (loadResult as InstallReceiptResult.Success).receipt
        assertEquals("1.0.0", loaded.runtimeVersion)
        assertEquals("test-package", loaded.packageId)
        assertEquals(receipt.packageSha256, loaded.packageSha256)
    }
}
