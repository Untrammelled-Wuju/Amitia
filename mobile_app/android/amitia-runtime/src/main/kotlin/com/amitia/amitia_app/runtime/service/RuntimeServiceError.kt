package com.amitia.amitia_app.runtime.service

enum class RuntimeServiceErrorCode {
    SERVICE_START_FAILED,
    SERVICE_BIND_FAILED,
    SERVICE_NOT_AVAILABLE,
    SERVICE_FOREGROUND_FAILED,
    SERVICE_NOTIFICATION_FAILED,
    SERVICE_ALREADY_ACTIVE,
    SERVICE_STOP_FAILED,
    SERVICE_TERMINATED,
    SERVICE_INTERNAL_ERROR
}

data class RuntimeServiceError(
    val code: RuntimeServiceErrorCode,
    override val message: String,
    override val cause: Throwable? = null
) : RuntimeException(message, cause)
