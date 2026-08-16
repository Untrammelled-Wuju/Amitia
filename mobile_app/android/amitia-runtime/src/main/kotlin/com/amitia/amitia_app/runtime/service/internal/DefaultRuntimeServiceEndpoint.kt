package com.amitia.amitia_app.runtime.service.internal

import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceSnapshot
import com.amitia.amitia_app.runtime.service.RuntimeServiceLifecycleSnapshot
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeServiceEndpoint(
    private val serviceProvider: () -> RuntimeService?
) : RuntimeServiceEndpoint {

    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val lastEventRef = AtomicReference<RuntimeServiceHostEvent?>(null)
    private val fullSnapshotRef = AtomicReference<List<RuntimeServiceHostEvent>?>(null)

    override fun snapshot(): RuntimeServiceSnapshot {
        val service = serviceProvider()
        return service?.snapshot() ?: RuntimeServiceSnapshot(
            created = false, foreground = false, boundClients = 0
        )
    }

    override fun currentSnapshot(): RuntimeServiceSnapshot = snapshot()

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    fun notify(event: RuntimeServiceHostEvent) {
        lastEventRef.set(event)
        updateFullSnapshot(event)
        val snapshot = ArrayList(listeners)
        for (listener in snapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (_: Throwable) {
            }
        }
    }

    fun lastEvent(): RuntimeServiceHostEvent? = lastEventRef.get()

    fun fullSnapshot(): List<RuntimeServiceHostEvent>? = fullSnapshotRef.get()

    private fun updateFullSnapshot(event: RuntimeServiceHostEvent) {
        val current = fullSnapshotRef.get() ?: emptyList()
        val updated = current + event
        fullSnapshotRef.set(updated)
    }

    fun updateLifecycleSnapshot(lifecycleSnapshot: RuntimeServiceLifecycleSnapshot) {
    }
}
