package com.amitia.amitia_app.runtime.recovery

import com.amitia.amitia_app.runtime.api.RuntimeErrorCode
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.internal.DefaultRuntimeController
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import com.amitia.amitia_app.runtime.service.RuntimeServiceHost
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

internal class FakeServiceHostNoRecovery : RuntimeServiceHost {
    private val listeners = mutableListOf<com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener>()

    override fun ensureStarted(generation: Long) = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun requestStop() = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun addListener(listener: com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener) {
        listeners.add(listener)
    }
    override fun removeListener(listener: com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener) {
        listeners.remove(listener)
    }
    override fun currentSession(): com.amitia.amitia_app.runtime.proot.ProotSession? = null
    override fun currentGeneration(): Long = 0L
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

class RuntimeCrashRecoveryControllerTest {

    private fun createStateStoreWith(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        val path = when (targetState) {
            RuntimeState.FAILED -> listOf(
                RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.FAILED
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
            else -> listOf()
        }
        for (s in path) {
            store.update { it.copy(state = s) }
        }
        return store
    }

    @Test
    fun unexpectedTermination_fromStarting_transitionsToFailed() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeServiceHostNoRecovery()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun unexpectedTermination_fromReady_transitionsToFailed() {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.INSTALLED) }
        store.update { it.copy(state = RuntimeState.STARTING) }
        store.update { it.copy(state = RuntimeState.READY) }
        val host = FakeServiceHostNoRecovery()
        val controller = DefaultRuntimeController(
            stateStore = store,
            serviceHost = host,
        )
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun unexpectedTermination_setsLastError() {
        val stateStore = createStateStoreWith(RuntimeState.STARTING)
        val host = FakeServiceHostNoRecovery()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        host.emit(RuntimeServiceHostEvent.UnexpectedTermination(
            RuntimeServiceTerminationCause.SESSION_EXITED
        ))
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
        val err = controller.snapshot().lastError
        assertEquals(RuntimeErrorCode.RUNTIME_EXECUTION_NOT_AVAILABLE, err?.code)
    }

    @Test
    fun expectedStopped_fromStopping_transitionsToStopped() {
        val stateStore = createStateStoreWith(RuntimeState.STOPPING)
        val host = FakeServiceHostNoRecovery()
        val controller = DefaultRuntimeController(
            stateStore = stateStore,
            serviceHost = host,
        )
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
        assertNull(controller.snapshot().lastError)
    }
}
