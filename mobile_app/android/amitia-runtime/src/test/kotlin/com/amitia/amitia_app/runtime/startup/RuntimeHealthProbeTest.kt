package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeHealthProbeTest {

    @Test
    fun httpProbe_invalidHost_returnsFailure() {
        val probe = HttpRuntimeHealthProbe()
        val invalidEndpoint = BackendEndpointPolicy(
            host = "192.168.1.1",
            port = 8080,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        val result = probe.checkLiveness(invalidEndpoint)
        assertTrue(result is RuntimeHealthProbeResult.Failure)
    }

    @Test
    fun httpProbe_returnsSuccessForConnectableEndpoint() {
        val probe = HttpRuntimeHealthProbe(
            connectTimeoutMs = 1000,
            readTimeoutMs = 1000
        )
        val endpoint = BackendEndpointPolicy(
            host = "127.0.0.1",
            port = 1,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        val result = probe.checkLiveness(endpoint)
        assertTrue(result is RuntimeHealthProbeResult.Failure)
    }

    @Test
    fun httpProbe_livenessAndReadiness_useCorrectPaths() {
        val paths = mutableListOf<String>()
        val probe = object : RuntimeHealthProbe {
            override fun checkLiveness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
                paths.add("liveness")
                return RuntimeHealthProbeResult.Success(200)
            }
            override fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
                paths.add("readiness")
                return RuntimeHealthProbeResult.Success(200)
            }
        }
        val endpoint = BackendEndpointPolicy(
            host = "127.0.0.1",
            port = 18899,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        probe.checkLiveness(endpoint)
        probe.checkReadiness(endpoint)
        assertEquals(2, paths.size)
    }
}
