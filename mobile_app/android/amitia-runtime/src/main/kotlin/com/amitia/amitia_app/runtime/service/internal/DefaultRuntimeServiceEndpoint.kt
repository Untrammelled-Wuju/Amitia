package com.amitia.amitia_app.runtime.service.internal

import com.amitia.amitia_app.runtime.service.RuntimeService
import com.amitia.amitia_app.runtime.service.RuntimeServiceEndpoint
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostEvent
import com.amitia.amitia_app.runtime.service.RuntimeServiceHostListener
import com.amitia.amitia_app.runtime.service.RuntimeServiceSnapshot
import java.util.concurrent.CopyOnWriteArrayList

internal class DefaultRuntimeServiceEndpoint(
    private val serviceProvider: () -> RuntimeService?
) : RuntimeServiceEndpoint {

    private val listeners = CopyOnWriteArrayList<RuntimeServiceHostListener>()

    override fun snapshot(): RuntimeServiceSnapshot {
        val service = serviceProvider()
        return service?.snapshot() ?: RuntimeServiceSnapshot(
            created = false, foreground = false, boundClients = 0
        )
    }

    override fun addListener(listener: RuntimeServiceHostListener) {
        listeners.add(listener)
    }

    override fun removeListener(listener: RuntimeServiceHostListener) {
        listeners.remove(listener)
    }

    fun notify(event: RuntimeServiceHostEvent) {
        val snapshot = ArrayList(listeners)
        for (listener in snapshot) {
            try {
                listener.onServiceHostEvent(event)
            } catch (_: Throwable) {
            }
        }
    }
}
