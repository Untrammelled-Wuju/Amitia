package com.amitia.amitia_app.runtime.connection

import org.junit.Assert.assertEquals
import org.junit.Test

class BackendEndpointPolicyTest {
    @Test
    fun `embedded policy returns 127_0_0_1 for host`() {
        val policy = embeddedAndroidBackendPolicy()
        assertEquals("127.0.0.1", policy.host)
    }

    @Test
    fun `embedded policy returns 18899 for port`() {
        val policy = embeddedAndroidBackendPolicy()
        assertEquals(18899, policy.port)
    }

    @Test
    fun `embedded policy returns http for httpScheme`() {
        val policy = embeddedAndroidBackendPolicy()
        assertEquals("http", policy.httpScheme)
    }

    @Test
    fun `embedded policy returns ws for webSocketScheme`() {
        val policy = embeddedAndroidBackendPolicy()
        assertEquals("ws", policy.webSocketScheme)
    }
}
