package com.amitia.amitia_app.runtime.internal

import com.amitia.amitia_app.runtime.api.RuntimeListener
import com.amitia.amitia_app.runtime.api.RuntimeSnapshot
import com.amitia.amitia_app.runtime.api.RuntimeSubscription
import com.amitia.amitia_app.runtime.api.RuntimeState
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.locks.ReentrantReadWriteLock
import kotlin.concurrent.withLock

class RuntimeStateStore(
    private val clock: RuntimeClock = SystemRuntimeClock
) {
    private val lock = ReentrantReadWriteLock()
    private var currentSnapshot: RuntimeSnapshot = RuntimeSnapshot.initial()
    private val listeners = CopyOnWriteArrayList<ListenerEntry>()
    private val closed = AtomicBoolean(false)

    fun snapshot(): RuntimeSnapshot {
        return lock.readLock().withLock {
            currentSnapshot.copy(
                components = currentSnapshot.components.toList()
            )
        }
    }

    fun subscribe(listener: RuntimeListener): RuntimeSubscription {
        val entry = ListenerEntry(listener)
        val snapshotCopy = currentSnapshot.copy(
            components = currentSnapshot.components.toList()
        )
        lock.writeLock().withLock {
            if (closed.get()) {
                return CancelledSubscription
            }
            listeners.add(entry)
        }
        try {
            listener.onRuntimeSnapshotChanged(snapshotCopy)
        } catch (_: Throwable) {
        }
        return RuntimeSubscriptionImpl(entry, this)
    }

    fun update(transform: (RuntimeSnapshot) -> RuntimeSnapshot): RuntimeSnapshot {
        var resultSnapshot: RuntimeSnapshot
        var snapshotToNotify: RuntimeSnapshot? = null

        lock.writeLock().withLock {
            if (closed.get()) {
                return currentSnapshot.copy(
                    components = currentSnapshot.components.toList()
                )
            }
            val oldSnapshot = currentSnapshot
            val newSnapshot = transform(oldSnapshot)

            RuntimeStateMachine.requireTransition(oldSnapshot.state, newSnapshot.state)

            if (oldSnapshot == newSnapshot) {
                return oldSnapshot.copy(
                    components = oldSnapshot.components.toList()
                )
            }

            val resolvedSnapshot = newSnapshot.copy(
                updatedAtEpochMillis = clock.nowEpochMillis()
            )

            currentSnapshot = resolvedSnapshot
            resultSnapshot = resolvedSnapshot
            snapshotToNotify = resolvedSnapshot.copy(
                components = resolvedSnapshot.components.toList()
            )
        }

        val snap = snapshotToNotify
        if (snap != null) {
            val snapshotListeners = ArrayList(listeners)
            for (entry in snapshotListeners) {
                try {
                    if (!entry.cancelled.get()) {
                        entry.listener.onRuntimeSnapshotChanged(snap)
                    }
                } catch (_: Throwable) {
                }
            }
        }

        return resultSnapshot.copy(
            components = resultSnapshot.components.toList()
        )
    }

    fun transitionToStarting(): RuntimeSnapshot {
        var resultSnapshot: RuntimeSnapshot
        var snapshotToNotify: RuntimeSnapshot? = null

        lock.writeLock().withLock {
            if (closed.get()) {
                throw IllegalStateException("state store is closed")
            }

            val oldSnapshot = currentSnapshot

            val stateError = RuntimeStateMachine.requireTransitionTo(oldSnapshot.state, RuntimeState.STARTING)
            if (stateError != null) {
                throw IllegalRuntimeTransitionException(stateError.from, stateError.to)
            }

            if (oldSnapshot.generation == Long.MAX_VALUE) {
                throw IllegalStateException("generation overflow")
            }

            val newGeneration = oldSnapshot.generation + 1

            val newSnapshot = oldSnapshot.copy(
                state = RuntimeState.STARTING,
                generation = newGeneration,
                updatedAtEpochMillis = clock.nowEpochMillis()
            )

            currentSnapshot = newSnapshot
            resultSnapshot = newSnapshot
            snapshotToNotify = newSnapshot.copy(
                components = newSnapshot.components.toList()
            )
        }

        val snap = snapshotToNotify
        if (snap != null) {
            val snapshotListeners = ArrayList(listeners)
            for (entry in snapshotListeners) {
                try {
                    if (!entry.cancelled.get()) {
                        entry.listener.onRuntimeSnapshotChanged(snap)
                    }
                } catch (_: Throwable) {
                }
            }
        }

        return resultSnapshot.copy(
            components = resultSnapshot.components.toList()
        )
    }

    fun close() {
        lock.writeLock().withLock {
            if (closed.compareAndSet(false, true)) {
                listeners.clear()
            }
        }
    }

    private fun removeListener(entry: ListenerEntry) {
        listeners.remove(entry)
    }

    private class ListenerEntry(
        val listener: RuntimeListener,
        val cancelled: AtomicBoolean = AtomicBoolean(false)
    )

    private class RuntimeSubscriptionImpl(
        private val entry: ListenerEntry,
        private val store: RuntimeStateStore
    ) : RuntimeSubscription {
        override fun cancel() {
            entry.cancelled.set(true)
            store.removeListener(entry)
        }

        override fun isCancelled(): Boolean = entry.cancelled.get()
    }

    private object CancelledSubscription : RuntimeSubscription {
        override fun cancel() {}
        override fun isCancelled(): Boolean = true
    }
}
