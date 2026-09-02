package com.amitia.amitia_app.nativeprovider.accessibility

import android.accessibilityservice.AccessibilityService
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

data class AccessibilityHealthSnapshot(
    val configured: Boolean,
    val enabled: Boolean,
    val connected: Boolean,
    val lastConnectedAt: Long,
    val lastEventAt: Long,
    val lastDisconnectAt: Long,
    val generation: Long,
)

internal object AccessibilityHealthMonitor {
    private val lastConnectedAt = AtomicLong(0L)
    private val lastEventAt = AtomicLong(0L)
    private val lastDisconnectAt = AtomicLong(0L)
    private val generation = AtomicLong(0L)
    private val connectedService = AtomicReference<AccessibilityService?>(null)

    fun onConnected(service: AccessibilityService): Long {
        connectedService.set(service)
        lastConnectedAt.set(System.currentTimeMillis())
        return generation.incrementAndGet()
    }

    fun onEvent(service: AccessibilityService) {
        if (connectedService.get() === service) lastEventAt.set(System.currentTimeMillis())
    }

    fun onDisconnected(service: AccessibilityService): Long {
        connectedService.compareAndSet(service, null)
        lastDisconnectAt.set(System.currentTimeMillis())
        return generation.incrementAndGet()
    }

    fun snapshot(configured: Boolean, enabled: Boolean): AccessibilityHealthSnapshot =
        AccessibilityHealthSnapshot(
            configured = configured,
            enabled = enabled,
            connected = connectedService.get() != null,
            lastConnectedAt = lastConnectedAt.get(),
            lastEventAt = lastEventAt.get(),
            lastDisconnectAt = lastDisconnectAt.get(),
            generation = generation.get(),
        )
}
