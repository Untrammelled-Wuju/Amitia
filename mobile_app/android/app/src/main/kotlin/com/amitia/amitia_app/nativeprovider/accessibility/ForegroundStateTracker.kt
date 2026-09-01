package com.amitia.amitia_app.nativeprovider.accessibility

import android.view.accessibility.AccessibilityEvent

data class ForegroundStateSnapshot(
    val currentPackage: String,
    val currentActivity: String?,
    val previousPackage: String,
    val changedAt: Long,
    val generation: Long,
)

data class ForegroundPackageTransition(
    val previousPackage: String,
    val currentPackage: String,
    val currentActivity: String?,
    val changedAt: Long,
    val generation: Long,
)

internal object ForegroundStateTracker {
    private val lock = Object()
    private var snapshot = ForegroundStateSnapshot("", null, "", 0L, 0L)

    fun update(event: AccessibilityEvent): ForegroundPackageTransition? {
        if (event.eventType != AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED && event.eventType != AccessibilityEvent.TYPE_WINDOWS_CHANGED) {
            return null
        }
        val packageName = event.packageName?.toString()?.trim().orEmpty()
        if (packageName.isEmpty()) return null
        val activityName = event.className?.toString()?.trim()?.takeIf { it.isNotEmpty() }
        synchronized(lock) {
            val old = snapshot
            val transition = old.currentPackage.isNotEmpty() && old.currentPackage != packageName
            val changedAt = if (old.currentPackage != packageName) System.currentTimeMillis() else old.changedAt
            val generation = if (old.currentPackage != packageName || old.currentActivity != activityName) old.generation + 1 else old.generation
            snapshot = ForegroundStateSnapshot(
                currentPackage = packageName,
                currentActivity = activityName ?: old.currentActivity.takeIf { old.currentPackage == packageName },
                previousPackage = if (old.currentPackage != packageName) old.currentPackage else old.previousPackage,
                changedAt = changedAt,
                generation = generation,
            )
            lock.notifyAll()
            if (!transition) return null
            return ForegroundPackageTransition(
                previousPackage = old.currentPackage,
                currentPackage = packageName,
                currentActivity = snapshot.currentActivity,
                changedAt = changedAt,
                generation = generation,
            )
        }
    }

    fun reset() {
        synchronized(lock) {
            snapshot = ForegroundStateSnapshot("", null, "", 0L, snapshot.generation + 1)
            lock.notifyAll()
        }
    }

    fun current(): ForegroundStateSnapshot = synchronized(lock) { snapshot.copy() }

    fun awaitPackage(packageName: String, timeoutMs: Long): ForegroundStateSnapshot {
        val target = packageName.trim()
        val timeout = timeoutMs.coerceIn(1L, 60_000L)
        val deadline = System.currentTimeMillis() + timeout
        synchronized(lock) {
            while (snapshot.currentPackage != target) {
                val remaining = deadline - System.currentTimeMillis()
                if (remaining <= 0L) break
                lock.wait(remaining)
            }
            return snapshot.copy()
        }
    }
}
