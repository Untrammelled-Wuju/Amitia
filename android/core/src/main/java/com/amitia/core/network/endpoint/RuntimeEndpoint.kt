package com.amitia.core.network.endpoint

sealed interface RuntimeEndpoint {

    data class Local(
        val host: String = "127.0.0.1",
        val port: Int = 18899,
        val authToken: String?
    ) : RuntimeEndpoint

    data class Remote(
        val baseUrl: String,
        val authToken: String?
    ) : RuntimeEndpoint

    fun baseUrl(): String = when (this) {
        is Local -> "http://$host:$port"
        is Remote -> baseUrl.removeSuffix("/")
    }

    fun wsUrl(): String = when (this) {
        is Local -> "ws://$host:$port"
        is Remote -> baseUrl.removeSuffix("/")
            .replace("http://", "ws://")
            .replace("https://", "wss://")
    }

    fun authHeader(): String? = when (this) {
        is Local -> authToken
        is Remote -> authToken
    }
}
