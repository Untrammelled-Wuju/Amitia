package com.amitia.amitia_app.nativeprovider.model

import org.junit.Assert.assertEquals
import org.junit.Test

class OperationTimeoutTest {
    @Test
    fun globalPolicyOverridesCommandTimeout() {
        val request = NativeBridgeRequest(1, "test", "android", "root.execute", timeoutPolicy = mapOf("disabled" to false, "seconds" to 450))
        assertEquals(450000L, request.executionTimeoutMillis(5000L))
        assertEquals(0L, request.copy(timeoutPolicy = mapOf("disabled" to true, "seconds" to 450)).executionTimeoutMillis(5000L))
        assertEquals(5000L, request.copy(timeoutPolicy = null).executionTimeoutMillis(5000L))
        assertEquals(5000L, request.copy(timeoutPolicy = mapOf("disabled" to true, "seconds" to -1)).executionTimeoutMillis(5000L))
    }
}
