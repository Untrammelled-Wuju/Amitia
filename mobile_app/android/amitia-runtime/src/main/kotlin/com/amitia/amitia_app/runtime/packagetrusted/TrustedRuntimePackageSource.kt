package com.amitia.amitia_app.runtime.packagetrusted

import com.amitia.amitia_app.runtime.BuildConfig
import java.io.File

data class RuntimePackageReference(
    val packageFile: File,
    val expectedRuntimeVersion: String,
    val expectedPackageSha256: String,
    val expectedGuestOS: String,
    val expectedArchitecture: String,
)

object TrustedRuntimePackageSource {

    const val RUNTIME_VERSION: String = "1.0.0"
    val PACKAGE_SHA256: String = BuildConfig.RUNTIME_PACKAGE_SHA256
    const val GUEST_OS: String = "linux"
    const val ARCHITECTURE: String = "arm64"
    const val FILE_NAME: String = "amitia-runtime-1.0.0.zip"
    const val ASSET_PATH: String = "runtime-package/$FILE_NAME"

    internal fun createReference(packageFile: File): RuntimePackageReference {
        return RuntimePackageReference(
            packageFile = packageFile,
            expectedRuntimeVersion = RUNTIME_VERSION,
            expectedPackageSha256 = PACKAGE_SHA256,
            expectedGuestOS = GUEST_OS,
            expectedArchitecture = ARCHITECTURE,
        )
    }

    fun expectedRuntimeVersion(): String = RUNTIME_VERSION

    fun expectedPackageSha256(): String = PACKAGE_SHA256

    fun expectedGuestOS(): String = GUEST_OS

    fun expectedArchitecture(): String = ARCHITECTURE
}
