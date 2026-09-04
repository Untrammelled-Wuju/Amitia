package com.amitia.amitia_app.nativeprovider.model

data class NativeBridgeRequest(
    val protocolVersion: Int,
    val requestId: String,
    val platform: String,
    val operation: String,
    val payload: Map<String, Any?> = emptyMap(),
)

data class NativeBridgeResponse(
    val protocolVersion: Int,
    val requestId: String,
    val status: String,
    val result: Map<String, Any?>? = null,
    val error: NativeBridgeError? = null,
)

data class NativeBridgeError(
    val code: String,
    val message: String,
    val domainCode: String? = null,
)

data class NativeBridgeHealth(
    val status: String,
    val platform: String = "android",
    val protocolVersion: Int = 1,
    val hostGeneration: Long = 0L,
    val foreground: Boolean = true,
    val capabilities: Map<String, Boolean> = emptyMap(),
)

object NativeBridgeProtocol {
    const val PROTOCOL_VERSION = 1
    const val PLATFORM_ANDROID = "android"
    const val STATUS_SUCCESS = "success"
    const val STATUS_ERROR = "error"
    const val HEALTH_READY = "ready"
    const val HEALTH_UNHEALTHY = "unhealthy"
    const val HEALTH_UNKNOWN = "unknown"

    const val ERR_INVALID_PLATFORM = "INVALID_PLATFORM"
    const val ERR_OPERATION_NOT_SUPPORTED = "OPERATION_NOT_SUPPORTED"
    const val ERR_INVALID_REQUEST = "INVALID_REQUEST"
    const val ERR_UNSUPPORTED_PROTOCOL = "UNSUPPORTED_PROTOCOL"
    const val ERR_INTERNAL = "INTERNAL_ERROR"
    const val ERR_TIMEOUT = "TIMEOUT"
    const val ERR_HOST_UNAVAILABLE = "HOST_UNAVAILABLE"
}
