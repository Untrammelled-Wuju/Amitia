package com.amitia.amitia_app.nativeprovider.notification

import android.service.notification.NotificationListenerService
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal object NotificationServiceRegistry {

    private val current = AtomicReference<NotificationListenerService?>(null)
    private val generationRef = AtomicLong(0L)

    fun attach(service: NotificationListenerService) {
        current.set(service)
        generationRef.incrementAndGet()
    }

    fun detach(service: NotificationListenerService) {
        current.compareAndSet(service, null)
    }

    fun current(): NotificationListenerService? = current.get()

    fun generation(): Long = generationRef.get()

    fun markConnected() {
        generationRef.incrementAndGet()
    }

    fun isServiceAttached(): Boolean = current.get() != null
}
