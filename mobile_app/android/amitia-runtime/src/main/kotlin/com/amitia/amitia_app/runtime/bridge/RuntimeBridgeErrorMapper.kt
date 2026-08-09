package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeError

internal object RuntimeBridgeErrorMapper {

    fun mapToBridgeError(error: RuntimeError): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["code"] = mapErrorCode(error.code)
        result["message"] = sanitizeMessage(error.message)
        result["retryable"] = error.recoverable
        return result
    }

    fun mapToCommandResultError(error: RuntimeError): Map<String, Any?> {
        val result = LinkedHashMap<String, Any?>()
        result["accepted"] = false
        result["error"] = mapToBridgeError(error)
        return result
    }

    private fun mapErrorCode(code: RuntimeErrorCode): String {
        return when (code) {
            RuntimeErrorCode.NOT_IMPLEMENTED -> "NOT_IMPLEMENTED"
            RuntimeErrorCode.INVALID_REQUEST -> "INVALID_REQUEST"
            RuntimeErrorCode.INVALID_STATE -> "INVALID_STATE"
            RuntimeErrorCode.OPERATION_ALREADY_RUNNING -> "OPERATION_ALREADY_RUNNING"
            RuntimeErrorCode.OPERATION_CANCELLED -> "OPERATION_CANCELLED"
            RuntimeErrorCode.RUNTIME_NOT_INSTALLED -> "RUNTIME_NOT_INSTALLED"
            RuntimeErrorCode.RUNTIME_ALREADY_INSTALLED -> "RUNTIME_ALREADY_INSTALLED"
            RuntimeErrorCode.RUNTIME_CORRUPTED -> "RUNTIME_CORRUPTED"
            RuntimeErrorCode.PACKAGE_NOT_FOUND -> "PACKAGE_NOT_FOUND"
            RuntimeErrorCode.PACKAGE_INVALID -> "PACKAGE_INVALID"
            RuntimeErrorCode.MANIFEST_INVALID -> "MANIFEST_INVALID"
            RuntimeErrorCode.CHECKSUM_MISMATCH -> "CHECKSUM_MISMATCH"
            RuntimeErrorCode.STORAGE_UNAVAILABLE -> "STORAGE_UNAVAILABLE"
            RuntimeErrorCode.STORAGE_INSUFFICIENT -> "STORAGE_INSUFFICIENT"
            RuntimeErrorCode.UNSUPPORTED_PLATFORM -> "UNSUPPORTED_PLATFORM"
            RuntimeErrorCode.UNSUPPORTED_ABI -> "UNSUPPORTED_ABI"
            RuntimeErrorCode.PROOT_UNAVAILABLE -> "PROOT_UNAVAILABLE"
            RuntimeErrorCode.INSTALL_FAILED -> "INSTALL_FAILED"
            RuntimeErrorCode.VERIFY_FAILED -> "VERIFY_FAILED"
            RuntimeErrorCode.START_FAILED -> "START_FAILED"
            RuntimeErrorCode.STOP_FAILED -> "STOP_FAILED"
            RuntimeErrorCode.REPAIR_FAILED -> "REPAIR_FAILED"
            RuntimeErrorCode.TIMEOUT -> "TIMEOUT"
            RuntimeErrorCode.PERMISSION_DENIED -> "PERMISSION_DENIED"
            RuntimeErrorCode.INTERNAL_ERROR -> "INTERNAL_ERROR"
            RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE -> "RUNTIME_EXECUTION_NOT_AVAILABLE"
            RuntimeErrorCode.STARTUP_CANCELLED -> "STARTUP_CANCELLED"
            RuntimeErrorCode.STARTUP_GENERATION_STALE -> "STARTUP_GENERATION_STALE"
            RuntimeErrorCode.STARTUP_PROOT_NOT_RUNNING -> "STARTUP_PROOT_NOT_RUNNING"
            RuntimeErrorCode.STARTUP_PROOT_EXITED -> "STARTUP_PROOT_EXITED"
            RuntimeErrorCode.STARTUP_BACKEND_CONNECTION_REFUSED -> "STARTUP_BACKEND_CONNECTION_REFUSED"
            RuntimeErrorCode.STARTUP_BACKEND_LIVENESS_FAILED -> "STARTUP_BACKEND_LIVENESS_FAILED"
            RuntimeErrorCode.STARTUP_BACKEND_READINESS_FAILED -> "STARTUP_BACKEND_READINESS_FAILED"
            RuntimeErrorCode.STARTUP_HEALTH_AUTH_FAILED -> "STARTUP_HEALTH_AUTH_FAILED"
            RuntimeErrorCode.STARTUP_HEALTH_ENDPOINT_MISSING -> "STARTUP_HEALTH_ENDPOINT_MISSING"
            RuntimeErrorCode.STARTUP_TIMEOUT -> "STARTUP_TIMEOUT"
            RuntimeErrorCode.STARTUP_INVALID_ENDPOINT -> "STARTUP_INVALID_ENDPOINT"
        }
    }

    private fun sanitizeMessage(message: String): String {
        return message
            .replace(Regex("/data/user/\\d+/[^ \n]*"), "[redacted]")
            .replace(Regex("/data/data/[^ \n]*"), "[redacted]")
            .replace(Regex("noBackupFilesDir"), "[redacted]")
            .replace(Regex("filesDir"), "[redacted]")
            .replace(Regex("X-Amitia-Local-Token:[^\\s]*"), "X-Amitia-Local-Token:[redacted]")
            .replace(Regex("local-token=[^\\s]*"), "local-token=[redacted]")
    }
}
