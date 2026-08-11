package com.amitia.amitia_app.runtime.startup

internal sealed class RuntimeStartupError {
    data object Cancelled : RuntimeStartupError()
    data object GenerationStale : RuntimeStartupError()
    data object ProotNotRunning : RuntimeStartupError()
    data class ProotExited(val exitCode: Int?, val elapsedMs: Long = 0L) : RuntimeStartupError()
    data object BackendConnectionRefused : RuntimeStartupError()
    data object BackendLivenessFailed : RuntimeStartupError()
    data object BackendReadinessFailed : RuntimeStartupError()
    data object HealthAuthFailed : RuntimeStartupError()
    data object HealthEndpointMissing : RuntimeStartupError()
    data class Timeout(val timeoutMs: Long, val elapsedMs: Long, val probeCount: Int) : RuntimeStartupError()
    data object InvalidEndpoint : RuntimeStartupError()
    data class InvalidResponse(val reason: String, val elapsedMs: Long = 0L) : RuntimeStartupError()
    data class InternalError(val message: String) : RuntimeStartupError()
}
