package com.amitia.amitia_app.nativeprovider.accessibility

import android.accessibilityservice.AccessibilityService
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal object AccessibilityServiceRegistry {

    private val current = AtomicReference<AccessibilityService?>(null)
    private val generationRef = AtomicLong(0L)

    fun attach(service: AccessibilityService) {
        current.set(service)
        generationRef.incrementAndGet()
    }

    fun detach(service: AccessibilityService) {
        current.compareAndSet(service, null)
    }

    fun current(): AccessibilityService? = current.get()

    fun generation(): Long = generationRef.get()

    fun isServiceConnected(): Boolean = current.get() != null
}
