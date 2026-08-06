package com.amitia.amitia_app.runtime.abi

sealed class RuntimeAbiStatus {
    data class Supported(
        val abi: String,
        val processIs64Bit: Boolean?,
        val snapshot: RuntimeAbiSnapshot
    ) : RuntimeAbiStatus()

    data class Unsupported(
        val reason: UnsupportedReason,
        val snapshot: RuntimeAbiSnapshot
    ) : RuntimeAbiStatus()

    data class DetectionFailed(
        val error: RuntimeAbiError,
        val snapshot: RuntimeAbiSnapshot
    ) : RuntimeAbiStatus()
}

enum class UnsupportedReason {
    SUPPORTED_ABIS_EMPTY,
    ARM64_ABI_MISSING,
    ARM64_64_BIT_ABI_MISSING,
    PROCESS_IS_32_BIT,
    ABI_DATA_INVALID
}