package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

internal class FakeRuntimeServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    var stopRequested = false
    var ensureStartCount = 0
    var stopCallCount = 0

    override fun ensureStarted(): RuntimeServiceResult {
        ensureStartCount++
        return RuntimeServiceResult.Success
    }
    override fun requestStop(): RuntimeServiceResult {
        stopCallCount++
        stopRequested = true
        return RuntimeServiceResult.Success
    }
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = 0L
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

class RuntimeShutdownControllerTest {

    private fun createStateStoreWith(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        val path = when (targetState) {
            RuntimeState.READY -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.READY
            )
            RuntimeState.STARTING -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING
            )
            RuntimeState.STOPPING -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING
            )
            RuntimeState.STOPPED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING, RuntimeState.STOPPED
            )
            RuntimeState.FAILED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.FAILED
            )
            RuntimeState.DEGRADED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.READY, RuntimeState.DEGRADED
            )
            RuntimeState.INSTALLED -> listOf(RuntimeState.INSTALLED)
            else -> listOf()
        }
        for (s in path) {
            store.update { it.copy(state = s) }
        }
        return store
    }

    private fun stopAndCapture(controller: RuntimeController): RuntimeOperationResult {
        val resultRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.stop(RuntimeStopRequest(reason = RuntimeStopReason.USER_REQUEST, force = false), callback)
        latch.await(5, TimeUnit.SECONDS)
        return resultRef.get()
    }

    @Test
    fun stopped_stop_returnsSuccess_idempotent() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPED)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(0, host.stopCallCount)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun stopping_stop_returnsAlreadyInProgress() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.STOP_ALREADY_IN_PROGRESS, (result as RuntimeOperationResult.Failure).error.code)
        assertEquals(0, host.stopCallCount)
    }

    @Test
    fun ready_stop_transitionsToStopping_thenStopped() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun starting_stop_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun degraded_stop_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.DEGRADED)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun failed_stop_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.FAILED)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun invalidStateInstalled_stop_returnsInvalidState() {
        val stateStore = createStateStoreWith(RuntimeState.INSTALLED)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.INVALID_STATE, (result as RuntimeOperationResult.Failure).error.code)
    }

    @Test
    fun duplicateExpectedStopped_onlyOneStateTransition() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun unexpectedTerminationDuringStop_goesToStopped() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
            )
        )
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun serviceRequestStopFails_transitionsToFailed() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = object : RuntimeServiceHost {
            private val listeners = mutableListOf<RuntimeServiceHostListener>()
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Failure(
                com.amitia.amitia_app.runtime.service.RuntimeServiceError(
                    com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                    "stop failed"
                )
            )
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
            override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
            override fun currentGeneration(): Long = 0L
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.STOP_SERVICE_TEARDOWN_FAILED, (result as RuntimeOperationResult.Failure).error.code)
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }
}
