package com.amitia.amitia_app.runtime

import android.content.Context
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.install.RuntimeInstaller
import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeHostLayout
import com.amitia.amitia_app.runtime.install.internal.DefaultRuntimeInstaller
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.service.internal.AndroidRuntimeServiceHost
import java.io.File

object AndroidRuntimeModule {
    @Volatile private var cachedModule: RuntimeModule? = null

    fun create(context: Context): RuntimeModule {
        return cachedModule ?: synchronized(this) {
            cachedModule ?: createModule(context).also { cachedModule = it }
        }
    }

    private fun createModule(context: Context): RuntimeModule {
        val appContext = context.applicationContext
        val abiProvider = BuildAndroidAbiProvider()
        val abiGate = DefaultRuntimeAbiGate(provider = abiProvider)
        val stateStore = RuntimeStateStore()
        val serviceHost = AndroidRuntimeServiceHost(appContext)
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            abiGate = abiGate
        )
        val installer = createRuntimeInstaller(appContext)
        return DefaultRuntimeModule(
            controller = controller,
            runtimeInstaller = installer,
            stateStore = stateStore,
            abiGate = abiGate,
            serviceHost = serviceHost
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
        cachedModule = null
    }
}
