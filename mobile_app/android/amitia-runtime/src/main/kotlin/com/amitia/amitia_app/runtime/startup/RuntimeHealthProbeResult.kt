package com.amitia.amitia_app.runtime.startup

internal sealed interface RuntimeHealthProbeResult {
    data class Success(val statusCode: Int) : RuntimeHealthProbeResult
    data class Failure(val error: RuntimeHealthProbeError) : RuntimeHealthProbeResult
}

internal sealed class RuntimeHealthProbeError {
    data object ConnectionRefused : RuntimeHealthProbeError()
    data object ConnectionTimeout : RuntimeHealthProbeError()
    data object Unauthorized : RuntimeHealthProbeError()
    data object Forbidden : RuntimeHealthProbeError()
    data object NotFound : RuntimeHealthProbeError()
    data class ServerError(val statusCode: Int) : RuntimeHealthProbeError()
    data class IOError(val message: String) : RuntimeHealthProbeError()
    data class MalformedResponse(val message: String) : RuntimeHealthProbeError()
}
