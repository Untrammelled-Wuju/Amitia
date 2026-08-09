package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeOperationType
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
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
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class RuntimeStartupControllerIntegrationTest {

    private fun createAlwaysAliveSession(): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-alive"
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long): ProotStopResult =
            ProotStopResult.AlreadyStopped(sessionId, null)
        override fun close() {}
    }

    private fun createStoppedStateStore(): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update {
            RuntimeSnapshot(
                state = RuntimeState.STOPPED,
                runtimeVersion = null,
                activeOperationId = null,
                activeOperationType = null,
                progress = it.progress,
                components = it.components,
                lastError = null,
                generation = it.generation,
                updatedAtEpochMillis = it.updatedAtEpochMillis
            )
        }
        return store
    }

    @Test
    fun start_withProotComponent_transitionsToStartingAndReturnsSuccess() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val session = createAlwaysAliveSession()
        val prootComponent = object : ProotComponent {
            override fun availability(): ProotAvailability =
                ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "test")
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun currentSession(): ProotSession? = session
            override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("test", null)
            override fun close() {}
        }
        val startupDetector = DefaultRuntimeStartupDetector(
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L
        )
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
            startupDetector = startupDetector
        )
        val callbackRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                callbackRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(2, TimeUnit.SECONDS)
        val result = callbackRef.get()
        assertNotNull(result)
        assertEquals(RuntimeOperationType.START, result?.type)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STARTING, stateStore.snapshot().state)
    }

    @Test
    fun start_withoutProotComponent_returnsFailure() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = null,
            startupDetector = null
        )
        val callbackRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                callbackRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(2, TimeUnit.SECONDS)
        val result = callbackRef.get()
        assertNotNull(result)
        assertTrue(result is RuntimeOperationResult.Failure)
        val failure = result as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE, failure.error.code)
    }

    @Test
    fun stop_cancelsStartupDetector() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val session = createAlwaysAliveSession()
        val prootComponent = object : ProotComponent {
            override fun availability(): ProotAvailability =
                ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "test")
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun currentSession(): ProotSession? = session
            override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("test", null)
            override fun close() {}
        }
        val cancelCount = AtomicInteger(0)
        val startupDetector = object : RuntimeStartupDetector {
            @Volatile private var cancelled = false
            override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
                while (!cancelled) {
                    Thread.sleep(50)
                }
                return RuntimeStartupResult.Cancelled(request.generation)
            }
            override fun cancel() {
                cancelled = true
                cancelCount.incrementAndGet()
            }
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
            startupDetector = startupDetector
        )
        val startLatch = CountDownLatch(1)
        controller.start(
            RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    startLatch.countDown()
                }
            }
        )
        startLatch.await(2, TimeUnit.SECONDS)

        val stopLatch = CountDownLatch(1)
        controller.stop(
            RuntimeStopRequest(reason = RuntimeStopReason.USER_REQUEST, force = false),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    stopLatch.countDown()
                }
            }
        )
        stopLatch.await(2, TimeUnit.SECONDS)
        assertTrue(cancelCount.get() > 0)
    }

    @Test
    fun start_whenAlreadyStarting_returnsOperationAlreadyRunning() {
        val stateStore = createStoppedStateStore()
        stateStore.update { it.copy(state = RuntimeState.STARTING) }
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = null,
            startupDetector = null
        )
        val callbackRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                callbackRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(2, TimeUnit.SECONDS)
        val result = callbackRef.get()
        assertNotNull(result)
        assertTrue(result is RuntimeOperationResult.Failure)
        val failure = result as RuntimeOperationResult.Failure
        assertEquals(RuntimeErrorCode.OPERATION_ALREADY_RUNNING, failure.error.code)
    }

    @Test
    fun start_withNullSession_returnsFailure() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val prootComponent = object : ProotComponent {
            override fun availability(): ProotAvailability =
                ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "test")
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = createAlwaysAliveSession()
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = createAlwaysAliveSession()
            override fun currentSession(): ProotSession? = null
            override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("test", null)
            override fun close() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
            startupDetector = DefaultRuntimeStartupDetector()
        )
        val callbackRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                callbackRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(2, TimeUnit.SECONDS)
        val result = callbackRef.get()
        assertNotNull(result)
        assertTrue(result is RuntimeOperationResult.Failure)
    }

    @Test
    fun duplicateDetector_onlyOneDetectorPerGeneration() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
        }
        val session = createAlwaysAliveSession()
        val prootComponent = object : ProotComponent {
            override fun availability(): ProotAvailability =
                ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "test")
            override fun launch(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun launchProbe(request: ProotLaunchRequest, observer: ProotObserver): ProotSession = session
            override fun currentSession(): ProotSession? = session
            override fun stop(): ProotStopResult = ProotStopResult.AlreadyStopped("test", null)
            override fun close() {}
        }
        val detectorCount = AtomicInteger(0)
        val startupDetector = object : RuntimeStartupDetector {
            private val detectorId = detectorCount.incrementAndGet()
            override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
                Thread.sleep(1000)
                return RuntimeStartupResult.Ready(request.generation)
            }
            override fun cancel() {}
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
            prootComponent = prootComponent,
            startupDetector = startupDetector
        )
        val startLatch = CountDownLatch(1)
        controller.start(
            RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST),
            object : RuntimeOperationCallback {
                override fun onCompleted(result: RuntimeOperationResult) {
                    startLatch.countDown()
                }
            }
        )
        startLatch.await(2, TimeUnit.SECONDS)
        assertEquals(1, detectorCount.get())
    }
}
