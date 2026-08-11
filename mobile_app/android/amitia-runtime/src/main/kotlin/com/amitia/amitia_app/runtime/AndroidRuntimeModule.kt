package com.amitia.amitia_app.runtime

import android.content.Context
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.connection.internal.DefaultBackendConnectionProvider
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.internal.AndroidProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.internal.AndroidProotComponent
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotArtifactVerifier
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotCommandBuilder
import com.amitia.amitia_app.runtime.proot.internal.DefaultProotProcessLauncher
import com.amitia.amitia_app.runtime.proot.internal.ProotMetadataLoaderInternal
import com.amitia.amitia_app.runtime.service.internal.AndroidRuntimeServiceHost
import java.io.File

object AndroidRuntimeModule {
    @Volatile private var cachedModule: RuntimeModule? = null
    @Volatile private var cachedProotComponent: ProotComponent? = null
    @Volatile private var cachedRootfsPath: String? = null

    val prootComponent: ProotComponent? get() = cachedProotComponent
    val prootRootfsPath: String? get() = cachedRootfsPath

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

        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            abiGate = abiGate,
        )

        val backendConnectionProvider = DefaultBackendConnectionProvider(
            snapshotProvider = { controller.snapshot() },
            dataRootProvider = { layout.dataRoot.absolutePath },
        )

        val installer = createRuntimeInstaller(appContext)
        return DefaultRuntimeModule(
            controller = controller,
            runtimeInstaller = installer,
            backendConnectionProvider = backendConnectionProvider,
            stateStore = stateStore,
            abiGate = abiGate,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
        )
    }

    private fun createProotComponent(context: Context): ProotComponent {
        val metadataLoader = object : ProotMetadataLoaderInternal {
            override fun load() = null
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

    private fun createRuntimeInstaller(context: Context): RuntimeInstaller {
        val layout = DefaultRuntimeHostLayout.fromContext(
            noBackupFilesDir = context.noBackupFilesDir ?: File(context.filesDir, "nobackup"),
            filesDir = context.filesDir,
        )
        val abiProvider = BuildAndroidAbiProvider()
        val abiGate = DefaultRuntimeAbiGate(provider = abiProvider)
        return DefaultRuntimeInstaller(
            layout = layout,
            abiGate = abiGate,
            packageVerifier = DefaultPackageVerifier(),
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
        cachedRootfsPath = null
    }
}
