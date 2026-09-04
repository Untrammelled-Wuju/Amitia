package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.proot.ProotEnvironment

internal object ProotBackendEnvironment {
    fun apply(
        policy: BackendEndpointPolicy,
        environment: Map<String, String>,
    ): ProotEnvironment {
        val merged = HashMap<String, String>(environment)
        merged["AMITIA_SERVER_HOST"] = policy.host
        merged["AMITIA_SERVER_PORT"] = policy.port.toString()
        return ProotEnvironment.of(merged.toList())
    }

    fun entries(policy: BackendEndpointPolicy): List<Pair<String, String>> {
        return listOf(
            "AMITIA_SERVER_HOST" to policy.host,
            "AMITIA_SERVER_PORT" to policy.port.toString(),
        )
    }
}
