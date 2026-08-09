package com.amitia.amitia_app.runtime

import android.content.Context
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.service.internal.AndroidRuntimeServiceHost

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
        return DefaultRuntimeModule(
            controller = controller,
            stateStore = stateStore,
            abiGate = abiGate,
            serviceHost = serviceHost
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
