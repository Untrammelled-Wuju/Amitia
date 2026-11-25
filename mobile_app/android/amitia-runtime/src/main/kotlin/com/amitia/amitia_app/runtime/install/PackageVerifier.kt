package com.amitia.amitia_app.runtime.install

import java.io.File

internal data class PackageIndex(
    val runtimeVersion: String,
    val packageId: String,
    val sourceRevision: String,
    val target: PackageTarget,
    val guestLayout: PayloadRef,
    val mountContract: PayloadRef,
    val rootfsPayload: PayloadRef,
    val runtimePayload: PayloadRef?,
    val sha256sums: PayloadRef,
    val licenses: PayloadRef?,
    val packageFormatVersion: Int = 1,
    val components: List<ComponentRef> = emptyList(),
    val sharedPayload: ComponentPayloadRef? = null,
)

internal data class ComponentPayloadRef(
    val id: String,
    val path: String,
    val sha256: String,
    val size: Long,
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
    val payloadPath: String? = null,
    val payloadSha256: String? = null,
    val payloadSize: Long? = null,
)

internal data class ComponentPayload(
    val componentId: String,
    val path: String,
    val sha256: String,
    val size: Long,
    val file: File,
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
    val runtimePayloadFile: File?,
    val sha256sumsFile: File,
    val metadataDir: File,
    val packageFormatVersion: Int = 1,
    val componentPayloads: List<ComponentPayload> = emptyList(),
    val sharedPayload: ComponentPayload? = null,
) {
    val usesComponentPayloads: Boolean
        get() = packageFormatVersion >= 2
}

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
