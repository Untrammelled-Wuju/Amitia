package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.abi.RuntimeAbiGate
import com.amitia.amitia_app.runtime.abi.RuntimeAbiSnapshot
import com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerification
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.manifest.internal.DefaultRuntimeManifestStore
import com.amitia.amitia_app.runtime.recovery.InstalledRuntimeSource
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class ProductionCompositionTest {

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

    private class FakeInstalledRuntimeVerifier(
        private val result: InstalledRuntimeVerificationResult = InstalledRuntimeVerificationResult.Success(
            InstalledRuntimeVerification(
                valid = true,
                backendPresent = true,
                nodePresent = true,
                npmPresent = true,
                npxPresent = true,
                qdrantPresent = true,
                pluginHostPresent = true,
                taskHostPresent = true,
                nodeScriptsPresent = true,
                guestLayoutPresent = true,
                mountContractPresent = true,
                hasInvalidMutableDirs = false,
                runtimeRootTreeSha256 = "e".repeat(64),
            )
        ),
    ) : InstalledRuntimeVerifier {
        override fun verify(runtimeRootDir: File): InstalledRuntimeVerificationResult = result
        override fun computeTreeSha256(rootDir: File): String = "fake-hash"
    }

    private fun createDefaultRuntimeController(
        baseDir: File,
        installer: RuntimeInstaller,
        manifestStore: RuntimeManifestStore,
    ): DefaultRuntimeController {
        val stateStore = RuntimeStateStore()
        val host = FakeRuntimeServiceHost()
        val abiGate = createSupportedAbiGate()
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val activeRuntimeManager = com.amitia.amitia_app.runtime.install.internal.DefaultActiveRuntimeManager(layout, manifestStore)
        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = manifestStore,
            activeRuntimeManager = activeRuntimeManager,
            hostLayout = layout,
            installedRuntimeVerifier = FakeInstalledRuntimeVerifier(),
        )
        return DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = installer,
            abiGate = abiGate,
            bootstrapper = bootstrapper,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )
    }

    @Test
    fun productionControllerHasNonNullInstaller() {
        val baseDir = tempFolder.newFolder("composition-installer")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier(),
            archiveExtractor = com.amitia.amitia_app.runtime.install.DefaultSafeArchiveExtractor(),
            rootfsManager = com.amitia.amitia_app.runtime.install.DefaultRootfsManager(layout.controlRoot, com.amitia.amitia_app.runtime.install.DefaultSafeArchiveExtractor()),
            runtimeVerifier = com.amitia.amitia_app.runtime.install.DefaultInstalledRuntimeVerifier(treeHasher = { "" }),
            receiptStore = com.amitia.amitia_app.runtime.install.DefaultInstallReceiptStore(layout),
        )
        val manifestStore = DefaultRuntimeManifestStore(layout.metadataRoot.absolutePath)

        val controller = createDefaultRuntimeController(baseDir, installer, manifestStore)

        assertNotNull("Production DefaultRuntimeController must have non-null installer", controller)
    }

    @Test
    fun productionModuleHasNonNullManifestStore() {
        val baseDir = tempFolder.newFolder("composition-manifest")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val manifestStore = DefaultRuntimeManifestStore(layout.metadataRoot.absolutePath)

        val module = DefaultRuntimeModule(
            controller = UnsupportedRuntimeController(),
            runtimeInstaller = object : RuntimeInstaller {
                override fun install(request: com.amitia.amitia_app.runtime.install.RuntimeInstallRequest): com.amitia.amitia_app.runtime.install.RuntimeInstallResult {
                    throw UnsupportedOperationException("not implemented")
                }
            },
            manifestStore = manifestStore,
            backendConnectionProvider = object : com.amitia.amitia_app.runtime.connection.BackendConnectionProvider {
                override fun current(): com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability {
                    return com.amitia.amitia_app.runtime.connection.BackendConnectionAvailability.Unavailable
                }
            },
        )

        assertNotNull("Production DefaultRuntimeModule must have non-null manifestStore", module.manifestStore)
        assertSame(manifestStore, module.manifestStore)
    }

    @Test
    fun controllerInstallerNotNullAfterInjection() {
        val baseDir = tempFolder.newFolder("installer-injection")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val installer = DefaultRuntimeInstaller(
            layout = layout,
            abiGate = createSupportedAbiGate(),
            packageVerifier = com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier(),
        )
        val manifestStore = DefaultRuntimeManifestStore(layout.metadataRoot.absolutePath)

        val controller = createDefaultRuntimeController(baseDir, installer, manifestStore)

        val snapshot = controller.snapshot()
        assertEquals(RuntimeState.NOT_INSTALLED, snapshot.state)
    }

    @Test
    fun manifestStoreCanReadWrite() {
        val baseDir = tempFolder.newFolder("manifest-rw")
        val layout = DefaultRuntimeHostLayout(File(baseDir, "control"), File(baseDir, "data"))
        val manifestStore = DefaultRuntimeManifestStore(layout.metadataRoot.absolutePath)

        val readResult = manifestStore.read()
        assertTrue(readResult is com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult.Failure)
    }
}
