package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeError
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeBridgeErrorMapperTest {

    @Test
    fun maps_error_code_to_string() {
        val error = RuntimeError(
            code = RuntimeErrorCode.START_FAILED,
            message = "start failed",
            recoverable = true,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        assertEquals("START_FAILED", mapped["code"])
    }

    @Test
    fun maps_error_message() {
        val error = RuntimeError(
            code = RuntimeErrorCode.INSTALL_FAILED,
            message = "installation error occurred",
            recoverable = false,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        assertEquals("installation error occurred", mapped["message"])
    }

    @Test
    fun maps_recoverable_flag() {
        val error = RuntimeError(
            code = RuntimeErrorCode.TIMEOUT,
            message = "timed out",
            recoverable = true,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        assertEquals(true, mapped["retryable"])
    }

    @Test
    fun sanitizes_host_path_in_message() {
        val error = RuntimeError(
            code = RuntimeErrorCode.INSTALL_FAILED,
            message = "failed at /data/user/0/com.amitia.amitia_app/files/runtime",
            recoverable = true,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        val message = mapped["message"] as String
        assertFalse(message.contains("/data/user/"))
        assertTrue(message.contains("[redacted]"))
    }

    @Test
    fun sanitizes_data_path_in_message() {
        val error = RuntimeError(
            code = RuntimeErrorCode.INSTALL_FAILED,
            message = "failed at /data/data/com.amitia.amitia_app",
            recoverable = true,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        val message = mapped["message"] as String
        assertFalse(message.contains("/data/data/"))
    }

    @Test
    fun sanitizes_local_token_in_message() {
        val error = RuntimeError(
            code = RuntimeErrorCode.PERMISSION_DENIED,
            message = "invalid X-Amitia-Local-Token:abc123",
            recoverable = false,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        val message = mapped["message"] as String
        assertFalse(message.contains("abc123"))
        assertTrue(message.contains("[redacted]"))
    }

    @Test
    fun maps_all_error_codes() {
        for (code in RuntimeErrorCode.values()) {
            val error = RuntimeError(code = code, message = "test", recoverable = false)
            val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
            assertNotNull(mapped["code"])
            assertFalse((mapped["code"] as String).isEmpty())
        }
    }

    @Test
    fun command_result_error_contains_accepted_false() {
        val error = RuntimeError(
            code = RuntimeErrorCode.INTERNAL_ERROR,
            message = "internal error",
            recoverable = false,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToCommandResultError(error)
        assertEquals(false, mapped["accepted"])
        assertNotNull(mapped["error"])
    }

    @Test
    fun not_implemented_code_mapped() {
        val error = RuntimeError(
            code = RuntimeErrorCode.NOT_IMPLEMENTED,
            message = "not implemented",
            recoverable = false,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        assertEquals("NOT_IMPLEMENTED", mapped["code"])
    }

    @Test
    fun runtime_execution_not_available_code_mapped() {
        val error = RuntimeError(
            code = RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE,
            message = "runtime not available",
            recoverable = true,
        )
        val mapped = RuntimeBridgeErrorMapper.mapToBridgeError(error)
        assertEquals("RUNTIME_EXECUTION_NOT_AVAILABLE", mapped["code"])
    }
}
