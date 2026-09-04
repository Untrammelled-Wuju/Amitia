package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy

internal interface RuntimeHealthProbe {
    fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult
}
