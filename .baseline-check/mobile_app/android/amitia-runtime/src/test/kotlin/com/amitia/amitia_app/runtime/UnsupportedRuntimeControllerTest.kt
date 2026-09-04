package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import com.amitia.amitia_app.runtime.internal.RuntimeIdGenerator
import com.amitia.amitia_app.runtime.internal.UnsupportedRuntimeController
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class UnsupportedRuntimeControllerTest {

    private val idStub = object : RuntimeIdGenerator {
        override fun nextOperationId(): String = "test-op-001"
    }
    private val controller = UnsupportedRuntimeController(idStub)

    @Test
    fun install_returnsNotImplemented() {
        val received = mutableListOf<RuntimeOperationResult>()
        controller.install(
            RuntimeInstallRequest("file:///test/runtime.zip", null, false)
        ) { received.add(it) }

        assertEquals(1, received.size)
        val result = received[0]
        assertTrue(result is RuntimeOperationResult.Failure)
        val failure = result as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.NOT_IMPLEMENTED, failure.error.code)
        assertFalse(failure.error.recoverable)
    }

    @Test
    fun verify_returnsNotImplemented() {
        val received = mutableListOf<RuntimeOperationResult>()
        controller.verify(RuntimeVerifyRequest(deep = false)) { received.add(it) }

        assertEquals(1, received.size)
        val result = received[0] as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.NOT_IMPLEMENTED, result.error.code)
    }

    @Test
    fun start_returnsNotImplemented() {
        val received = mutableListOf<RuntimeOperationResult>()
        controller.start(RuntimeStartRequest(RuntimeStartReason.APP_LAUNCH)) { received.add(it) }

        assertEquals(1, received.size)
        val result = received[0] as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.NOT_IMPLEMENTED, result.error.code)
    }

    @Test
    fun stop_returnsNotImplemented() {
        val received = mutableListOf<RuntimeOperationResult>()
        controller.stop(RuntimeStopRequest(RuntimeStopReason.USER_REQUEST, force = false)) { received.add(it) }

        assertEquals(1, received.size)
        val result = received[0] as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.NOT_IMPLEMENTED, result.error.code)
    }

    @Test
    fun repair_returnsNotImplemented() {
        val received = mutableListOf<RuntimeOperationResult>()
        controller.repair(RuntimeRepairRequest(packageUri = null, preserveUserData = true)) { received.add(it) }

        assertEquals(1, received.size)
        val result = received[0] as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.NOT_IMPLEMENTED, result.error.code)
    }

    @Test
    fun callback_invokedExactlyOnce() {
        val count = mutableListOf<RuntimeOperationResult>()
        controller.install(
            RuntimeInstallRequest("file:///test.zip", null, false)
        ) { count.add(it) }
        assertEquals(1, count.size)
    }

    @Test
    fun handle_isAlreadyCompleted() {
        val handle = controller.install(
            RuntimeInstallRequest("file:///test.zip", null, false)
        ) { }
        assertTrue(handle.isCompleted())
    }

    @Test
    fun cancel_returnsFalse() {
        val handle = controller.install(
            RuntimeInstallRequest("file:///test.zip", null, false)
        ) { }
        assertFalse(handle.cancel())
    }

    @Test
    fun snapshot_unchangedAfterOperations() {
        val initial = controller.snapshot()
        controller.install(
            RuntimeInstallRequest("file:///test.zip", null, false)
        ) { }
        assertEquals(initial, controller.snapshot())
        assertEquals(RuntimeState.UNKNOWN, controller.snapshot().state)
    }

    @Test
    fun listener_doesNotReceiveFalseStateChanges() {
        val received = mutableListOf<RuntimeSnapshot>()
        controller.subscribe { received.add(it) }
        assertEquals(1, received.size)
        assertEquals(RuntimeState.UNKNOWN, received[0].state)
    }
}
