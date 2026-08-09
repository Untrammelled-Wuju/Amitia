package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceTerminationCause
import com.amitia.amitia_app.runtime.api.RuntimeState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

internal class FakeRuntimeServiceHost : com.amitia.amitia_app.runtime.service.RuntimeServiceHost {
    private val listeners = mutableListOf<RuntimeServiceHostListener>()
    override fun ensureStarted() = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun requestStop() = com.amitia.amitia_app.runtime.service.RuntimeServiceResult.Success
    override fun addListener(listener: RuntimeServiceHostListener) { listeners.add(listener) }
    override fun removeListener(listener: RuntimeServiceHostListener) { listeners.remove(listener) }
    fun emit(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        snapshot.forEach { it.onServiceHostEvent(event) }
    }
}

class DefaultRuntimeControllerEventTest {

    private fun stateStoreWith(targetState: RuntimeState): RuntimeStateStore {
        val store = RuntimeStateStore()
        val path = when (targetState) {
            RuntimeState.READY -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.READY)
            RuntimeState.STARTING -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING)
            RuntimeState.STOPPING -> listOf(RuntimeState.INSTALLED, RuntimeState.STARTING, RuntimeState.STOPPING)
            else -> listOf()
        }
        for (s in path) {
            store.update { it.copy(state = s) }
        }
        return store
    }

    @Test
    fun controller_receivesUnexpectedTermination_fromReadyGoesToFailed() {
        val stateStore = stateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(stateStore = stateStore, serviceHost = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                RuntimeServiceTerminationCause.FOREGROUND_FAILED
            )
        )
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun controller_doesNotAutoRestart_fromStartingGoesToFailed() {
        val stateStore = stateStoreWith(RuntimeState.STARTING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(stateStore = stateStore, serviceHost = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                RuntimeServiceTerminationCause.SERVICE_INTERNAL_ERROR
            )
        )
        assertEquals(RuntimeState.FAILED, controller.snapshot().state)
    }

    @Test
    fun expectedStop_fromStoppingGoesToStopped() {
        val stateStore = stateStoreWith(RuntimeState.STOPPING)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(stateStore = stateStore, serviceHost = host)
        host.emit(RuntimeServiceHostEvent.ExpectedStopped)
        assertEquals(RuntimeState.STOPPED, controller.snapshot().state)
    }

    @Test
    fun unexpectedTermination_setsLastError() {
        val stateStore = stateStoreWith(RuntimeState.READY)
        val host = FakeRuntimeServiceHost()
        val controller = DefaultRuntimeController(stateStore = stateStore, serviceHost = host)
        host.emit(
            RuntimeServiceHostEvent.UnexpectedTermination(
                RuntimeServiceTerminationCause.NOTIFICATION_FAILED
            )
        )
        assertTrue(controller.snapshot().lastError != null)
    }

    @Test
    fun addListener_registersMultipleControllers() {
        val host = FakeRuntimeServiceHost()
        val stateStore1 = RuntimeStateStore()
        val stateStore2 = RuntimeStateStore()
        DefaultRuntimeController(stateStore = stateStore1, serviceHost = host)
        DefaultRuntimeController(stateStore = stateStore2, serviceHost = host)
        assertEquals(2, host.listenersSize())
    }

    private fun FakeRuntimeServiceHost.listenersSize(): Int {
        val field = this::class.java.getDeclaredField("listeners")
        field.isAccessible = true
        @Suppress("UNCHECKED_CAST")
        return (field.get(this) as MutableList<*>).size
    }
}
