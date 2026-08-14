package com.amitia.amitia_app.runtime

import android.content.Context
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.connection.internal.DefaultBackendConnectionProvider
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.install.internal.DefaultActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import com.amitia.amitia_app.runtime.manifest.internal.DefaultRuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.internal.DefaultRuntimeManifestStore
import com.amitia.amitia_app.runtime.packagetrusted.AndroidBundledRuntimePackageSource
import com.amitia.amitia_app.runtime.packagetrusted.RuntimePackageSource
import com.amitia.amitia_app.runtime.packagetrusted.TrustedRuntimePackageSource
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.internal.AndroidProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.internal.AndroidProotComponent
import com.amitia.amitia_app.runtime.proot.internal.AndroidProotRawResourceMetadataLoader
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotArtifactVerifier
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.internal.ProotMetadataLoaderInternal
import com.amitia.amitia_app.runtime.recovery.ActiveRuntimeBackedInstalledRuntimeSource
import com.amitia.amitia_app.runtime.recovery.DefaultRuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.ExecutorRuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.recovery.RuntimeCrashRecoveryPolicy
import com.amitia.amitia_app.runtime.recovery.RuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.service.internal.AndroidRuntimeServiceHost
import java.io.File

object AndroidRuntimeModule {
    @Volatile private var cachedModule: RuntimeModule? = null
    @Volatile private var cachedProotComponent: ProotComponent? = null
    @Volatile private var cachedProotEnvironmentAssembler: com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentAssembler? = null
    @Volatile private var cachedRuntimeHostLayout: com.amitia.amitia_app.runtime.install.RuntimeHostLayout? = null
    @Volatile private var cachedActiveRuntimeManager: ActiveRuntimeManager? = null
    @Volatile private var cachedRootfsPath: String? = null
    @Volatile private var cachedManifestStore: RuntimeManifestStore? = null
    @Volatile private var cachedRuntimeInstaller: RuntimeInstaller? = null
    @Volatile private var cachedRuntimePackageSource: RuntimePackageSource? = null

    val prootComponent: ProotComponent? get() = cachedProotComponent
    internal val prootEnvironmentAssembler: com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentAssembler? get() = cachedProotEnvironmentAssembler
    val runtimeHostLayout: com.amitia.amitia_app.runtime.install.RuntimeHostLayout? get() = cachedRuntimeHostLayout
    internal val activeRuntimeManager: ActiveRuntimeManager? get() = cachedActiveRuntimeManager
    val prootRootfsPath: String? get() = cachedRootfsPath
    internal val manifestStore: RuntimeManifestStore? get() = cachedManifestStore
    val runtimeInstaller: RuntimeInstaller? get() = cachedRuntimeInstaller
    val runtimePackageSource: RuntimePackageSource? get() = cachedRuntimePackageSource

    fun create(context: Context): RuntimeModule {
        return cachedModule ?: synchronized(this) {
            cachedModule ?: createModule(context).also { cachedModule = it }
        }
    }

    private fun createModule(context: Context): RuntimeModule {
        val appContext = context.applicationContext
        val layout = DefaultRuntimeHostLayout.fromContext(
            noBackupFilesDir = appContext.noBackupFilesDir ?: File(appContext.filesDir, "nobackup"),
            filesDir = appContext.filesDir,
        )
        val abiProvider = BuildAndroidAbiProvider()
        val abiGate = DefaultRuntimeAbiGate(provider = abiProvider)
        val stateStore = RuntimeStateStore()
        val serviceHost = AndroidRuntimeServiceHost(appContext)

        val prootComponent = createProotComponent(appContext)
        cachedProotComponent = prootComponent
        cachedRootfsPath = layout.rootfsRoot.absolutePath
        cachedRuntimeHostLayout = layout

        val runtimePackageSource = AndroidBundledRuntimePackageSource(
            context = appContext,
            layout = layout,
        )
        cachedRuntimePackageSource = runtimePackageSource

        val environmentBuilder = com.amitia.amitia_app.runtime.proot.internal.DefaultRuntimeEnvironmentBuilder()
        val prootEnvironmentAssembler = com.amitia.amitia_app.runtime.proot.internal.ProotEnvironmentAssembler(
            layout = layout,
            environmentBuilder = environmentBuilder,
        )
        cachedProotEnvironmentAssembler = prootEnvironmentAssembler

        val manifestStore = DefaultRuntimeManifestStore(layout.metadataRoot.absolutePath)
        cachedManifestStore = manifestStore

        val activeRuntimeManager = DefaultActiveRuntimeManager(layout, manifestStore)
        cachedActiveRuntimeManager = activeRuntimeManager
        val manifestBuilder = DefaultRuntimeManifestBuilder(
            layout = layout,
            abiStatus = abiGate.evaluate() as com.amitia.amitia_app.runtime.abi.RuntimeAbiStatus.Supported,
        )
        val installedRuntimeSource = ActiveRuntimeBackedInstalledRuntimeSource(activeRuntimeManager)
        val recoveryPolicy: RuntimeCrashRecoveryPolicy = DefaultRuntimeCrashRecoveryPolicy(
            installedRuntimeSource = installedRuntimeSource
        )
        val recoveryScheduler: RuntimeRecoveryScheduler = ExecutorRuntimeRecoveryScheduler()

        val installer = createRuntimeInstaller(
            layout = layout,
            abiGate = abiGate,
            manifestStore = manifestStore,
            manifestBuilder = manifestBuilder,
            activeRuntimeManager = activeRuntimeManager,
        )
        cachedRuntimeInstaller = installer

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            installer = installer,
            abiGate = abiGate,
            recoveryPolicy = recoveryPolicy,
            recoveryScheduler = recoveryScheduler,
            installedRuntimeSource = installedRuntimeSource,
        )

        val backendConnectionProvider = DefaultBackendConnectionProvider(
            snapshotProvider = { controller.snapshot() },
            dataRootProvider = { layout.dataRoot.absolutePath },
        )

        return DefaultRuntimeModule(
            controller = controller,
            runtimeInstaller = installer,
            manifestStore = manifestStore,
            backendConnectionProvider = backendConnectionProvider,
            stateStore = stateStore,
            abiGate = abiGate,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
        )
    }

    private fun createProotComponent(context: Context): ProotComponent {
        val rawResourceId = context.resources.getIdentifier("proot_artifact", "raw", context.packageName)
        val metadataLoader = if (rawResourceId != 0) {
            AndroidProotRawResourceMetadataLoader(context, rawResourceId)
        } else {
            object : ProotMetadataLoaderInternal {
                override fun load() = null
            }
        }
        val binaryLocator = AndroidProotBinaryLocator(context, metadataLoader)
        val artifactVerifier = DefaultProotArtifactVerifier(binaryLocator, null)
        return AndroidProotComponent(
            binaryLocator = binaryLocator,
            artifactVerifier = artifactVerifier,
            commandBuilder = DefaultProotCommandBuilder(),
            processLauncher = DefaultProotProcessLauncher(),
            abiGate = null,
        )
    }

    private fun createRuntimeInstaller(
        layout: com.amitia.amitia_app.runtime.install.RuntimeHostLayout,
        abiGate: com.amitia.amitia_app.runtime.abi.RuntimeAbiGate,
        manifestStore: RuntimeManifestStore,
        manifestBuilder: com.amitia.amitia_app.runtime.manifest.RuntimeManifestBuilder,
        activeRuntimeManager: ActiveRuntimeManager,
    ): RuntimeInstaller {
        return DefaultRuntimeInstaller(
            layout = layout,
            abiGate = abiGate,
            packageVerifier = DefaultPackageVerifier(),
            manifestStore = manifestStore,
            manifestBuilder = manifestBuilder,
            activeRuntimeManager = activeRuntimeManager,
        )
    }

    internal fun resetCacheForTest() {
        val module = cachedModule
        if (module is DefaultRuntimeModule) {
            module.close()
        }
        val proot = cachedProotComponent
        if (proot != null) {
            runCatching { proot.close() }
        }
        cachedModule = null
        cachedProotComponent = null
        cachedProotEnvironmentAssembler = null
        cachedRuntimeHostLayout = null
        cachedActiveRuntimeManager = null
        cachedRootfsPath = null
        cachedManifestStore = null
        cachedRuntimeInstaller = null
        cachedRuntimePackageSource = null
    }
}
