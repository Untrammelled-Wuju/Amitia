package com.amitia.amitia_app.runtime.proot

class ProotError(val code: ProotErrorCode, val message: String, val detailsSource: Map<String, String>? = null) {
    val details: Map<String, String> = detailsSource?.toMap() ?: emptyMap()
    companion object {
        fun of(code: ProotErrorCode, message: String) = ProotError(code, message)
        fun of(code: ProotErrorCode, message: String, details: Map<String, String>) = ProotError(code, message, details)
    }
}

enum class ProotErrorCode {
    NOT_PACKAGED, METADATA_MISSING, METADATA_INVALID, BINARY_NOT_FOUND,
    BINARY_NOT_FILE, BINARY_NOT_READABLE, BINARY_NOT_EXECUTABLE,
    ELF_INVALID, ELF_CLASS_UNSUPPORTED,
    ELF_ENDIAN_UNSUPPORTED, ELF_ARCH_UNSUPPORTED, ELF_ENTRY_INVALID,
    CHECKSUM_MISMATCH, UNSUPPORTED_ABI, INVALID_REQUEST, INVALID_BIND,
    INVALID_ENVIRONMENT, INVALID_EXECUTABLE, PROCESS_START_FAILED, PROCESS_EXITED,
    PROCESS_STOP_FAILED, PROCESS_TIMEOUT, SESSION_CLOSED, SESSION_ALREADY_RUNNING,
    ROOTFS_MISSING, RUNTIME_ROOT_MISSING, PATH_TRAVERSAL_DETECTED,
    NUL_BYTE_DETECTED, INTERNAL_ERROR;

    companion object {
        val ALL = entries.toSet()
    }
}
