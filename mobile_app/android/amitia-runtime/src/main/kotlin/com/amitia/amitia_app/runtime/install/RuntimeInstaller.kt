package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class RuntimeInstallRequest(
    val packageFile: File,
    val expectedRuntimeVersion: String? = null,
)

internal interface RuntimeInstaller {
    fun install(request: RuntimeInstallRequest): RuntimeInstallResult
}
