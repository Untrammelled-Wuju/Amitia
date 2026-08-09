package com.amitia.amitia_app.runtime.bridge

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeInstallRequest
import com.amitia.amitia_app.runtime.api.RuntimeListener
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationHandle
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
import com.amitia.amitia_app.runtime.api.RuntimeRepairRequest
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeVerifyRequest
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeBridgeHandlerTest {

    private class FakeController : RuntimeController {
        var startCalled = false
        var stopCalled = false
        var installCalled = false
        var verifyCalled = false
        var repairCalled = false
        val snapshot = RuntimeSnapshot.initial()
        private val subscriptions = mutableListOf<RuntimeListener>()

        override fun snapshot(): RuntimeSnapshot = snapshot
        override fun subscribe(listener: RuntimeListener): RuntimeSubscription {
            subscriptions.add(listener)
            return object : RuntimeSubscription {
                override fun cancel() {}
                override fun isCancelled() = false
            }
        }
        override fun install(
            request: RuntimeInstallRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle {
            installCalled = true
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = "op-1",
                    type = RuntimeOperationType.INSTALL,
                    snapshot = snapshot,
                )
            )
            return createHandle(RuntimeOperationType.INSTALL)
        }
        override fun verify(
            request: RuntimeVerifyRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle {
            verifyCalled = true
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = "op-2",
                    type = RuntimeOperationType.VERIFY,
                    snapshot = snapshot,
                )
            )
            return createHandle(RuntimeOperationType.VERIFY)
        }
        override fun start(
            request: RuntimeStartRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle {
            startCalled = true
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = "op-3",
                    type = RuntimeOperationType.START,
                    snapshot = snapshot,
                )
            )
            return createHandle(RuntimeOperationType.START)
        }
        override fun stop(
            request: RuntimeStopRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle {
            stopCalled = true
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = "op-4",
                    type = RuntimeOperationType.STOP,
                    snapshot = snapshot,
                )
            )
            return createHandle(RuntimeOperationType.STOP)
        }
        override fun repair(
            request: RuntimeRepairRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle {
            repairCalled = true
            callback.onCompleted(
                RuntimeOperationResult.Success(
                    operationId = "op-5",
                    type = RuntimeOperationType.REPAIR,
                    snapshot = snapshot,
                )
            )
            return createHandle(RuntimeOperationType.REPAIR)
        }
        private fun createHandle(type: RuntimeOperationType) = object : RuntimeOperationHandle {
            override val operationId = "op-test"
            override val type = type
            override fun cancel() = false
            override fun isCancelled() = false
            override fun isCompleted() = true
        }
    }

    private class FakeResult : MethodChannel.Result {
        var successResult: Any? = null
        var errorCode: String? = null
        var errorMessage: String? = null
        var notImplementedCalled = false

        override fun success(result: Any?) { successResult = result }
        override fun error(code: String, message: String?, details: Any?) {
            errorCode = code
            errorMessage = message
        }
        override fun notImplemented() { notImplementedCalled = true }
    }

    @Test
    fun snapshot_returns_mapped_snapshot() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_SNAPSHOT, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertNotNull(result.successResult)
        val map = result.successResult as Map<*, *>
        assertEquals(1, map["schemaVersion"])
        assertEquals("UNAVAILABLE", map["state"])
    }

    @Test
    fun start_delegates_to_controller() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_START, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(controller.startCalled)
        assertNotNull(result.successResult)
    }

    @Test
    fun stop_delegates_to_controller() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_STOP, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(controller.stopCalled)
        assertNotNull(result.successResult)
    }

    @Test
    fun install_delegates_to_controller() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_INSTALL, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(controller.installCalled)
        assertNotNull(result.successResult)
    }

    @Test
    fun verify_delegates_to_controller() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_VERIFY, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(controller.verifyCalled)
        assertNotNull(result.successResult)
    }

    @Test
    fun repair_delegates_to_controller() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_REPAIR, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(controller.repairCalled)
        assertNotNull(result.successResult)
    }

    @Test
    fun unknown_method_returns_not_implemented() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall("unknown.method", null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertTrue(result.notImplementedCalled)
    }

    @Test
    fun manifest_summary_returns_null_when_not_installed() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_MANIFEST_SUMMARY, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        assertEquals(null, result.successResult)
    }

    @Test
    fun command_result_contains_accepted_field() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_START, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        val map = result.successResult as Map<*, *>
        assertNotNull(map["accepted"])
        assertEquals(true, map["accepted"])
    }

    @Test
    fun command_result_contains_snapshot() {
        val controller = FakeController()
        val handler = RuntimeBridgeHandler(controller = controller, manifestStore = null)
        val call = MethodCall(RuntimeBridgeContract.METHOD_START, null)
        val result = FakeResult()

        handler.onMethodCall(call, result)

        val map = result.successResult as Map<*, *>
        assertNotNull(map["snapshot"])
    }
}
