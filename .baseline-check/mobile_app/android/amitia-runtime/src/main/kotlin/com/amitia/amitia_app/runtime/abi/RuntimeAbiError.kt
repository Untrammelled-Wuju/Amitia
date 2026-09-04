package com.amitia.amitia_app.runtime.abi

data class RuntimeAbiError(
    val code: RuntimeAbiErrorCode,
    val messageKey: String,
    val recoverable: Boolean
)

enum class RuntimeAbiErrorCode {
    ABI_PROVIDER_FAILED,
    ABI_LIST_EMPTY,
    ARM64_NOT_SUPPORTED,
    ARM64_PROCESS_UNAVAILABLE,
    ABI_DATA_INVALID
}