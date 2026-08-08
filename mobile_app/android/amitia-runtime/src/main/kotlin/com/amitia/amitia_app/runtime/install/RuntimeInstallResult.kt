package com.amitia.amitia_app.runtime.install

internal sealed interface RuntimeInstallResult {
    data class Success(
        val runtimeVersion: String,
        val packageSha256: String,
        val rootfsId: String,
        val rootfsPayloadSha256: String,
        val runtimePayloadSha256: String,
        val runtimeRootTreeSha256: String,
    ) : RuntimeInstallResult

    data class AlreadyInstalled(
        val runtimeVersion: String,
        val packageSha256: String,
    ) : RuntimeInstallResult

    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
        val phase: RuntimeInstallPhase,
        val transactionId: String? = null,
        val cause: Throwable? = null,
    ) : RuntimeInstallResult
}

internal enum class RuntimeInstallPhase {
    ABI_GATE,
    SPACE_CHECK,
    LOCK_ACQUIRE,
    RECOVERY,
    PACKAGE_VERIFY,
    ROOTFS_PREPARE,
    RUNTIME_EXTRACT,
    INSTALLED_VERIFY,
    PUBLISH,
    RECEIPT,
    ACTIVATE,
    CLEANUP,
}

internal enum class RuntimeInstallErrorCode {
    UNSUPPORTED_ABI,
    INSTALL_ALREADY_IN_PROGRESS,
    PACKAGE_NOT_FOUND,
    PACKAGE_READ_FAILED,
    PACKAGE_INVALID,
    PACKAGE_HASH_MISMATCH,
    PACKAGE_TARGET_MISMATCH,
    PACKAGE_VERSION_MISMATCH,
    ARCHIVE_INVALID,
    ARCHIVE_PATH_INVALID,
    ARCHIVE_ENTRY_DUPLICATE,
    ARCHIVE_ENTRY_UNSUPPORTED,
    ARCHIVE_TOO_LARGE,
    INSUFFICIENT_STORAGE,
    ROOTFS_CONFLICT,
    ROOTFS_INSTALL_FAILED,
    RUNTIME_VERSION_CONFLICT,
    RUNTIME_VERSION_INVALID,
    RUNTIME_EXTRACT_FAILED,
    RUNTIME_VERIFY_FAILED,
    RUNTIME_PUBLISH_FAILED,
    INSTALL_RECEIPT_WRITE_FAILED,
    ACTIVE_RUNTIME_UPDATE_FAILED,
    ACTIVE_RUNTIME_METADATA_INVALID,
    ACTIVE_RUNTIME_MISSING,
    ACTIVE_RUNTIME_WRITE_FAILED,
    INSTALL_RECOVERY_FAILED,
    LEGACY_INSTALL_INVALID,
    LEGACY_INSTALL_UNMIGRATABLE,
    IO_ERROR,
    INTERNAL_ERROR,
}
