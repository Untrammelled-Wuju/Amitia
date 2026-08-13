package com.amitia.amitia_app.runtime

import com.amitia.amitia_app.runtime.api.RuntimeComponentSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeComponentState
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeState
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.internal.IllegalRuntimeTransitionException
import com.amitia.amitia_app.runtime.internal.RuntimeClock
import com.amitia.amitia_app.runtime.internal.RuntimeStateStore
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotSame
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class RuntimeStateStoreTest {

    private class FakeClock(private var time: Long = 0L) : RuntimeClock {
        override fun nowEpochMillis(): Long = time
    }

    @Test
    fun initial_snapshot_isUnknown() {
        val store = RuntimeStateStore(FakeClock(0))
        val snapshot = store.snapshot()
        assertEquals(RuntimeState.UNKNOWN, snapshot.state)
        assertEquals(0, snapshot.generation)
    }

    @Test
    fun subscribe_immediatelyReceives_currentSnapshot() {
        val store = RuntimeStateStore(FakeClock(0))
        val received = AtomicReference<RuntimeSnapshot?>()
        received.set(null)
        store.subscribe { received.set(it) }
        assertEquals(RuntimeState.UNKNOWN, received.get()!!.state)
    }

    @Test
    fun legalUpdate_notifiesListener() {
        val clock = FakeClock(1000)
        val store = RuntimeStateStore(clock)
        val received = mutableListOf<RuntimeSnapshot>()
        store.subscribe { received.add(it) }
        received.clear()

        store.update { it.copy(state = RuntimeState.NOT_INSTALLED) }

        assertEquals(1, received.size)
        assertEquals(RuntimeState.NOT_INSTALLED, received[0].state)
    }

    @Test
    fun idempotentUpdate_doesNotNotify() {
        val store = RuntimeStateStore(FakeClock(0))
        val count = AtomicInteger(0)
        store.subscribe { count.incrementAndGet() }
        count.set(0)

        store.update { it }

        assertEquals(0, count.get())
    }

    @Test
    fun realUpdate_generationUnchangedWithoutExplicitSet() {
        val store = RuntimeStateStore(FakeClock(1000))
        val initialGen = store.snapshot().generation

        store.update { it.copy(state = RuntimeState.INSTALLED) }

        val newGen = store.snapshot().generation
        assertEquals(initialGen, newGen)
    }

    @Test
    fun transitionToStarting_generatesNewGeneration() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update { it.copy(state = RuntimeState.INSTALLED, generation = 5) }

        val result = store.transitionToStarting()

        assertEquals(RuntimeState.STARTING, result.state)
        assertEquals(6, result.generation)
        assertEquals(6, store.snapshot().generation)
    }

    @Test
    fun transitionFromStopped_generatesFirstGeneration() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update { it.copy(state = RuntimeState.INSTALLED, generation = 0) }
        store.update { it.copy(state = RuntimeState.STOPPED) }

        val result = store.transitionToStarting()

        assertEquals(RuntimeState.STARTING, result.state)
        assertEquals(1, result.generation)
    }

    @Test
    fun transitionToStarting_fromReady_fails() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update { it.copy(state = RuntimeState.INSTALLED, generation = 1) }
        store.update { it.copy(state = RuntimeState.STARTING, generation = 2) }
        store.update { it.copy(state = RuntimeState.READY) }

        try {
            store.transitionToStarting()
            throw AssertionError("should have thrown")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }

    @Test
    fun transitionToStarting_fromStarting_fails() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update { it.copy(state = RuntimeState.INSTALLED, generation = 1) }
        store.transitionToStarting()

        try {
            store.transitionToStarting()
            throw AssertionError("should have thrown")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }

    @Test
    fun idempotentUpdate_generationUnchanged() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update { it.copy(state = RuntimeState.INSTALLED) }
        val genAfterFirst = store.snapshot().generation

        store.update { it.copy(state = RuntimeState.INSTALLED) }

        assertEquals(genAfterFirst, store.snapshot().generation)
    }

    @Test
    fun time_fromFakeClock() {
        val clock = FakeClock(12345)
        val store = RuntimeStateStore(clock)

        store.update { it.copy(state = RuntimeState.INSTALLED) }

        assertEquals(12345L, store.snapshot().updatedAtEpochMillis)
    }

    @Test
    fun listener_cancel_stopsNotification() {
        val store = RuntimeStateStore(FakeClock(0))
        val count = AtomicInteger(0)
        val sub: RuntimeSubscription = store.subscribe { count.incrementAndGet() }
        count.set(0)

        sub.cancel()
        store.update { it.copy(state = RuntimeState.NOT_INSTALLED) }

        assertEquals(0, count.get())
    }

    @Test
    fun listener_exception_doesNotAffectOthers() {
        val store = RuntimeStateStore(FakeClock(1000))
        val received = mutableListOf<RuntimeSnapshot>()
        store.subscribe { throw RuntimeException("boom") }
        store.subscribe { received.add(it) }
        received.clear()

        store.update { it.copy(state = RuntimeState.INSTALLED) }

        assertEquals(1, received.size)
        assertEquals(RuntimeState.INSTALLED, received[0].state)
    }

    @Test
    fun snapshot_returnsImmutableCopyOfComponents() {
        val store = RuntimeStateStore(FakeClock(1000))
        store.update {
            it.copy(
                components = listOf(
                    RuntimeComponentSnapshot(
                        id = "runtime.package",
                        state = RuntimeComponentState.INSTALLED,
                        required = true,
                        version = "1.0.0",
                        errorCode = null,
                        updatedAtEpochMillis = 1000L
                    ),
                    RuntimeComponentSnapshot(
                        id = "backend.go",
                        state = RuntimeComponentState.INSTALLED,
                        required = true,
                        version = null,
                        errorCode = null,
                        updatedAtEpochMillis = 1000L
                    )
                )
            )
        }
        val snapshot1 = store.snapshot()
        val snapshot2 = store.snapshot()
        assertNotSame("components should be separate copies", snapshot1.components, snapshot2.components)
        assertEquals(snapshot1.components.size, snapshot2.components.size)
    }

    @Test
    fun concurrentReads_areSafe() {
        val store = RuntimeStateStore(FakeClock(0))
        val executor = Executors.newFixedThreadPool(4)
        try {
            val latch = CountDownLatch(4)
            val errors = AtomicInteger(0)
            repeat(4) {
                executor.submit {
                    try {
                        repeat(100) { store.snapshot() }
                    } catch (e: Exception) {
                        errors.incrementAndGet()
                    } finally {
                        latch.countDown()
                    }
                }
            }
            assertTrue(latch.await(10, TimeUnit.SECONDS))
            assertEquals(0, errors.get())
        } finally {
            executor.shutdownNow()
        }
    }

    @Test
    fun concurrentSubscriptions_areSafe() {
        val store = RuntimeStateStore(FakeClock(0))
        val executor = Executors.newFixedThreadPool(4)
        try {
            val latch = CountDownLatch(4)
            val errors = AtomicInteger(0)
            repeat(4) {
                executor.submit {
                    try {
                        repeat(50) {
                            val sub = store.subscribe { }
                            sub.cancel()
                        }
                    } catch (e: Exception) {
                        errors.incrementAndGet()
                    } finally {
                        latch.countDown()
                    }
                }
            }
            assertTrue(latch.await(10, TimeUnit.SECONDS))
            assertEquals(0, errors.get())
        } finally {
            executor.shutdownNow()
        }
    }

    @Test
    fun concurrentCancels_areSafe() {
        val store = RuntimeStateStore(FakeClock(0))
        val subs = (0 until 10).map { store.subscribe { } }
        val executor = Executors.newFixedThreadPool(4)
        try {
            val latch = CountDownLatch(subs.size)
            val errors = AtomicInteger(0)
            for (sub in subs) {
                executor.submit {
                    try {
                        sub.cancel()
                    } catch (e: Exception) {
                        errors.incrementAndGet()
                    } finally {
                        latch.countDown()
                    }
                }
            }
            assertTrue(latch.await(10, TimeUnit.SECONDS))
            assertEquals(0, errors.get())
        } finally {
            executor.shutdownNow()
        }
    }

    @Test
    fun close_isIdempotent() {
        val store = RuntimeStateStore(FakeClock(0))
        store.close()
        store.close()
    }

    @Test
    fun close_stopsNotifications() {
        val store = RuntimeStateStore(FakeClock(0))
        val count = AtomicInteger(0)
        store.subscribe { count.incrementAndGet() }
        count.set(0)

        store.close()
        store.update { it.copy(state = RuntimeState.INSTALLED) }

        assertEquals(0, count.get())
    }

    @Test
    fun close_returnsCancelledSubscription() {
        val store = RuntimeStateStore(FakeClock(0))
        store.close()
        val sub = store.subscribe { }
        assertTrue(sub.isCancelled())
    }

    @Test
    fun illegalTransition_rejected() {
        val store = RuntimeStateStore(FakeClock(0))
        try {
            store.update { it.copy(state = RuntimeState.READY) }
            throw AssertionError("should have thrown")
        } catch (_: IllegalRuntimeTransitionException) {
        }
    }
}
