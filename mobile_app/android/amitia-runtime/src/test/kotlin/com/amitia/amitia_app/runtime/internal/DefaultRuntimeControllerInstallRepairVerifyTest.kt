package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerificationResult
import com.amitia.amitia_app.runtime.install.InstalledRuntimeVerifier
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.install.RuntimeInstallResult
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.util.concurrent.atomic.AtomicReference

class DefaultRuntimeControllerInstallRepairVerifyTest {

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

    private class FakeInstaller(
        private val result: RuntimeInstallResult,
    ) : RuntimeInstaller {
        override fun install(request: RuntimeInstallRequest): RuntimeInstallResult = result
    }

    private class FakeInstalledVerifier(
        private val result: InstalledRuntimeVerificationResult,
    ) : InstalledRuntimeVerifier {
        override fun verify(runtimeRootDir: File): InstalledRuntimeVerificationResult = result
        override fun computeTreeSha256(rootDir: File): String = "fake-hash"
    }

    private fun manifestNotFoundResult(): RuntimeManifestResult.Failure =
        RuntimeManifestResult.Failure(
            com.amitia.amitia_app.runtime.manifest.RuntimeManifestError(
                code = com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                manifestMessage = "manifest not found",
            )
        )

    private fun createControllerWithInstaller(
        installer: RuntimeInstaller,
        installResult: RuntimeInstallResult,
    ): Pair<DefaultRuntimeController, AtomicReference<RuntimeOperationResult>> {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.NOT_INSTALLED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(installResult),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        return Pair(controller, callbackResult)
    }

    private fun executeInstallAndGetResult(
        controller: DefaultRuntimeController,
        callbackResult: AtomicReference<RuntimeOperationResult>,
    ) {
        controller.install(
            RuntimeInstallRequest(
                packageUri = "/tmp/test.7z",
                expectedVersion = "1.0.0",
                allowRepairExisting = false,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )
    }

    @Test
    fun install_failure_mapsToOperationFailure() {
        val (controller, callbackResult) = createControllerWithInstaller(
            installer = FakeInstaller(RuntimeInstallResult.Failure(
                code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.PACKAGE_INVALID,
                message = "invalid package",
                phase = com.amitia.amitia_app.runtime.install.RuntimeInstallPhase.PACKAGE_VERIFY,
            )),
            installResult = RuntimeInstallResult.Failure(
                code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.PACKAGE_INVALID,
                message = "invalid package",
                phase = com.amitia.amitia_app.runtime.install.RuntimeInstallPhase.PACKAGE_VERIFY,
            ),
        )

        executeInstallAndGetResult(controller, callbackResult)

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.INSTALL, result.type)
    }

    @Test
    fun install_failure_doesNotReturnSuccess() {
        val (controller, callbackResult) = createControllerWithInstaller(
            installer = FakeInstaller(RuntimeInstallResult.Failure(
                code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.IO_ERROR,
                message = "IO error",
                phase = com.amitia.amitia_app.runtime.install.RuntimeInstallPhase.RUNTIME_EXTRACT,
            )),
            installResult = RuntimeInstallResult.Failure(
                code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.IO_ERROR,
                message = "IO error",
                phase = com.amitia.amitia_app.runtime.install.RuntimeInstallPhase.RUNTIME_EXTRACT,
            ),
        )

        executeInstallAndGetResult(controller, callbackResult)

        assertFalse(callbackResult.get() is RuntimeOperationResult.Success)
    }

    @Test
    fun install_successWithoutBootstrapper_returnsSuccess() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.NOT_INSTALLED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.install(
            RuntimeInstallRequest(
                packageUri = "/tmp/test.7z",
                expectedVersion = "1.0.0",
                allowRepairExisting = false,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPED, result.snapshot.state)
    }

    @Test
    fun install_successWithBootstrapperRebootstrapFails_returnsFailure() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.NOT_INSTALLED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val bootstrapper = DefaultRuntimeBootstrapper(
            manifestStore = FakeManifestStore(manifestNotFoundResult()),
            activeRuntimeManager = FakeActiveRuntimeManager(ActiveRuntimeResult.NoActiveRuntime),
            hostLayout = FakeHostLayout(),
        )

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = bootstrapper,
            installedVerifier = null,
            hostLayout = FakeHostLayout(),
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.install(
            RuntimeInstallRequest(
                packageUri = "/tmp/test.7z",
                expectedVersion = "1.0.0",
                allowRepairExisting = false,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.INSTALL, result.type)
    }

    @Test
    fun install_success_doesNotMarkReady() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.NOT_INSTALLED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.install(
            RuntimeInstallRequest(
                packageUri = "/tmp/test.7z",
                expectedVersion = "1.0.0",
                allowRepairExisting = false,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun install_success_doesNotIncrementGeneration() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.NOT_INSTALLED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)
        val initialGeneration = stateStore.snapshot().generation

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.install(
            RuntimeInstallRequest(
                packageUri = "/tmp/test.7z",
                expectedVersion = "1.0.0",
                allowRepairExisting = false,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        assertEquals(initialGeneration, controller.snapshot().generation)
    }

    @Test
    fun repair_failure_mapsToOperationFailure() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.FAILED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Failure(
                code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.REPAIR_FAILED,
                message = "repair failed",
                phase = com.amitia.amitia_app.runtime.install.RuntimeInstallPhase.RUNTIME_EXTRACT,
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.repair(
            RuntimeRepairRequest(
                packageUri = "/tmp/test.7z",
                preserveUserData = true,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.REPAIR, result.type)
    }

    @Test
    fun repair_successWithoutPackageUri_returnsFailure() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.FAILED)
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.repair(
            RuntimeRepairRequest(
                packageUri = null,
                preserveUserData = true,
            ),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.REPAIR, result.type)
    }

    @Test
    fun verify_failure_mapsToOperationFailure() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.STOPPED, "1.0.0")
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = FakeInstalledVerifier(
                InstalledRuntimeVerificationResult.Failure(
                    code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    message = "verification failed",
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("1.0.0")),
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.verify(
            RuntimeVerifyRequest(deep = true),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.VERIFY, result.type)
    }

    @Test
    fun verify_failure_updatesStateToFailed() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.STOPPED, "1.0.0")
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = FakeInstalledVerifier(
                InstalledRuntimeVerificationResult.Failure(
                    code = com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode.RUNTIME_VERIFY_FAILED,
                    message = "verification failed",
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("1.0.0")),
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.verify(
            RuntimeVerifyRequest(deep = true),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun verify_success_mapsToOperationSuccess() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.STOPPED, "1.0.0")
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = FakeInstalledVerifier(
                InstalledRuntimeVerificationResult.Success(
                    com.amitia.amitia_app.runtime.install.InstalledRuntimeVerification(
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
                        runtimeRootTreeSha256 = "d".repeat(64),
                    )
                )
            ),
            hostLayout = FakeHostLayout(existingVersions = setOf("1.0.0")),
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.verify(
            RuntimeVerifyRequest(deep = true),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeOperationType.VERIFY, result.type)
    }

    @Test
    fun verify_noVerifier_returnsFailure() {
        val stateStore = RuntimeStateStore()
        stateStore.initialize(RuntimeState.STOPPED, "1.0.0")
        val host = FakeRuntimeServiceHost()
        val callbackResult = AtomicReference<RuntimeOperationResult>(null)

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            installer = FakeInstaller(RuntimeInstallResult.Success(
                runtimeVersion = "1.0.0",
                packageSha256 = "a".repeat(64),
                rootfsId = "rootfs-001",
                rootfsPayloadSha256 = "b".repeat(64),
                runtimePayloadSha256 = "c".repeat(64),
                runtimeRootTreeSha256 = "d".repeat(64),
            )),
            bootstrapper = null,
            installedVerifier = null,
            hostLayout = null,
            recoveryPolicy = noRecoveryPolicy,
            recoveryScheduler = immediateScheduler,
            installedRuntimeSource = installedSource,
        )

        controller.verify(
            RuntimeVerifyRequest(deep = true),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    callbackResult.set(result)
                }
            }
        )

        val result = callbackResult.get()
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeOperationType.VERIFY, result.type)
    }
}
