package com.amitia.amitia_app.runtime.api

enum class RuntimeErrorCode {
    NOT_IMPLEMENTED,
    INVALID_REQUEST,
    INVALID_STATE,
    OPERATION_ALREADY_RUNNING,
    OPERATION_CANCELLED,
    RUNTIME_NOT_INSTALLED,
    RUNTIME_ALREADY_INSTALLED,
    RUNTIME_CORRUPTED,
    PACKAGE_NOT_FOUND,
    PACKAGE_INVALID,
    MANIFEST_INVALID,
    CHECKSUM_MISMATCH,
    STORAGE_UNAVAILABLE,
    STORAGE_INSUFFICIENT,
    UNSUPPORTED_PLATFORM,
    UNSUPPORTED_ABI,
    PROOT_UNAVAILABLE,
    INSTALL_FAILED,
    VERIFY_FAILED,
    START_FAILED,
    STOP_FAILED,
    REPAIR_FAILED,
    TIMEOUT,
    PERMISSION_DENIED,
    INTERNAL_ERROR,
    RUNTIME_EXECUTION_NOT_AVAILABLE,
    STARTUP_CANCELLED,
    STARTUP_GENERATION_STALE,
    STARTUP_PROOT_NOT_RUNNING,
    STARTUP_PROOT_EXITED,
    STARTUP_BACKEND_CONNECTION_REFUSED,
    STARTUP_BACKEND_LIVENESS_FAILED,
    STARTUP_BACKEND_READINESS_FAILED,
    STARTUP_HEALTH_AUTH_FAILED,
    STARTUP_HEALTH_ENDPOINT_MISSING,
    STARTUP_TIMEOUT,
    STARTUP_INVALID_ENDPOINT,
    STOP_ALREADY_IN_PROGRESS,
    STOP_SESSION_MISSING,
    STOP_GRACEFUL_TIMEOUT,
    STOP_FORCE_FAILED,
    STOP_SERVICE_TEARDOWN_FAILED,
    NO_ACTIVE_RUNTIME,
    RECOVERY_EXHAUSTED
}

class RuntimeError(
    val code: RuntimeErrorCode,
    val message: String,
    val recoverable: Boolean,
    val componentId: String? = null,
    detailsSource: Map<String, String> = emptyMap()
) {
    val details: Map<String, String> = LinkedHashMap(detailsSource)

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is RuntimeError) return false
        return code == other.code &&
                message == other.message &&
                recoverable == other.recoverable &&
                componentId == other.componentId &&
                details == other.details
    }

    override fun hashCode(): Int {
        var result = code.hashCode()
        result = 31 * result + message.hashCode()
        result = 31 * result + recoverable.hashCode()
        result = 31 * result + (componentId?.hashCode() ?: 0)
        result = 31 * result + details.hashCode()
        return result
    }

    override fun toString(): String {
        return "RuntimeError(code=$code, message=$message, recoverable=$recoverable, componentId=$componentId, details=$details)"
    }
}
