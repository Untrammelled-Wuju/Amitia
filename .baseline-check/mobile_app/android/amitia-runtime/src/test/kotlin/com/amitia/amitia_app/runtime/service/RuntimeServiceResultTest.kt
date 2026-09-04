package com.amitia.amitia_app.runtime.service

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeServiceResultTest {

    @Test
    fun success_isSingletonObject() {
        val a = RuntimeServiceResult.Success
        val b = RuntimeServiceResult.Success
        assertTrue(a === b)
    }

    @Test
    fun failure_carriesError() {
        val error = RuntimeServiceError(
            code = RuntimeServiceErrorCode.SERVICE_START_FAILED,
            message = "failure"
        )
        val result = RuntimeServiceResult.Failure(error)
        assertEquals(error, result.error)
    }

    @Test
    fun failure_canBeMatched() {
        val error = RuntimeServiceError(
            code = RuntimeServiceErrorCode.SERVICE_NOT_AVAILABLE,
            message = "not available"
        )
        val result: RuntimeServiceResult = RuntimeServiceResult.Failure(error)
        when (result) {
            is RuntimeServiceResult.Success -> throw AssertionError("expected Failure")
            is RuntimeServiceResult.Failure -> {
                assertEquals(RuntimeServiceErrorCode.SERVICE_NOT_AVAILABLE, result.error.code)
            }
        }
    }
}
