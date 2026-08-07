package com.amitia.amitia_app.runtime.manifest

internal data class RuntimeManifestInstallation(
    val activeVersion: String,
    val rootfsId: String,
    val runtimeRootId: String,
    val runtimeRootTreeSha256: String,
) {
    init {
        require(activeVersion.isNotBlank()) { "activeVersion must not be blank" }
        require(rootfsId.isNotBlank()) { "rootfsId must not be blank" }
        require(runtimeRootId.isNotBlank()) { "runtimeRootId must not be blank" }
        require(runtimeRootTreeSha256.trim().length == 64) { "runtimeRootTreeSha256 must be 64 hex chars" }
    }

    companion object {
        const val JSON_ACTIVE_VERSION: String = "activeVersion"
        const val JSON_ROOTFS_ID: String = "rootfsId"
        const val JSON_RUNTIME_ROOT_ID: String = "runtimeRootId"
        const val JSON_RUNTIME_ROOT_TREE_SHA256: String = "runtimeRootTreeSha256"
    }
}
