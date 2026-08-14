package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class RuntimeStateStoreInitializeTest {

    @Test
    fun initialize_fromUnknown_setsState() {
        val store = RuntimeStateStore()

        val result = store.initialize(RuntimeState.NOT_INSTALLED)

        assertEquals(RuntimeState.NOT_INSTALLED, result.state)
        assertEquals(RuntimeState.NOT_INSTALLED, store.snapshot().state)
    }

    @Test
    fun initialize_fromUnknown_withVersion_setsVersion() {
        val store = RuntimeStateStore()

        val result = store.initialize(RuntimeState.STOPPED, "1.0.0")

        assertEquals(RuntimeState.STOPPED, result.state)
        assertEquals("1.0.0", result.runtimeVersion)
        assertEquals("1.0.0", store.snapshot().runtimeVersion)
    }

    @Test
    fun initialize_doesNotIncrementGeneration() {
        val store = RuntimeStateStore()

        val result = store.initialize(RuntimeState.NOT_INSTALLED)

        assertEquals(0, result.generation)
        assertEquals(0, store.snapshot().generation)
    }

    @Test
    fun initialize_canOnlyBeCalledOnce() {
        val store = RuntimeStateStore()

        store.initialize(RuntimeState.NOT_INSTALLED)

        assertThrows(IllegalStateException::class.java) {
            store.initialize(RuntimeState.STOPPED)
        }
    }

    @Test
    fun initialize_failsFromNonUnknownState() {
        val store = RuntimeStateStore()
        store.update { it.copy(state = RuntimeState.STOPPED) }

        assertThrows(IllegalStateException::class.java) {
            store.initialize(RuntimeState.NOT_INSTALLED)
        }
    }

    @Test
    fun isInitialized_returnsFalseInitially() {
        val store = RuntimeStateStore()

        assertFalse(store.isInitialized())
    }

    @Test
    fun isInitialized_returnsTrueAfterInitialize() {
        val store = RuntimeStateStore()

        store.initialize(RuntimeState.NOT_INSTALLED)

        assertTrue(store.isInitialized())
    }

    @Test
    fun initialize_notifiesListeners() {
        val store = RuntimeStateStore()
        val received = mutableListOf<RuntimeState>()

        store.subscribe { received.add(it.state) }
        received.clear()

        store.initialize(RuntimeState.STOPPED, "1.0.0")

        assertEquals(1, received.size)
        assertEquals(RuntimeState.STOPPED, received[0])
    }
}
