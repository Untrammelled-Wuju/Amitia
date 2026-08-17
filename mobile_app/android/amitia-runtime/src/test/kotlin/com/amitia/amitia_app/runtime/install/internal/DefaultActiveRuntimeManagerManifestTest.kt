package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.ActiveProgramRootResult
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.PathValidator
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class DefaultActiveRuntimeManagerManifestTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private lateinit var layout: FakeRuntimeHostLayoutWithManifest
    private lateinit var manifestStore: FakeManifestStore

    @Before
    fun setUp() {
        layout = FakeRuntimeHostLayoutWithManifest(
            tempFolder.newFolder("control"),
            tempFolder.newFolder("data"),
        )
        manifestStore = FakeManifestStore()
    }

    @Test
    fun activate_failsWhenManifestMissing() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "node/bin/node").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_failsWhenManifestVersionMismatch() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "node/bin/node").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "2.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_failsWhenBackendMissing() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "node/bin/node").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_failsWhenNodeMissing() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_failsWhenQdrantMissing() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "node/bin/node").mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
            (result as ActiveRuntimeResult.Failure).code
        )
    }

    @Test
    fun activate_succeedsWhenManifestMatches() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "node/bin/node").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("1.0.0")

        assertTrue(result is ActiveRuntimeResult.Active)
        assertEquals("1.0.0", (result as ActiveRuntimeResult.Active).info.version)
    }

    @Test
    fun resolveActiveProgramRoot_returnsNoActiveWhenNoMarker() {
        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.NoActiveRuntime)
    }

    @Test
    fun resolveActiveProgramRoot_returnsReadyWhenValid() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        manager.activate("1.0.0")

        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.Ready)
        val root = (result as ActiveProgramRootResult.Ready).root
        assertEquals("1.0.0", root.runtimeVersion)
        assertEquals(versionDir.absolutePath, root.hostDirectory.absolutePath)
    }

    @Test
    fun resolveActiveProgramRoot_failsWhenManifestMissing() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        manager.activate("1.0.0")

        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
            (result as ActiveProgramRootResult.Failure).code
        )
    }

    @Test
    fun resolveActiveProgramRoot_failsWhenManifestVersionMismatch() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        manifestStore.manifestVersion = "2.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        manager.activate("1.0.0")

        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.Failure)
        assertEquals(
            RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
            (result as ActiveProgramRootResult.Failure).code
        )
    }

    @Test
    fun resolveActiveProgramRoot_failsWhenSymlinkEscape() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        manager.activate("1.0.0")

        val outsideDir = tempFolder.newFolder("outside")
        val symlinkFile = File(layout.versionsRoot, "1.0.0")
        versionDir.deleteRecursively()
        java.nio.file.Files.createSymbolicLink(
            symlinkFile.toPath(),
            outsideDir.toPath(),
        )

        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.Failure)
    }

    @Test
    fun resolveActiveProgramRoot_failsWhenVersionsRootUsed() {
        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        layout.activeRuntimeFile.parentFile?.mkdirs()
        layout.activeRuntimeFile.writeText(
            """
            {
              "schemaVersion": 1,
              "runtimeVersion": "1.0.0"
            }
            """.trimIndent()
        )

        val versionsRootFile = layout.versionsRoot
        versionsRootFile.mkdirs()

        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.Failure)
    }

    @Test
    fun activate_doesNotScanVersions() {
        val version1 = File(layout.versionsRoot, "1.0.0")
        version1.mkdirs()
        val version2 = File(layout.versionsRoot, "2.0.0")
        version2.mkdirs()

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.resolveActiveProgramRoot()
        assertTrue(result is ActiveProgramRootResult.NoActiveRuntime)
    }

    @Test
    fun activate_idempotent() {
        val versionDir = File(layout.versionsRoot, "1.0.0")
        versionDir.mkdirs()
        File(versionDir, "backend/amitia-server").mkdirs()
        File(versionDir, "node/bin/node").mkdirs()
        File(versionDir, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "1.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result1 = manager.activate("1.0.0")
        val result2 = manager.activate("1.0.0")

        assertTrue(result1 is ActiveRuntimeResult.Active)
        assertTrue(result2 is ActiveRuntimeResult.Active)
    }

    @Test
    fun activate_versionSortingDoesNotAffectResult() {
        val version2 = File(layout.versionsRoot, "2.0.0")
        version2.mkdirs()
        File(version2, "backend/amitia-server").mkdirs()
        File(version2, "node/bin/node").mkdirs()
        File(version2, "qdrant/bin/qdrant").mkdirs()

        val version10 = File(layout.versionsRoot, "10.0.0")
        version10.mkdirs()
        File(version10, "backend/amitia-server").mkdirs()
        File(version10, "node/bin/node").mkdirs()
        File(version10, "qdrant/bin/qdrant").mkdirs()

        manifestStore.manifestVersion = "2.0.0"

        val manager = DefaultActiveRuntimeManager(layout, manifestStore)
        val result = manager.activate("2.0.0")

        assertTrue(result is ActiveRuntimeResult.Active)
        assertEquals("2.0.0", (result as ActiveRuntimeResult.Active).info.version)
    }
}

internal class FakeRuntimeHostLayoutWithManifest(
    override val controlRoot: File,
    private val dataBaseDir: File,
) : RuntimeHostLayout {

    private val dataDir = File(dataBaseDir, "amitia")

    override val rootfsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_ROOTFS)
    override val versionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_VERSIONS)
    override val stagingRoot: File = File(controlRoot, RuntimeHostLayout.DIR_STAGING)
    override val metadataRoot: File = File(controlRoot, RuntimeHostLayout.DIR_METADATA)
    override val transactionsRoot: File = File(controlRoot, RuntimeHostLayout.DIR_TRANSACTIONS)
    override val locksRoot: File = File(controlRoot, RuntimeHostLayout.DIR_LOCKS)

    override val configRoot: File = File(dataDir, RuntimeHostLayout.DIR_CONFIG)
    override val dataRoot: File = File(dataDir, RuntimeHostLayout.DIR_DATA)
    override val cacheRoot: File = File(dataDir, RuntimeHostLayout.DIR_CACHE)
    override val logRoot: File = File(dataDir, RuntimeHostLayout.DIR_LOGS)
    override val runRoot: File = File(dataDir, RuntimeHostLayout.DIR_RUN)
    override val homeRoot: File = File(dataDir, RuntimeHostLayout.DIR_HOME)

    override fun runtimeVersionRoot(version: String): File {
        if (!PathValidator.isValidRuntimeVersion(version)) {
            throw IllegalArgumentException("invalid version: $version")
        }
        return File(versionsRoot, version)
    }

    override fun installReceiptFile(version: String): File {
        return File(File(metadataRoot, RuntimeHostLayout.DIR_INSTALL_RECEIPTS), "$version.json")
    }

    override val activeRuntimeFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_ACTIVE_RUNTIME)

    override val runtimeManifestFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST)

    override val runtimeManifestShaFile: File
        get() = File(metadataRoot, RuntimeHostLayout.FILE_RUNTIME_MANIFEST_SHA)
}

internal class FakeManifestStore : RuntimeManifestStore {
    var manifestVersion: String? = null

    override fun read(): RuntimeManifestResult {
        val version = manifestVersion
        if (version == null) {
            return RuntimeManifestResult.failure(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                    "no manifest"
                )
            )
        }
        return RuntimeManifestResult.success(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifest(
                schemaVersion = 1,
                runtimeVersion = version,
                sourceCommit = "a".repeat(40),
                packageId = "test-package",
                packageSha256 = "a".repeat(64),
                target = com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget(
                    hostPlatform = "android",
                    hostAbi = "arm64-v8a",
                    runtimeKind = "embedded-proot",
                    guestPlatform = "linux",
                    guestArchitecture = "arm64",
                    distribution = "ubuntu",
                    distributionRelease = "24.04",
                ),
                installation = com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation(
                    activeVersion = version,
                    rootfsId = "ubuntu-24.04.4-arm64-r1",
                    runtimeRootId = "rt-1",
                    runtimeRootTreeSha256 = "a".repeat(64),
                ),
                payloads = listOf(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload(
                        id = "backend",
                        role = "runtime",
                        sha256 = "a".repeat(64),
                        size = 1000L,
                    ),
                ),
                components = listOf(
                    com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent(
                        id = "backend",
                        version = "1.0.0",
                        architecture = "arm64",
                        root = "backend",
                        entry = "amitia-server",
                        sha256 = "a".repeat(64),
                        treeSha256 = null,
                        source = "package",
                    ),
                ),
                paths = com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths(
                    rootfsHostPath = "/test/rootfs",
                    runtimeRootHostPath = "/test/runtime",
                    configHostPath = "/test/config",
                    dataHostPath = "/test/data",
                    cacheHostPath = "/test/cache",
                    logHostPath = "/test/log",
                    runHostPath = "/test/run",
                    guestRuntimeRoot = "/opt/amitia",
                    guestConfigRoot = "/etc/amitia",
                    guestDataRoot = "/var/lib/amitia",
                    guestCacheRoot = "/var/cache/amitia",
                    guestLogRoot = "/var/log/amitia",
                    guestRunRoot = "/run/amitia",
                ),
                verification = com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification(
                    packageVerified = true,
                    rootfsVerified = true,
                    runtimeRootVerified = true,
                    componentsVerified = true,
                    guestLayoutVerified = true,
                    mountContractVerified = true,
                ),
            )
        )
    }

    override fun write(manifest: com.amitia.amitia_app.runtime.manifest.RuntimeManifest): RuntimeManifestResult {
        return RuntimeManifestResult.failure(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                "not implemented"
            )
        )
    }

    override fun delete(): RuntimeManifestResult {
        return RuntimeManifestResult.failure(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                "not implemented"
            )
        )
    }
}
