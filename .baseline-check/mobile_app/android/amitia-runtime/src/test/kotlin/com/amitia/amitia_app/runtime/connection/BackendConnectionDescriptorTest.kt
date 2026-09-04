package com.amitia.amitia_app.runtime.connection

import org.junit.Assert.assertEquals
import org.junit.Test

class BackendConnectionDescriptorTest {
    @Test
    fun `descriptor stores all fields`() {
        val credential = BackendConnectionCredential.create("a".repeat(32))
        val descriptor = BackendConnectionDescriptor(
            schemaVersion = 1,
            generation = 1L,
            host = "127.0.0.1",
            port = 18899,
            httpScheme = "http",
            webSocketScheme = "ws",
            livenessPath = "/livez",
            readinessPath = "/readyz",
            credential = credential,
        )
        assertEquals(1, descriptor.schemaVersion)
        assertEquals(1L, descriptor.generation)
        assertEquals("127.0.0.1", descriptor.host)
        assertEquals(18899, descriptor.port)
        assertEquals("http", descriptor.httpScheme)
        assertEquals("ws", descriptor.webSocketScheme)
        assertEquals("/livez", descriptor.livenessPath)
        assertEquals("/readyz", descriptor.readinessPath)
    }
}
