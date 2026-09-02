package com.amitia.amitia_app.nativeprovider.accessibility

import android.accessibilityservice.AccessibilityService
import com.amitia.amitia_app.nativeprovider.uitree.AccessibilityNodeReferenceRegistry
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal object AccessibilityServiceRegistry {

    private val current = AtomicReference<AccessibilityService?>(null)
    private val generationRef = AtomicLong(0L)

    fun attach(service: AccessibilityService) {
        current.set(service)
        AccessibilityNodeReferenceRegistry.invalidateAll()
        val healthGeneration = AccessibilityHealthMonitor.onConnected(service)
        generationRef.set(maxOf(generationRef.incrementAndGet(), healthGeneration))
    }

    fun detach(service: AccessibilityService) {
        if (current.compareAndSet(service, null)) {
            AccessibilityNodeReferenceRegistry.invalidateAll()
            val healthGeneration = AccessibilityHealthMonitor.onDisconnected(service)
            generationRef.set(maxOf(generationRef.incrementAndGet(), healthGeneration))
        }
    }

    fun current(): AccessibilityService? = current.get()

    fun generation(): Long = generationRef.get()

    fun isServiceConnected(): Boolean = current.get() != null
}
