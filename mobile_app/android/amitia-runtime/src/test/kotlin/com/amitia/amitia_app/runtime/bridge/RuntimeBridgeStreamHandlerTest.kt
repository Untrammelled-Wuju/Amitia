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
import io.flutter.plugin.common.EventChannel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeBridgeStreamHandlerTest {

    private class FakeController : RuntimeController {
        val listeners = mutableListOf<RuntimeListener>()
        val snapshot = RuntimeSnapshot.initial()

        override fun snapshot(): RuntimeSnapshot = snapshot
        override fun subscribe(listener: RuntimeListener): RuntimeSubscription {
            listeners.add(listener)
            return object : RuntimeSubscription {
                override fun cancel() { listeners.remove(listener) }
                override fun isCancelled() = !listeners.contains(listener)
            }
        }
        override fun install(
            request: RuntimeInstallRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle = createHandle(RuntimeOperationType.INSTALL)
        override fun verify(
            request: RuntimeVerifyRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle = createHandle(RuntimeOperationType.VERIFY)
        override fun start(
            request: RuntimeStartRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle = createHandle(RuntimeOperationType.START)
        override fun stop(
            request: RuntimeStopRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle = createHandle(RuntimeOperationType.STOP)
        override fun repair(
            request: RuntimeRepairRequest,
            callback: RuntimeOperationCallback
        ): RuntimeOperationHandle = createHandle(RuntimeOperationType.REPAIR)
        private fun createHandle(type: RuntimeOperationType) = object : RuntimeOperationHandle {
            override val operationId = "op-test"
            override val type = type
            override fun cancel() = false
            override fun isCancelled() = false
            override fun isCompleted() = true
        }
    }

    private class FakeSink : EventChannel.EventSink {
        val events = mutableListOf<Any?>()
        var endOfStreamCalled = false
        var errorCode: String? = null

        override fun success(event: Any?) { events.add(event) }
        override fun error(code: String, message: String?, details: Any?) { errorCode = code }
        override fun endOfStream() { endOfStreamCalled = true }
    }

    @Test
    fun onListen_emits_current_snapshot() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink = FakeSink()

        handler.onListen(null, sink)

        assertTrue(sink.events.isNotEmpty())
        val firstEvent = sink.events.first() as Map<*, *>
        assertEquals(1, firstEvent["schemaVersion"])
    }

    @Test
    fun onCancel_stops_emitting() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink = FakeSink()

        handler.onListen(null, sink)
        handler.onCancel(null)

        assertTrue(controller.listeners.isEmpty())
    }

    @Test
    fun detach_releases_resources() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink = FakeSink()

        handler.onListen(null, sink)
        handler.detach()

        assertTrue(controller.listeners.isEmpty())
    }

    @Test
    fun snapshot_contains_required_fields() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink = FakeSink()

        handler.onListen(null, sink)

        val event = sink.events.first() as Map<*, *>
        assertNotNull(event["schemaVersion"])
        assertNotNull(event["state"])
        assertNotNull(event["generation"])
        assertNotNull(event["runtimeInstalled"])
        assertNotNull(event["runtimeAvailable"])
    }

    @Test
    fun snapshot_state_is_mapped_correctly() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink = FakeSink()

        handler.onListen(null, sink)

        val event = sink.events.first() as Map<*, *>
        assertEquals("UNAVAILABLE", event["state"])
    }

    @Test
    fun onListen_with_active_flag_prevents_duplicate_subscription() {
        val controller = FakeController()
        val handler = RuntimeBridgeStreamHandler(
            controller = controller,
            manifestStore = null,
        )
        val sink1 = FakeSink()
        val sink2 = FakeSink()

        handler.onListen(null, sink1)
        val firstListenerCount = controller.listeners.size
        handler.onListen(null, sink2)

        assertEquals(firstListenerCount, controller.listeners.size)
        assertTrue(sink2.events.isNotEmpty())
    }
}
