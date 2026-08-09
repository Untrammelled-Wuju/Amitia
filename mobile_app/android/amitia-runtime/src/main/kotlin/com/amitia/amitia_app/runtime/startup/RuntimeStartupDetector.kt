package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.proot.ProotSession

internal data class RuntimeStartupRequest(
    val generation: Long,
    val session: ProotSession,
    val endpoint: BackendEndpointPolicy
)

internal interface RuntimeStartupDetector {
    fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult
    fun cancel()
}
