package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class DefaultRuntimeBootstrapperTest {

    private class FakeManifestStore(
        private val result: RuntimeManifestResult,
    ) : RuntimeManifestStore {
        override fun read(): RuntimeManifestResult = result
        override fun write(manifest: RuntimeManifest): RuntimeManifestResult = result
        override fun delete(): RuntimeManifestResult = result
    }

    private class FakeActiveRuntimeManager(
        private val result: ActiveRuntimeResult,
    ) : ActiveRuntimeManager {
        override fun current(): ActiveRuntimeResult = result
        override fun activate(version: String): ActiveRuntimeResult = result
        override fun resolveActiveProgramRoot() = com.amitia.amitia_app.runtime.install.ActiveProgramRootResult.NoActiveRuntime
    }

    private class FakeHostLayout(
        private val existingVersions: Set<String> = emptySet(),
    ) : RuntimeHostLayout {
        override val controlRoot: File = File("/tmp/test")
        override val rootfsRoot: File = File("/tmp/test/rootfs")
        override val versionsRoot: File = File("/tmp/test/versions")
        override val stagingRoot: File = File("/tmp/test/staging")
        override val metadataRoot: File = File("/tmp/test/metadata")
        override val transactionsRoot: File = File("/tmp/test/transactions")
        override val locksRoot: File = File("/tmp/test/locks")
        override val packagesRoot: File = File("/tmp/test/packages")
        override val configRoot: File = File("/tmp/test/config")
        override val dataRoot: File = File("/tmp/test/data")
        override val cacheRoot: File = File("/tmp/test/cache")
        override val logRoot: File = File("/tmp/test/logs")
        override val runRoot: File = File("/tmp/test/run")
        override val homeRoot: File = File("/tmp/test/home")

        override fun runtimeVersionRoot(version: String): File {
            val dir = File(versionsRoot, version)
            if (version in existingVersions) {
                dir.mkdirs()
            }
            return dir
        }

        override fun installReceiptFile(version: String): File = File("/tmp/test/receipt-$version")
        override val activeRuntimeFile: File = File("/tmp/test/active-runtime.json")
        override val runtimeManifestFile: File = File("/tmp/test/runtime-manifest.json")
        override val runtimeManifestShaFile: File = File("/tmp/test/runtime-manifest.json.sha256")
    }

    private fun manifest(
        version: String = "1.0.0",
    ): RuntimeManifest = RuntimeManifest(
        schemaVersion = 1,
        runtimeVersion = version,
        sourceCommit = "a".repeat(40),
        packageId = "test-package",
        packageSha256 = "b".repeat(64),
        target = com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget(
            hostPlatform = "android",
            hostAbi = "arm64-v8a",
            runtimeKind = "proot",
            guestPlatform = "linux",
            guestArchitecture = "arm64",
            distribution = "ubuntu",
            distributionRelease = "22.04",
        ),
        installation = com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation(
            activeVersion = version,
            rootfsId = "rootfs-001",
            runtimeRootId = "runtime-001",
            runtimeRootTreeSha256 = "e".repeat(64),
        ),
        payloads = listOf(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload(
                id = "runtime",
                role = "runtime",
                sha256 = "d".repeat(64),
                size = 1000L,
            )
        ),
        components = listOf(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent(
                id = "runtime.backend",
                version = "1.0.0",
                architecture = "arm64",
                root = "backend",
                entry = "amitia-server",
                sha256 = "f".repeat(64),
                treeSha256 = null,
                source = "package",
            )
        ),
        paths = com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths(
            rootfsHostPath = "rootfs/default",
            runtimeRootHostPath = "versions/1.0.0",
            configHostPath = "config",
            dataHostPath = "data",
            cacheHostPath = "cache",
            logHostPath = "logs",
            runHostPath = "run",
            guestRuntimeRoot = "/amitia/runtime",
            guestConfigRoot = "/amitia/config",
            guestDataRoot = "/amitia/data",
            guestCacheRoot = "/amitia/cache",
            guestLogRoot = "/amitia/logs",
            guestRunRoot = "/amitia/run",
        ),
        verification = com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification(
            packageVerified = true,
            rootfsVerified = true,
            runtimeRootVerified = true,
            componentsVerified = true,
            guestLayoutVerified = true,
            mountContractVerified = true,
        )
    )

    private fun manifestNotFoundResult(): RuntimeManifestResult.Failure =
        RuntimeManifestResult.Failure(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                code = com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                manifestMessage = "manifest not found",
            )
        )

    @Test
    fun freshInstall_noManifestNoActive_returnsNotInstalled() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(manifestNotFoundResult()),
            activeRuntimeManager = FakeActiveRuntimeManager(ActiveRuntimeResult.NoActiveRuntime),
            hostLayout = FakeHostLayout(),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.NotInstalled)
    }

    @Test
    fun validInstalled_returnsInstalledStopped() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(RuntimeManifestResult.Success(manifest("1.0.0"))),
            activeRuntimeManager = FakeActiveRuntimeManager(
                ActiveRuntimeResult.Active(
                    com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                        version = "1.0.0",
                        activatedAtEpochMillis = 1000L,
                    )
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("1.0.0")),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.InstalledStopped)
        assertEquals("1.0.0", (result as RuntimeBootstrapResult.InstalledStopped).runtimeVersion)
    }

    @Test
    fun manifestWithoutActive_returnsFailed() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(RuntimeManifestResult.Success(manifest("1.0.0"))),
            activeRuntimeManager = FakeActiveRuntimeManager(ActiveRuntimeResult.NoActiveRuntime),
            hostLayout = FakeHostLayout(),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.Failed)
        assertEquals(
            RuntimeBootstrapErrorCode.MANIFEST_WITHOUT_ACTIVE,
            (result as RuntimeBootstrapResult.Failed).code
        )
    }

    @Test
    fun activeWithoutManifest_returnsFailed() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(manifestNotFoundResult()),
            activeRuntimeManager = FakeActiveRuntimeManager(
                ActiveRuntimeResult.Active(
                    com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                        version = "1.0.0",
                        activatedAtEpochMillis = 1000L,
                    )
                )
            ),
            hostLayout = FakeHostLayout(),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.Failed)
        assertEquals(
            RuntimeBootstrapErrorCode.ACTIVE_WITHOUT_MANIFEST,
            (result as RuntimeBootstrapResult.Failed).code
        )
    }

    @Test
    fun activeVersionMissing_returnsFailed() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(RuntimeManifestResult.Success(manifest("1.0.0"))),
            activeRuntimeManager = FakeActiveRuntimeManager(
                ActiveRuntimeResult.Active(
                    com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                        version = "1.0.0",
                        activatedAtEpochMillis = 1000L,
                    )
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = emptySet()),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.Failed)
        assertEquals(
            RuntimeBootstrapErrorCode.ACTIVE_VERSION_MISSING,
            (result as RuntimeBootstrapResult.Failed).code
        )
    }

    @Test
    fun versionMismatch_returnsFailed() {
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(RuntimeManifestResult.Success(manifest("1.0.0"))),
            activeRuntimeManager = FakeActiveRuntimeManager(
                ActiveRuntimeResult.Active(
                    com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                        version = "0.9.0",
                        activatedAtEpochMillis = 1000L,
                    )
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("0.9.0")),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.Failed)
        assertEquals(
            RuntimeBootstrapErrorCode.PACKAGE_IDENTITY_MISMATCH,
            (result as RuntimeBootstrapResult.Failed).code
        )
    }

    @Test
    fun bootstrap_doesNotIncrementGeneration() {
        val stateStore = RuntimeStateStore()
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(RuntimeManifestResult.Success(manifest("1.0.0"))),
            activeRuntimeManager = FakeActiveRuntimeManager(
                ActiveRuntimeResult.Active(
                    com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                        version = "1.0.0",
                        activatedAtEpochMillis = 1000L,
                    )
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("1.0.0")),
        )

        val result = bootstrapper.bootstrap()

        assertTrue(result is RuntimeBootstrapResult.InstalledStopped)
        assertEquals(0, stateStore.snapshot().generation)
    }

    @Test
    fun bootstrap_doesNotWriteFilesystem() {
        val tempDir = createTempDir("bootstrap-test")
        val manifestFile = File(tempDir, "runtime-manifest.json")
        val activeFile = File(tempDir, "active-runtime.json")

        assertFalse(manifestFile.exists())
        assertFalse(activeFile.exists())

        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(manifestNotFoundResult()),
            activeRuntimeManager = FakeActiveRuntimeManager(ActiveRuntimeResult.NoActiveRuntime),
            hostLayout = FakeHostLayout(),
        )

        bootstrapper.bootstrap()

        assertFalse(manifestFile.exists())
        assertFalse(activeFile.exists())

        tempDir.deleteRecursively()
    }
}
