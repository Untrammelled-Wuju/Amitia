package com.amitia.amitia_app.runtime.connection

data class BackendEndpointPolicy(
    val host: String,
    val port: Int,
    val httpScheme: String,
    val webSocketScheme: String,
)

fun embeddedAndroidBackendPolicy(): BackendEndpointPolicy {
    return BackendEndpointPolicy(
        host = "127.0.0.1",
        port = 18899,
        httpScheme = "http",
        webSocketScheme = "ws",
    )
}
