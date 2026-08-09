package com.amitia.amitia_app.runtime.startup

internal sealed class RuntimeStartupError {
    data object Cancelled : RuntimeStartupError()
    data object GenerationStale : RuntimeStartupError()
    data object ProotNotRunning : RuntimeStartupError()
    data class ProotExited(val exitCode: Int) : RuntimeStartupError()
    data object BackendConnectionRefused : RuntimeStartupError()
    data object BackendLivenessFailed : RuntimeStartupError()
    data object BackendReadinessFailed : RuntimeStartupError()
    data object HealthAuthFailed : RuntimeStartupError()
    data object HealthEndpointMissing : RuntimeStartupError()
    data object Timeout : RuntimeStartupError()
    data object InvalidEndpoint : RuntimeStartupError()
    data class InternalError(val message: String) : RuntimeStartupError()
}
