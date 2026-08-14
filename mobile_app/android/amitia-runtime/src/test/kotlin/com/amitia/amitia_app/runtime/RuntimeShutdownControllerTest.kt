package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeController
import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal class FakeRuntimeServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    var stopRequested = false
    var ensureStartCount = 0
    var stopCallCount = 0
    var activeSession: ProotSession? = null

    override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult {
        ensureStartCount++
        return RuntimeServiceResult.Success
    }
    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        stopCallCount++
        stopRequested = true
        activeSession = null
        return RuntimeServiceResult.Success
    }
    override fun requestTeardownAfterStartupFailure() {}
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    override fun currentSession(): ProotSession? = activeSession
    override fun currentGeneration(): Long = 0L
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

internal class FakeStartupDetector : RuntimeStartupDetector {
    var cancelCount = 0
    var awaitCallCount = 0
    val cancelled = AtomicBoolean(false)
    private val pendingResult = AtomicReference<RuntimeStartupResult?>(null)

    fun completeWith(result: RuntimeStartupResult) {
        pendingResult.set(result)
    }

    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        awaitCallCount++
        cancelled.set(false)
        while (true) {
            val pending = pendingResult.getAndSet(null)
            if (pending != null) return pending
            if (cancelled.get()) return RuntimeStartupResult.Cancelled(request.generation)
            Thread.sleep(10)
        }
    }

    override fun cancel() {
        cancelCount++
        cancelled.set(true)
    }
}

internal class FakeProotSession(override val sessionId: String) : ProotSession {
    private val alive = AtomicBoolean(true)
    var stopCallCount = 0
    var forceCallCount = 0
    var gracefulTimeoutMs = 0L

    override fun isAlive(): Boolean = alive.get()
    override fun awaitExit(timeoutMillis: Long): Int? {
        Thread.sleep(10)
        return if (!alive.get()) 0 else null
    }
    override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult {
        stopCallCount++
        gracefulTimeoutMs = graceMillis
        Thread.sleep(10)
        alive.set(false)
        return com.amitia.amitia_app.runtime.proot.ProotStopResult.Graceful(sessionId, 0)
    }
    override fun close() { alive.set(false) }
    override fun requestStop() {}
    override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? = null
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
    fun stopping_stop_returnsSuccess_idempotent() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(0, host.stopCallCount)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun repeatedStop_onlyOneShutdownFlow() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val r1 = stopAndCapture(controller)
        val r2 = stopAndCapture(controller)
        val r3 = stopAndCapture(controller)
        assertTrue(r1 is RuntimeOperationResult.Success)
        assertTrue(r2 is RuntimeOperationResult.Success)
        assertTrue(r3 is RuntimeOperationResult.Success)
        assertEquals(1, host.stopCallCount)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
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
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
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
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
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
                generation = controller.snapshot().generation,
                cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
            )
        )
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun startingStop_cancelsStartupDetector() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val detector = FakeStartupDetector()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            startupDetector = detector,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(1, detector.cancelCount)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun lateReady_afterStop_isIgnored() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        stopAndCapture(controller)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
        host.emit(RuntimeServiceHostEvent.SessionReady(generation = 999L, sessionId = "stale"))
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun concurrentStop_singleShutdownFlow() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val latch = CountDownLatch(3)
        val results = mutableListOf<RuntimeOperationResult>()
        val lock = Any()
        repeat(3) {
            Thread {
                val callback = object : RuntimeOperationCallback {
                    override fun onCompleted(result: RuntimeOperationResult) {
                        synchronized(lock) { results.add(result) }
                        latch.countDown()
                    }
                }
                controller.stop(RuntimeStopRequest(RuntimeStopReason.USER_REQUEST, false), callback)
            }.start()
        }
        latch.await(5, TimeUnit.SECONDS)
        assertEquals(3, results.size)
        assertTrue(results.all { it is RuntimeOperationResult.Success })
        assertEquals(1, host.stopCallCount)
    }

    @Test
    fun stopEarly_invalidatesGeneration() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val genBefore = controller.snapshot().generation
        stopAndCapture(controller)
        assertTrue(controller.snapshot().generation > genBefore)
    }

    @Test
    fun stop_thenStart_createsNewGeneration() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val genBefore = controller.snapshot().generation
        stopAndCapture(controller)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped(generation = controller.snapshot().generation))
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        assertTrue(controller.snapshot().generation > genBefore)
    }

    @Test
    fun serviceRequestStopFails_transitionsToFailed() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = object : RuntimeServiceHost {
            private val listeners = mutableListOf<RuntimeServiceHostListener>()
            override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Failure(
                com.amitia.amitia_app.runtime.service.RuntimeServiceError(
                    com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                    "stop failed"
                )
            )
            override fun requestTeardownAfterStartupFailure() {}
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
