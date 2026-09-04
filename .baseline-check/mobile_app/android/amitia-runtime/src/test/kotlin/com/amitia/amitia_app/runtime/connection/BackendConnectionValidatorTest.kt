package com.amitia.amitia_app.runtime.connection

import com.amitia.amitia_app.runtime.connection.internal.BackendConnectionValidator
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

class BackendConnectionValidatorTest {
    @Test
    fun `embedded policy passes validation`() {
        val error = BackendConnectionValidator.validate(embeddedAndroidBackendPolicy())
        assertNull(error)
    }

    @Test
    fun `rejects localhost host`() {
        val policy = BackendEndpointPolicy("localhost", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects 0_0_0_0 host`() {
        val policy = BackendEndpointPolicy("0.0.0.0", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects double colon host`() {
        val policy = BackendEndpointPolicy("::", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects port zero`() {
        val policy = BackendEndpointPolicy("127.0.0.1", 0, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects port above 65535`() {
        val policy = BackendEndpointPolicy("127.0.0.1", 70000, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects host with scheme`() {
        val policy = BackendEndpointPolicy("http://127.0.0.1", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects host with path`() {
        val policy = BackendEndpointPolicy("127.0.0.1/path", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects mismatched http ws scheme`() {
        val policy = BackendEndpointPolicy("127.0.0.1", 18899, "http", "wss")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }

    @Test
    fun `rejects blank host`() {
        val policy = BackendEndpointPolicy("", 18899, "http", "ws")
        val error = BackendConnectionValidator.validate(policy)
        assertNotNull(error)
    }
}
