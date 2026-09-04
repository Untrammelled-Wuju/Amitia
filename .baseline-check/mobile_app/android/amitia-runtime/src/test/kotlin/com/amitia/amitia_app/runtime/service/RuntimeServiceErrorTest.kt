package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceErrorTest {

    @Test
    fun runtimeServiceErrorCode_containsExpectedValues() {
        val codes = RuntimeServiceErrorCode.entries
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_START_FAILED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_BIND_FAILED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_NOT_AVAILABLE))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_FOREGROUND_FAILED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_NOTIFICATION_FAILED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_ALREADY_ACTIVE))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_STOP_FAILED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_TERMINATED))
        assertTrue(codes.contains(RuntimeServiceErrorCode.SERVICE_INTERNAL_ERROR))
    }

    @Test
    fun runtimeServiceError_preservesCodeAndMessage() {
        val error = RuntimeServiceError(
            code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
            message = "test error"
        )
        assertEquals(RuntimeServiceErrorCode.SERVICE_START_FAILED, error.code)
        assertEquals("test error", error.message)
        assertNull(error.cause)
    }

    @Test
    fun runtimeServiceError_preservesCause() {
        val cause = IllegalStateException("root cause")
        val error = RuntimeServiceError(
            code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
            message = "test error",
            cause = cause
        )
        assertEquals(cause, error.cause)
    }
}
