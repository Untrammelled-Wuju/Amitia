package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicInteger

internal class TeardownTrackingServiceHost : RuntimeServiceHost {
    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    val teardownRequestCount = AtomicInteger(0)
    val ensureStartedGenerations = CopyOnWriteArrayList<Long>()

    override fun ensureStarted(generation: Long): RuntimeServiceResult {
        ensureStartedGenerations.add(generation)
        return RuntimeServiceResult.Success
    }

    override fun requestStop(targetGeneration: Long): RuntimeServiceResult = RuntimeServiceResult.Success

    override fun requestTeardownAfterStartupFailure() {
        teardownRequestCount.incrementAndGet()
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = 0L

    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

class DefaultRuntimeControllerStartupFailureTest {

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

    private fun startAndCapture(controller: DefaultRuntimeController): RuntimeOperationResult {
        val resultRef = java.util.concurrent.atomic.AtomicReference<RuntimeOperationResult>()
        val latch = java.util.concurrent.CountDownLatch(1)
        val callback = object : RuntimeOperationCallback {
            override fun onCompleted(result: RuntimeOperationResult) {
                resultRef.set(result)
                latch.countDown()
            }
        }
        controller.start(RuntimeStartRequest(reason = RuntimeStartReason.USER_REQUEST), callback)
        latch.await(5, java.util.concurrent.TimeUnit.SECONDS)
        return resultRef.get()
    }

    @Test
    fun launchFailure_emittedByService_triggersControllerFailure() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        val result = startAndCapture(controller)
        assertTrue(result is RuntimeOperationResult.Success)

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.LaunchFailed(
            generation = currentGen,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime"
        ))

        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun launchFailure_afterFailure_canRecover() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        val result1 = startAndCapture(controller)
        assertTrue(result1 is RuntimeOperationResult.Success)
        val gen1 = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.LaunchFailed(
            generation = gen1,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime"
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)

        val result2 = startAndCapture(controller)
        assertTrue(result2 is RuntimeOperationResult.Success)
        assertTrue(controller.snapshot().generation > gen1)
    }

    @Test
    fun launchFailure_doesNotRequestTeardownOnServiceHost() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        startAndCapture(controller)
        assertEquals(0, host.teardownRequestCount.get())

        val currentGen = controller.snapshot().generation
        host.emit(RuntimeServiceHostEvent.LaunchFailed(
            generation = currentGen,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime"
        ))

        assertEquals(0, host.teardownRequestCount.get())
    }

    @Test
    fun launchFailure_staleGeneration_ignored() {
        val stateStore = createStoppedStateStore()
        val host = TeardownTrackingServiceHost()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )

        startAndCapture(controller)
        val gen1 = controller.snapshot().generation

        startAndCapture(controller)
        val gen2 = controller.snapshot().generation

        host.emit(RuntimeServiceHostEvent.LaunchFailed(
            generation = gen1,
            cause = com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause.NO_ACTIVE_RUNTIME,
            message = "no active runtime"
        ))

        assertTrue(controller.snapshot().generation == gen2 || controller.snapshot().state == RuntimeState.STOPPED)
    }
}
