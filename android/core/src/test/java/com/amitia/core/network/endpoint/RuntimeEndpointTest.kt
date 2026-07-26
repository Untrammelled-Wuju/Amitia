package com.amitia.core.network.endpoint

import com.google.common.truth.Truth.assertThat
import org.junit.Test

class RuntimeEndpointTest {

    @Test
    fun local_endpoint_builds_http_baseUrl_with_default_port() {
        val endpoint = RuntimeEndpoint.Local(authToken = "abc")

        assertThat(endpoint.baseUrl()).isEqualTo("http://127.0.0.1:18899")
    }

    @Test
    fun local_endpoint_builds_wsUrl_with_default_port() {
        val endpoint = RuntimeEndpoint.Local(authToken = "abc")

        assertThat(endpoint.wsUrl()).isEqualTo("ws://127.0.0.1:18899")
    }

    @Test
    fun local_endpoint_uses_custom_host_and_port() {
        val endpoint = RuntimeEndpoint.Local(host = "192.168.1.10", port = 19178, authToken = null)

        assertThat(endpoint.baseUrl()).isEqualTo("http://192.168.1.10:19178")
        assertThat(endpoint.wsUrl()).isEqualTo("ws://192.168.1.10:19178")
    }

    @Test
    fun remote_endpoint_strips_trailing_slash_from_baseUrl() {
        val endpoint = RuntimeEndpoint.Remote(baseUrl = "https://api.example.com/", authToken = "tok")

        assertThat(endpoint.baseUrl()).isEqualTo("https://api.example.com")
    }

    @Test
    fun remote_endpoint_converts_http_to_ws_scheme() {
        val endpoint = RuntimeEndpoint.Remote(baseUrl = "http://api.example.com", authToken = null)

        assertThat(endpoint.wsUrl()).isEqualTo("ws://api.example.com")
    }

    @Test
    fun remote_endpoint_converts_https_to_wss_scheme() {
        val endpoint = RuntimeEndpoint.Remote(baseUrl = "https://api.example.com", authToken = null)

        assertThat(endpoint.wsUrl()).isEqualTo("wss://api.example.com")
    }

    @Test
    fun auth_header_returns_provided_token_for_local() {
        val endpoint = RuntimeEndpoint.Local(authToken = "token-xyz")

        assertThat(endpoint.authHeader()).isEqualTo("token-xyz")
    }

    @Test
    fun auth_header_returns_null_when_not_provided() {
        val endpoint = RuntimeEndpoint.Local(authToken = null)

        assertThat(endpoint.authHeader()).isNull()
    }
}
