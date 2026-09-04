package com.amitia.amitia_app.runtime.install

import java.io.File

data class RuntimeInstallRequest(
    val packageFile: File,
    val expectedRuntimeVersion: String? = null,
    val allowRepairExisting: Boolean = false,
)

interface RuntimeInstaller {
    fun install(request: RuntimeInstallRequest): RuntimeInstallResult
}
