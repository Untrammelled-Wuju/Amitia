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
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotComponent
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import com.amitia.amitia_app.runtime.proot.ProotLaunchRequest
import com.amitia.amitia_app.runtime.proot.ProotObserver
import com.amitia.amitia_app.runtime.proot.ProotSession
import com.amitia.amitia_app.runtime.proot.ProotStopResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceError
import com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.startup.RuntimeStartupDetector
import com.amitia.amitia_app.runtime.startup.RuntimeStartupRequest
import com.amitia.amitia_app.runtime.startup.RuntimeStartupResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

internal class FakeRuntimeServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    var stopRequested = false
    var stopCallCount = 0

    override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
    override fun requestStop(): RuntimeServiceResult {
        stopCallCount++
        stopRequested = true
        return RuntimeServiceResult.Success
    }
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

internal class FakeProotSession(
    override val sessionId: String = "fake-session",
    private val stopResult: ProotStopResult? = null
) : ProotSession {
    var stopCallCount = 0
    @Volatile private var alive = true

    override fun isAlive(): Boolean = alive
    override fun awaitExit(timeoutMillis: Long): Int? = if (!alive) 0 else null
    override fun stop(graceMillis: Long): ProotStopResult {
        stopCallCount++
        alive = false
        return stopResult ?: ProotStopResult.Graceful(sessionId, 0)
    }
    override fun close() { alive = false }
}

internal class FakeProotComponent(
    private var mainSession: ProotSession? = null,
    private val componentStopResult: ProotStopResult? = null
) : ProotComponent {
    var stopCallCount = 0
    var currentSessionCallCount = 0

    override fun availability(): ProotAvailability = ProotAvailability.Unavailable(
        ProotErrorCode.BINARY_NOT_FOUND, "test"
    )
    override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession {
        val s = mainSession ?: FakeProotSession()
        mainSession = s
        return s
    }
    override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = mainSession ?: FakeProotSession()
    override fun currentSession(): ProotSession? {
        currentSessionCallCount++
        return mainSession
    }
    override fun stop(): ProotStopResult {
        stopCallCount++
        val s = mainSession
        val result = componentStopResult ?: s?.stop(10_000) ?: ProotStopResult.AlreadyStopped("none", null)
        mainSession = null
        return result
    }
    override fun close() { mainSession = null }
    fun setSession(s: ProotSession?) { mainSession = s }
}

internal class FakeStartupDetector : RuntimeStartupDetector {
    var cancelCallCount = 0
    var cancelled = false
    private var resultToReturn: RuntimeStartupResult? = null

    fun setResult(result: RuntimeStartupResult) { resultToReturn = result }

    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        return resultToReturn ?: RuntimeStartupResult.Cancelled(request.generation)
    }
    override fun cancel() {
        cancelCallCount++
        cancelled = true
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
        val proot = FakeProotComponent()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(0, proot.stopCallCount)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun stopping_stop_returnsAlreadyInProgress() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.STOP_ALREADY_IN_PROGRESS, (result as RuntimeOperationResult.Failure).error.code)
        assertEquals(0, proot.stopCallCount)
    }

    @Test
    fun ready_stop_transitionsToStopping_thenStopped() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent(FakeProotSession())
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
        assertEquals(1, proot.stopCallCount)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun starting_stop_cancelsDetectorAndStopsSession() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent(FakeProotSession())
        val detector = FakeStartupDetector()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = detector
        )
        val resultRef = AtomicReference<RuntimeOperationResult>()
        val errorRef = AtomicReference<String>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                errorRef.set(result.toString())
                latch.countDown()
            }
        }
        controller.stop(RuntimeStopRequest(reason = RuntimeStopReason.USER_REQUEST, force = false), callback)
        latch.await(5, TimeUnit.SECONDS)
        val result = resultRef.get()
        val errorInfo = errorRef.get()
        assertTrue("Result was: $result, info: $errorInfo", result is RuntimeOperationResult.Success)
        assertTrue(detector.cancelled)
        assertEquals(1, detector.cancelCallCount)
        assertEquals(1, proot.stopCallCount)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun degraded_stop_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.DEGRADED)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent(FakeProotSession())
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun failed_stop_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.FAILED)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent(FakeProotSession())
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun invalidStateInstalled_stop_returnsInvalidState() {
        val stateStore = createStateStoreWith(RuntimeState.INSTALLED)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.INVALID_STATE, (result as RuntimeOperationResult.Failure).error.code)
    }

    @Test
    fun sessionStopFailed_transitionsToFailed() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val failResult = ProotStopResult.Failed(
            "test-session",
            ProotErrorCode.PROCESS_STOP_FAILED,
            "stop failed"
        )
        val proot = FakeProotComponent(FakeProotSession(), failResult)
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        assertNotNull(controller.snapshot().lastError)
    }

    @Test
    fun sessionStopForce_succeedsTransitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val forceResult = ProotStopResult.Forced("test-session", 1)
        val proot = FakeProotComponent(FakeProotSession(), forceResult)
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun sessionAlreadyStopped_transitionsToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val alreadyStoppedResult = ProotStopResult.AlreadyStopped("test-session", 0)
        val proot = FakeProotComponent(FakeProotSession(), alreadyStoppedResult)
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun duplicateExpectedStopped_onlyOneStateTransition() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val proot = FakeProotComponent(FakeProotSession())
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
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
        val proot = FakeProotComponent()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
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
                RuntimeServiceError(
                    RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                    "stop failed"
                )
            )
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val proot = FakeProotComponent(FakeProotSession())
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(RuntimeErrorCode.STOP_SERVICE_TEARDOWN_FAILED, (result as RuntimeOperationResult.Failure).error.code)
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun stop_noProotComponent_goesToStopping() {
        val stateStore = createStateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = null,
            startupDetector = FakeStartupDetector()
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STOPPING, controller.snapshot().state)
    }

    @Test
    fun stop_cancelsDetectorBeforeSessionStop() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val detector = FakeStartupDetector()
        val sequence = mutableListOf<String>()
        val trackingSession = object : ProotSession {
            override val sessionId = "tracking-session"
            override fun isAlive(): Boolean = true
            override fun awaitExit(timeoutMillis: Long): Int? = 0
            override fun stop(graceMillis: Long): ProotStopResult {
                sequence.add("sessionStop")
                return ProotStopResult.Graceful(sessionId, 0)
            }
            override fun close() {}
        }
        val proot = object : ProotComponent {
            override fun availability(): ProotAvailability = ProotAvailability.Unavailable(
                ProotErrorCode.BINARY_NOT_FOUND, "test"
            )
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = trackingSession
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = trackingSession
            override fun currentSession(): ProotSession? = trackingSession
            override fun stop(): ProotStopResult {
                sequence.add("componentStop")
                return trackingSession.stop(10_000)
            }
            override fun close() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
            prootComponent = proot,
            startupDetector = detector
        )
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
        assertTrue(detector.cancelled)
    }
}
