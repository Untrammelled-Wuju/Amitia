package com.amitia.amitia_app.runtime.manifest

enum class RuntimeManifestErrorCode {
    MANIFEST_NOT_FOUND,
    MANIFEST_HASH_MISMATCH,
    MANIFEST_READ_FAILED,
    MANIFEST_WRITE_FAILED,
    MANIFEST_INVALID_JSON,
    UNSUPPORTED_MANIFEST_SCHEMA,

    MANIFEST_FIELD_INVALID,
    MANIFEST_TARGET_MISMATCH,
    MANIFEST_ACTIVE_VERSION_MISMATCH,

    MANIFEST_COMPONENT_DUPLICATE,
    MANIFEST_PAYLOAD_DUPLICATE,

    MANIFEST_PATH_INVALID,
    MANIFEST_PATH_OVERLAP,

    MANIFEST_PACKAGE_MISMATCH,
    INSTALL_RECEIPT_MISMATCH,

    MANIFEST_COMPONENT_MISSING,
    MANIFEST_COMPONENT_HASH_MISMATCH,
    MANIFEST_TREE_HASH_MISMATCH,

    ROOTFS_MISSING,
    RUNTIME_ROOT_MISSING,
    PROOT_COMPONENT_MISSING,

    GUEST_LAYOUT_MISMATCH,
    MOUNT_CONTRACT_MISMATCH,

    RUNTIME_ROOT_MODIFIED,

    LEGACY_METADATA_INVALID,
    LEGACY_METADATA_UNMIGRATABLE,

    INTERNAL_ERROR,
}

class RuntimeManifestError(
    val code: RuntimeManifestErrorCode,
    val manifestMessage: String,
    val componentId: String? = null,
    cause: Throwable? = null,
) : RuntimeException(manifestMessage, cause) {

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is RuntimeManifestError) return false
        return code == other.code && manifestMessage == other.manifestMessage && componentId == other.componentId
    }

    override fun hashCode(): Int {
        var result = code.hashCode()
        result = 31 * result + manifestMessage.hashCode()
        result = 31 * result + (componentId?.hashCode() ?: 0)
        return result
    }

    override fun toString(): String =
        "RuntimeManifestError(code=$code, message=$manifestMessage, componentId=$componentId)"
}
