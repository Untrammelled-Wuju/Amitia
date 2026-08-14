package com.amitia.amitia_app.runtime.packagetrusted

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
    const val PACKAGE_SHA256: String = "3f061598a5c0b815cdb1d536694d9e251652be13f301fb215f1d1aae0c5f7f57"
    const val GUEST_OS: String = "linux"
    const val ARCHITECTURE: String = "arm64"

    fun resolve(packageFile: File): RuntimePackageReference {
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
