package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeHealthProbeTest {

    @Test
    fun httpProbe_invalidHost_rejectedForReadinessProbe() {
        val probe = HttpRuntimeHealthProbe()
        val invalidEndpoint = BackendEndpointPolicy(
            host = "192.168.1.1",
            port = 8080,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        val result = probe.checkReadiness(invalidEndpoint)
        assertTrue(result is RuntimeHealthProbeResult.Failure)
    }

    @Test
    fun httpProbe_connectionRefused_returnsFailure() {
        val probe = HttpRuntimeHealthProbe()
        val endpoint = BackendEndpointPolicy(
            host = "127.0.0.1",
            port = 1,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        val result = probe.checkReadiness(endpoint)
        assertTrue(result is RuntimeHealthProbeResult.Failure)
        val failure = result as RuntimeHealthProbeResult.Failure
        assertTrue(failure.error is RuntimeHealthProbeError.ConnectionRefused)
    }

    @Test
    fun readBackendStatus_responseSchema() {
        val readyResponse = """{"code":200,"msg":"ready","data":{"status":"ready"}}"""
        assertEquals("ready", HttpRuntimeHealthProbe.readBackendStatus(readyResponse))

        val startingResponse = """{"code":200,"msg":"starting","data":{"status":"starting"}}"""
        assertEquals("starting", HttpRuntimeHealthProbe.readBackendStatus(startingResponse))

        val degradedResponse = """{"code":200,"msg":"degraded","data":{"status":"degraded"}}"""
        assertEquals("degraded", HttpRuntimeHealthProbe.readBackendStatus(degradedResponse))

        val blockedResponse = """{"code":503,"msg":"blocked","data":{"status":"blocked","reason":"blocked"}}"""
        assertEquals("blocked", HttpRuntimeHealthProbe.readBackendStatus(blockedResponse))

        assertNull(HttpRuntimeHealthProbe.readBackendStatus("not-json"))
        assertNull(HttpRuntimeHealthProbe.readBackendStatus(null))
        assertNull(HttpRuntimeHealthProbe.readBackendStatus(""))
    }

    @Test
    fun isLoopbackHost_forwardsLoopbackAddresses() {
        assertTrue(HttpRuntimeHealthProbe.isLoopbackHost("127.0.0.1"))
        assertTrue(HttpRuntimeHealthProbe.isLoopbackHost("localhost"))
        assertTrue(HttpRuntimeHealthProbe.isLoopbackHost("[::1]"))
        assertTrue(HttpRuntimeHealthProbe.isLoopbackHost("::1"))
    }

    @Test
    fun isLoopbackHost_rejectsNonLoopbackAddresses() {
        assertTrue(!HttpRuntimeHealthProbe.isLoopbackHost("192.168.1.1"))
        assertTrue(!HttpRuntimeHealthProbe.isLoopbackHost("example.com"))
        assertTrue(!HttpRuntimeHealthProbe.isLoopbackHost("10.0.0.1"))
    }

    @Test
    fun sanitizeHostForLog_truncatesExcessiveHost() {
        val longHost = "a".repeat(128)
        val sanitized = HttpRuntimeHealthProbe.sanitizeHostForLog(longHost)
        assertTrue(sanitized.length <= 64)
        assertTrue(sanitized.contains("..."))
    }

    @Test
    fun sanitizeHostForLog_preservesShortHost() {
        val shortHost = "127.0.0.1"
        assertEquals(shortHost, HttpRuntimeHealthProbe.sanitizeHostForLog(shortHost))
    }
}
