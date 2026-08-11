package com.amitia.amitia_app.runtime.api

import com.amitia.amitia_app.runtime.install.RuntimeInstaller

interface RuntimeModule {
    val controller: RuntimeController
    val runtimeInstaller: RuntimeInstaller
    fun prootComponent(): Any
    fun close()
}
