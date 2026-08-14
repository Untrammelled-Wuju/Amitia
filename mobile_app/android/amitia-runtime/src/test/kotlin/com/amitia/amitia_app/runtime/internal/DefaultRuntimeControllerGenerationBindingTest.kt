package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeStopReason
import com.amitia.amitia_app.runtime.api.RuntimeStopRequest
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal class GenerationTrackingServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    val stopGenerations = mutableListOf<Long>()
    var ensureStartCount = 0
    var activeGeneration = 0L

    override fun ensureStarted(generation: Long): RuntimeServiceResult {
        ensureStartCount++
        activeGeneration = generation
        return RuntimeServiceResult.Success
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        stopGenerations.add(targetGeneration)
        return RuntimeServiceResult.Success
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = activeGeneration
}

internal class FailingStopServiceHost : RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    val stopGenerations = mutableListOf<Long>()
    var ensureStartCount = 0

    override fun ensureStarted(generation: Long): RuntimeServiceResult {
        ensureStartCount++
        return RuntimeServiceResult.Success
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult {
        stopGenerations.add(targetGeneration)
        return RuntimeServiceResult.Failure(
            com.amitia.amitia_app.runtime.service.RuntimeServiceError(
                com.amitia.amitia_app.runtime.service.RuntimeServiceErrorCode.SERVICE_STOP_FAILED,
                "stop failed"
            )
        )
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = 0L
}

class DefaultRuntimeControllerGenerationBindingTest {

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

    private fun createReadyStateStore(generation: Long): RuntimeStateStore {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.INSTALLED) }
        store.update { it.copy(state = RuntimeState.STARTING) }
        store.update { it.copy(state = RuntimeState.READY, generation = generation) }
        return store
    }

    private fun stopAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
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

    private fun startAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
        val resultRef = AtomicReference<RuntimeOperationResult>()
        val latch = CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(5, TimeUnit.SECONDS)
        return resultRef.get()
    }

    @Test
    fun stop_passesCurrentGenerationToServiceHost() {
        val stateStore = createReadyStateStore(generation = 42L)
        val host = GenerationTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(1, host.stopGenerations.size)
        assertEquals(42L, host.stopGenerations[0])
    }

    @Test
    fun stopFails_clearsExpectedStopContext() {
        val stateStore = createReadyStateStore(generation = 50L)
        val host = FailingStopServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Failure)
        assertEquals(1, host.stopGenerations.size)
        assertEquals(50L, host.stopGenerations[0])
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun startAfterStop_clearsExpectedStopContext() {
        val stateStore = createReadyStateStore(generation = 60L)
        val host = GenerationTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        stopAndCapture(controller)
        assertEquals(1, host.stopGenerations.size)
        assertEquals(60L, host.stopGenerations[0])

        startAndCapture(controller)
        assertTrue(controller.snapshot().generation > 60L)

        stopAndCapture(controller)
        assertEquals(2, host.stopGenerations.size)
        assertTrue(host.stopGenerations[1] > 60L)
    }

    @Test
    fun stopInStartingState_passesCorrectGeneration() {
        val stateStore = RuntimeStateStore()
        stateStore.update { it.copy(state = RuntimeState.INSTALLED) }
        stateStore.update { it.copy(state = RuntimeState.STARTING, generation = 70L) }
        val host = GenerationTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(1, host.stopGenerations.size)
        assertEquals(70L, host.stopGenerations[0])
    }

    @Test
    fun stopInFailedState_passesCorrectGeneration() {
        val stateStore = RuntimeStateStore()
        stateStore.update { it.copy(state = RuntimeState.INSTALLED) }
        stateStore.update { it.copy(state = RuntimeState.STARTING) }
        stateStore.update { it.copy(state = RuntimeState.FAILED, generation = 80L) }
        val host = GenerationTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = stopAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)
        assertEquals(1, host.stopGenerations.size)
        assertEquals(80L, host.stopGenerations[0])
    }
}
