package com.amitia.core.network.client

import com.google.common.truth.Truth.assertThat
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Before
import org.junit.Test
import java.net.SocketTimeoutException

class ErrorMappingInterceptorTest {

    private lateinit var server: MockWebServer
    private lateinit var client: OkHttpClient

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        client = OkHttpClient.Builder()
            .addInterceptor(ErrorMappingInterceptor())
            .connectTimeout(1, java.util.concurrent.TimeUnit.SECONDS)
            .readTimeout(1, java.util.concurrent.TimeUnit.SECONDS)
            .build()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun successful_response_passes_through() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("ok"))

        val request = Request.Builder().url(server.url("/test")).build()
        val response = client.newCall(request).execute()

        assertThat(response.code).isEqualTo(200)
        response.close()
    }

    @Test
    fun http_401_maps_to_TokenExpired() {
        server.enqueue(MockResponse().setResponseCode(401).setBody("unauthorized"))

        val request = Request.Builder().url(server.url("/api/test")).build()
        val exception = runCatching { client.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.TokenExpired::class.java)
    }

    @Test
    fun http_403_maps_to_TokenExpired() {
        server.enqueue(MockResponse().setResponseCode(403).setBody("forbidden"))

        val request = Request.Builder().url(server.url("/api/secret")).build()
        val exception = runCatching { client.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.TokenExpired::class.java)
    }

    @Test
    fun http_404_maps_to_NotFound_with_path() {
        server.enqueue(MockResponse().setResponseCode(404).setBody("missing"))

        val request = Request.Builder().url(server.url("/api/missing")).build()
        val exception = runCatching { client.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.NotFound::class.java)
        assertThat((exception as AmitiaApiException.NotFound).path).isEqualTo("/api/missing")
    }

    @Test
    fun http_500_maps_to_ServerError_with_body() {
        server.enqueue(MockResponse().setResponseCode(500).setBody("internal error"))

        val request = Request.Builder().url(server.url("/api/broken")).build()
        val exception = runCatching { client.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.ServerError::class.java)
        val serverError = exception as AmitiaApiException.ServerError
        assertThat(serverError.status).isEqualTo(500)
        assertThat(serverError.body).contains("internal error")
    }

    @Test
    fun http_503_maps_to_ServerError() {
        server.enqueue(MockResponse().setResponseCode(503).setBody("unavailable"))

        val request = Request.Builder().url(server.url("/api/down")).build()
        val exception = runCatching { client.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.ServerError::class.java)
        assertThat((exception as AmitiaApiException.ServerError).status).isEqualTo(503)
    }

    @Test
    fun connection_failure_to_unknown_host_maps_to_RemoteUnreachable() {
        val unreachableClient = OkHttpClient.Builder()
            .addInterceptor(ErrorMappingInterceptor())
            .connectTimeout(500, java.util.concurrent.TimeUnit.MILLISECONDS)
            .build()

        val request = Request.Builder().url("http://amitia-nonexistent-host-12345.local:18899/api/test").build()
        val exception = runCatching { unreachableClient.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.RemoteUnreachable::class.java)
    }

    @Test
    fun timeout_maps_to_Timeout_exception() {
        val slowServer = MockWebServer()
        slowServer.start()
        slowServer.enqueue(MockResponse().setResponseCode(200).setHeadersDelay(5, java.util.concurrent.TimeUnit.SECONDS))

        val timeoutClient = OkHttpClient.Builder()
            .addInterceptor(ErrorMappingInterceptor())
            .connectTimeout(500, java.util.concurrent.TimeUnit.MILLISECONDS)
            .readTimeout(500, java.util.concurrent.TimeUnit.MILLISECONDS)
            .build()

        val request = Request.Builder().url(slowServer.url("/slow")).build()
        val exception = runCatching { timeoutClient.newCall(request).execute() }.exceptionOrNull()

        assertThat(exception).isInstanceOf(AmitiaApiException.Timeout::class.java)
        slowServer.shutdown()
    }
}
