package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.abi.UnsupportedReason
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification
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
        manifestStore: RuntimeManifestStore? = null,
        manifestBuilder: RuntimeManifestBuilder? = null,
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
            manifestStore = manifestStore ?: object : RuntimeManifestStore {
                override fun read(): RuntimeManifestResult {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                            "no manifest"
                        )
                    )
                }

                override fun write(manifest: RuntimeManifest): RuntimeManifestResult {
                    return RuntimeManifestResult.success(manifest)
                }

                override fun delete(): RuntimeManifestResult {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                            "delete not supported in test"
                        )
                    )
                }
            },
            manifestBuilder = manifestBuilder ?: object : RuntimeManifestBuilder {
                override fun build(): RuntimeManifestResult {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                            "builder not configured"
                        )
                    )
                }
            },
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

    @Test
    fun manifestCommitted_activationFailure_doesNotDeleteVersion() {
        val baseDir = tempFolder.newFolder("manifest-committed-activation-fail")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for activation failure test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val mockManifestStore = object : RuntimeManifestStore {
            var writtenManifest: RuntimeManifest? = null
            override fun read(): RuntimeManifestResult {
                val m = writtenManifest
                return if (m != null) RuntimeManifestResult.success(m)
                else RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_NOT_FOUND, "no manifest")
                )
            }
            override fun write(manifest: RuntimeManifest): RuntimeManifestResult {
                writtenManifest = manifest
                return RuntimeManifestResult.success(manifest)
            }
            override fun delete(): RuntimeManifestResult {
                writtenManifest = null
                return RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED, "deleted")
                )
            }
        }

        val mockActiveRuntimeManager = object : ActiveRuntimeManager {
            private var current: String? = null
            override fun current(): ActiveRuntimeResult {
                return if (current != null) ActiveRuntimeResult.Active(ActiveRuntimeInfo(current!!, 0L))
                else ActiveRuntimeResult.NoActiveRuntime
            }
            override fun activate(version: String): ActiveRuntimeResult {
                return ActiveRuntimeResult.Failure(
                    RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    "activation intentionally fails in test"
                )
            }
            override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                return ActiveProgramRootResult.NoActiveRuntime
            }
        }

        val fakeVerifier = createFakePackageVerifier(baseDir, "2.0.0")
        val installer = createInstaller(
            baseDir = baseDir,
            packageVerifier = fakeVerifier,
            activeRuntimeManager = mockActiveRuntimeManager,
            manifestStore = mockManifestStore,
        )

        val result = installer.install(RuntimeInstallRequest(packageFile = packageFile))

        assertTrue(result is RuntimeInstallResult.Failure)
        assertEquals(RuntimeInstallPhase.ACTIVATE, (result as RuntimeInstallResult.Failure).phase)

        val versionDir = layout.runtimeVersionRoot("2.0.0")
        assertTrue(
            "Version directory must NOT be deleted after manifest committed + activation failure",
            versionDir.exists()
        )
    }

    @Test
    fun manifestCommitted_activeReadBackFailure_doesNotDeleteVersion() {
        val baseDir = tempFolder.newFolder("manifest-committed-active-readback-fail")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for active read-back failure test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val mockManifestStore = object : RuntimeManifestStore {
            var writtenManifest: RuntimeManifest? = null
            override fun read(): RuntimeManifestResult {
                val m = writtenManifest
                return if (m != null) RuntimeManifestResult.success(m)
                else RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_NOT_FOUND, "no manifest")
                )
            }
            override fun write(manifest: RuntimeManifest): RuntimeManifestResult {
                writtenManifest = manifest
                return RuntimeManifestResult.success(manifest)
            }
            override fun delete(): RuntimeManifestResult {
                writtenManifest = null
                return RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED, "deleted")
                )
            }
        }

        val mockActiveRuntimeManager = object : ActiveRuntimeManager {
            override fun current(): ActiveRuntimeResult {
                return ActiveRuntimeResult.Failure(
                    RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    "active read-back intentionally fails in test"
                )
            }
            override fun activate(version: String): ActiveRuntimeResult {
                return ActiveRuntimeResult.Active(ActiveRuntimeInfo(version, 0L))
            }
            override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                return ActiveProgramRootResult.NoActiveRuntime
            }
        }

        val fakeVerifier = createFakePackageVerifier(baseDir, "3.0.0")
        val installer = createInstaller(
            baseDir = baseDir,
            packageVerifier = fakeVerifier,
            activeRuntimeManager = mockActiveRuntimeManager,
            manifestStore = mockManifestStore,
        )

        val result = installer.install(RuntimeInstallRequest(packageFile = packageFile))

        assertTrue(result is RuntimeInstallResult.Failure)
        assertEquals(RuntimeInstallPhase.ACTIVATE, (result as RuntimeInstallResult.Failure).phase)

        val versionDir = layout.runtimeVersionRoot("3.0.0")
        assertTrue(
            "Version directory must NOT be deleted after active read-back failure",
            versionDir.exists()
        )
    }

    @Test
    fun fullVerifyFailure_deletesNewlyPublishedVersion() {
        val baseDir = tempFolder.newFolder("full-verify-fail-cleanup")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for full verify failure test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val failingVerifier = object : InstalledRuntimeVerifier {
            override fun verify(dir: File): InstalledRuntimeVerificationResult {
                return InstalledRuntimeVerificationResult.Failure(
                    RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    "full verify intentionally fails in test"
                )
            }
        }

        val fakeVerifier = createFakePackageVerifier(baseDir, "4.0.0")
        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = fakeVerifier,
            archiveExtractor = DefaultSafeArchiveExtractor(),
            rootfsManager = DefaultRootfsManager(layout.controlRoot, DefaultSafeArchiveExtractor()),
            runtimeVerifier = failingVerifier,
            receiptStore = DefaultInstallReceiptStore(layout),
            activeRuntimeManager = object : ActiveRuntimeManager {
                override fun current(): ActiveRuntimeResult = ActiveRuntimeResult.NoActiveRuntime
                override fun activate(version: String): ActiveRuntimeResult {
                    return ActiveRuntimeResult.Active(ActiveRuntimeInfo(version, 0L))
                }
                override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                    return ActiveProgramRootResult.NoActiveRuntime
                }
            },
        )

        val result = installer.install(RuntimeInstallRequest(packageFile = packageFile))

        assertTrue(result is RuntimeInstallResult.Failure)
        assertEquals(RuntimeInstallPhase.INSTALLED_VERIFY, (result as RuntimeInstallResult.Failure).phase)

        val versionDir = layout.runtimeVersionRoot("4.0.0")
        assertFalse(
            "Version directory MUST be deleted on FULL Verify failure (PRE-COMMIT)",
            versionDir.exists()
        )
    }

    @Test
    fun existingVersion_neverDeletedOnFailure() {
        val baseDir = tempFolder.newFolder("existing-version-preserved")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for existing version test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val versionDir = layout.runtimeVersionRoot("5.0.0")
        versionDir.mkdirs()
        File(versionDir, "existing-file.txt").writeText("existing content")

        val mockManifestStore = object : RuntimeManifestStore {
            var writtenManifest: RuntimeManifest? = null
            override fun read(): RuntimeManifestResult {
                val m = writtenManifest
                return if (m != null) RuntimeManifestResult.success(m)
                else RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_NOT_FOUND, "no manifest")
                )
            }
            override fun write(manifest: RuntimeManifest): RuntimeManifestResult {
                writtenManifest = manifest
                return RuntimeManifestResult.success(manifest)
            }
            override fun delete(): RuntimeManifestResult {
                return RuntimeManifestResult.failure(
                    RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED, "deleted")
                )
            }
        }

        val failingActiveRuntimeManager = object : ActiveRuntimeManager {
            override fun current(): ActiveRuntimeResult = ActiveRuntimeResult.NoActiveRuntime
            override fun activate(version: String): ActiveRuntimeResult {
                return ActiveRuntimeResult.Failure(
                    RuntimeInstallErrorCode.ACTIVE_RUNTIME_UPDATE_FAILED,
                    "intentional failure"
                )
            }
            override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                return ActiveProgramRootResult.NoActiveRuntime
            }
        }

        val fakeVerifier = createFakePackageVerifier(baseDir, "5.0.0")
        val installer = createInstaller(
            baseDir = baseDir,
            packageVerifier = fakeVerifier,
            activeRuntimeManager = failingActiveRuntimeManager,
            manifestStore = mockManifestStore,
        )

        val result = installer.install(RuntimeInstallRequest(packageFile = packageFile))

        assertTrue(result is RuntimeInstallResult.Failure)

        assertTrue(
            "Existing version directory must NEVER be deleted by failed re-install",
            versionDir.exists()
        )
        assertTrue(
            "Existing version content must be preserved",
            File(versionDir, "existing-file.txt").exists()
        )
    }

    @Test
    fun stagingAlwaysCleanedOnPreCommitFailure() {
        val baseDir = tempFolder.newFolder("staging-cleaned-on-failure")
        val packageFile = File(baseDir, "dummy.zip")
        packageFile.writeText("dummy content for staging cleanup test")

        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))

        val failingVerifier = object : PackageVerifier {
            override fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult {
                return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_INVALID,
                    "intentional failure for staging cleanup test"
                )
            }
        }

        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = failingVerifier,
            archiveExtractor = DefaultSafeArchiveExtractor(),
            rootfsManager = DefaultRootfsManager(layout.controlRoot, DefaultSafeArchiveExtractor()),
            runtimeVerifier = com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier(treeHasher = { "" }),
            receiptStore = DefaultInstallReceiptStore(layout),
            activeRuntimeManager = object : ActiveRuntimeManager {
                override fun current(): ActiveRuntimeResult = ActiveRuntimeResult.NoActiveRuntime
                override fun activate(version: String): ActiveRuntimeResult {
                    return ActiveRuntimeResult.Active(ActiveRuntimeInfo(version, 0L))
                }
                override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
                    return ActiveProgramRootResult.NoActiveRuntime
                }
            },
        )

        val result = installer.install(RuntimeInstallRequest(packageFile = packageFile))

        assertTrue(result is RuntimeInstallResult.Failure)

        val stagingDir = layout.stagingRoot
        assertFalse(
            "Staging directory must be cleaned on PRE-COMMIT failure",
            stagingDir.exists() && stagingDir.listFiles()?.isNotEmpty() == true
        )
    }

    private fun createFakePackageVerifier(baseDir: File, runtimeVersion: String): PackageVerifier {
        return object : PackageVerifier {
            override fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult {
                val metadataDir = File(packageFile.parentFile, ".meta-tmp").apply { mkdirs() }
                return PackageVerificationResult.Success(
                    VerifiedPackage(
                        packageFile = packageFile,
                        packageSha256 = "test-package-sha256-abcdef1234567890abcdef1234567890abcdef123456",
                        packageIndex = PackageIndex(
                            runtimeVersion = runtimeVersion,
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
                            runtimeVersion = runtimeVersion,
                            packageId = "test-package",
                            components = listOf(
                                RuntimeManifestComponent(
                                    id = "test-component",
                                    version = runtimeVersion,
                                    architecture = "arm64-v8a",
                                    root = "/test",
                                    entry = null,
                                    sha256 = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
                                    treeSha256 = null,
                                    source = RuntimeManifestComponent.SOURCE_PACKAGE,
                                )
                            ),
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
    }
}
