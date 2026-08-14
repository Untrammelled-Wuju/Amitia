package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeState
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
import java.util.concurrent.atomic.AtomicReference

class RuntimeStartupControllerIntegrationTest {

    private fun createAlwaysAliveSession(): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-alive"
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long): ProotStopResult =
            ProotStopResult.AlreadyStopped(sessionId, null)
        override fun close() {}
        override fun requestStop() {}
        override val exit: com.amitia.amitia_app.runtime.proot.ProotExit? = null
    }

    private fun createStoppedStateStore(): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update {
            com.amitia.amitia_app.runtime.api.RuntimeSnapshot(
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
    fun start_withServiceReady_transitionsToStartingAndReturnsSuccess() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestTeardownAfterStartupFailure() {}
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
            override fun currentSession(): ProotSession? = createAlwaysAliveSession()
            override fun currentGeneration(): Long = 1L
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
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
        assertEquals(com.amitia.amitia_app.runtime.api.RuntimeOperationType.START, result?.type)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(RuntimeState.STARTING, stateStore.snapshot().state)
    }

    @Test
    fun start_whenServiceFails_returnsFailure() {
        val stateStore = createStoppedStateStore()
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult = RuntimeServiceResult.Failure(
                com.amitia.amitia_app.runtime.service.RuntimeServiceError(
                    com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode.SERVICE_START_FAILED,
                    "service start unavailable"
                )
            )
            override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestTeardownAfterStartupFailure() {}
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
            override fun currentSession(): ProotSession? = null
            override fun currentGeneration(): Long = 0L
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
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
        assertEquals(RuntimeErrorCode.START_FAILED, failure.error.code)
    }

    @Test
    fun start_whenAlreadyStarting_returnsOperationAlreadyRunning() {
        val stateStore = createStoppedStateStore()
        stateStore.update { it.copy(state = RuntimeState.STARTING) }
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestTeardownAfterStartupFailure() {}
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
            override fun currentSession(): ProotSession? = null
            override fun currentGeneration(): Long = 0L
        }
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = serviceHost,
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
    fun start_fromCorrupted_returnsInvalidState() {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.UNKNOWN) }
        store.update { it.copy(state = RuntimeState.CORRUPTED) }
        val serviceHost = object : RuntimeServiceHost {
            override fun ensureStarted(generation: Long, profile: String): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success
            override fun requestTeardownAfterStartupFailure() {}
            override fun addListener(listener: RuntimeServiceHostListener) {}
            override fun removeListener(listener: RuntimeServiceHostListener) {}
            override fun currentSession(): ProotSession? = null
            override fun currentGeneration(): Long = 0L
        }
        val controller = DefaultRuntimeController(
            stateStore = store,
            serviceHost = serviceHost,
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
        assertEquals(RuntimeErrorCode.INVALID_STATE, failure.error.code)
    }
}
