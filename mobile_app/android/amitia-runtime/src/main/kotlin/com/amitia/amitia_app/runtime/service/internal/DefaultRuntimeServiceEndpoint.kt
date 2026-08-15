package com.amitia.amitia_app.runtime.service.internal

import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceSnapshot
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicReference

internal class DefaultRuntimeServiceEndpoint(
    private val serviceProvider: () -> RuntimeService?
) : RuntimeServiceEndpoint {

    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()
    private val lastEventRef = AtomicReference<RuntimeServiceHostEvent?>(null)

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
        val snapshot = ArrayList(listeners)
        for (listener in snapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (_: Throwable) {
            }
        }
    }

    fun lastEvent(): RuntimeServiceHostEvent? = lastEventRef.get()
}
