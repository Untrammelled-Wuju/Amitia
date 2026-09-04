package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.connection.internal.BackendConnectionMapper
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class BackendConnectionMapperTest {
    @Test
    fun `available payload has expected structure`() {
        val credential = BackendConnectionCredential.create("a".repeat(32))
        val descriptor = BackendConnectionMapper.buildDescriptor(
            policy = embeddedAndroidBackendPolicy(),
            generation = 1L,
            credential = credential,
        )
        val map = BackendConnectionMapper.toPayload(true, descriptor, null)
        assertEquals(1, map["schemaVersion"])
        assertEquals("available", map["status"])
        assertEquals(1L, map["generation"])
        val endpoint = map["endpoint"] as Map<*, *>
        assertEquals("127.0.0.1", endpoint["host"])
        assertEquals(18899, endpoint["port"])
        assertEquals("http", endpoint["httpScheme"])
        assertEquals("ws", endpoint["webSocketScheme"])
        assertEquals("/livez", endpoint["livenessPath"])
        assertEquals("/readyz", endpoint["readinessPath"])
        val auth = map["authentication"] as Map<*, *>
        assertEquals("local_token", auth["type"])
        assertEquals("X-Amitia-Local-Token", auth["header"])
        assertTrue(auth.containsKey("token"))
    }

    @Test
    fun `unavailable payload has expected structure`() {
        val map = BackendConnectionMapper.toPayload(false, null, null)
        assertEquals(1, map["schemaVersion"])
        assertEquals("unavailable", map["status"])
        val error = map["error"] as Map<*, *>
        assertTrue(error.containsKey("code"))
        assertFalse(map.containsKey("endpoint"))
        assertFalse(map.containsKey("authentication"))
    }

    @Test
    fun `error payload does not contain token`() {
        val map = BackendConnectionMapper.toPayload(false, null, null)
        val encoded = map.toString()
        assertFalse(encoded.contains("X-Amitia-Local-Token"))
    }
}
