package com.amitia.runtime.health

import okhttp3.OkHttpClient
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class OkHttpClientProvider @Inject constructor() {

    val client: OkHttpClient by lazy {
        OkHttpClient.Builder()
            .connectTimeout(DEFAULT_CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            .readTimeout(DEFAULT_READ_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            .writeTimeout(DEFAULT_WRITE_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            .retryOnConnectionFailure(false)
            .followRedirects(false)
            .build()
    }

    fun clientWithTimeout(timeoutMs: Long): OkHttpClient {
        if (timeoutMs <= 0L) return client
        return client.newBuilder()
            .connectTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .readTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .writeTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .build()
    }

    companion object {
        private const val DEFAULT_CONNECT_TIMEOUT_SECONDS = 2L
        private const val DEFAULT_READ_TIMEOUT_SECONDS = 2L
        private const val DEFAULT_WRITE_TIMEOUT_SECONDS = 2L
    }
}
