package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class PackageIndex(
    val runtimeVersion: String,
    val packageId: String,
    val target: PackageTarget,
    val guestLayout: PayloadRef,
    val mountContract: PayloadRef,
    val rootfsPayload: PayloadRef,
    val runtimePayload: PayloadRef,
    val sha256sums: PayloadRef,
    val licenses: PayloadRef?,
)

internal data class PackageTarget(
    val hostPlatform: String,
    val hostAbi: String,
    val runtimeKind: String,
    val guestPlatform: String,
    val guestArchitecture: String,
)

internal data class PayloadRef(
    val path: String,
    val sha256: String,
    val size: Long,
)

internal data class ComponentLock(
    val runtimeVersion: String,
    val packageId: String,
    val components: List<ComponentRef>,
)

internal data class ComponentRef(
    val id: String,
    val version: String?,
    val architecture: String?,
    val path: String,
    val sha256: String,
)

internal data class GuestLayout(
    val root: String,
    val directories: List<String>,
)

internal data class MountContract(
    val binds: List<BindMount>,
)

internal data class BindMount(
    val source: String,
    val target: String,
    val readOnly: Boolean,
)

internal data class VerifiedPackage(
    val packageFile: File,
    val packageSha256: String,
    val packageIndex: PackageIndex,
    val componentLock: ComponentLock,
    val guestLayout: GuestLayout,
    val mountContract: MountContract,
    val rootfsPayloadFile: File,
    val runtimePayloadFile: File,
    val sha256sumsFile: File,
    val metadataDir: File,
)

internal sealed interface PackageVerificationResult {
    data class Success(val package_: VerifiedPackage) : PackageVerificationResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : PackageVerificationResult
}

internal interface PackageVerifier {
    fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult
}
