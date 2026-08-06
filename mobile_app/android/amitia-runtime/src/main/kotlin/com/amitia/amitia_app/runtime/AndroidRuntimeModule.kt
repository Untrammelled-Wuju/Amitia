package com.amitia.amitia_app.runtime

import android.content.Context
import com.amitia.amitia_app.runtime.abi.internal.BuildAndroidAbiProvider
import com.amitia.amitia_app.runtime.abi.internal.DefaultRuntimeAbiGate
import com.amitia.amitia_app.runtime.api.RuntimeModule
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeModule

object AndroidRuntimeModule {
    fun create(context: Context): RuntimeModule {
        val appContext = context.applicationContext
        val abiProvider = BuildAndroidAbiProvider()
        val abiGate = DefaultRuntimeAbiGate(provider = abiProvider)
        return DefaultRuntimeModule(abiGate = abiGate)
    }
}
