package com.amitia.amitia_app.runtime.manifest

data class RuntimeManifestVerification(
    val packageVerified: Boolean,
    val rootfsVerified: Boolean,
    val runtimeRootVerified: Boolean,
    val componentsVerified: Boolean,
    val guestLayoutVerified: Boolean,
    val mountContractVerified: Boolean,
) {
    internal fun allVerified(): Boolean =
        packageVerified && rootfsVerified && runtimeRootVerified &&
                componentsVerified && guestLayoutVerified && mountContractVerified

    companion object {
        const val JSON_PACKAGE_VERIFIED: String = "packageVerified"
        const val JSON_ROOTFS_VERIFIED: String = "rootfsVerified"
        const val JSON_RUNTIME_ROOT_VERIFIED: String = "runtimeRootVerified"
        const val JSON_COMPONENTS_VERIFIED: String = "componentsVerified"
        const val JSON_GUEST_LAYOUT_VERIFIED: String = "guestLayoutVerified"
        const val JSON_MOUNT_CONTRACT_VERIFIED: String = "mountContractVerified"
    }
}
