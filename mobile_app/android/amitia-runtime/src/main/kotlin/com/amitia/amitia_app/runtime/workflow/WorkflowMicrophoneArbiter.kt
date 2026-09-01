package com.amitia.amitia_app.runtime.workflow

/**
 * Process-local arbitration between foreground realtime voice capture and the
 * background workflow wake listener. Realtime voice has priority.
 */
object WorkflowMicrophoneArbiter {
    private val lock = Object()
    private var realtimeCaptureLeases: Int = 0
    private var wakeCaptureActive: Boolean = false

    fun acquireRealtimeCapture() {
        synchronized(lock) {
            realtimeCaptureLeases++
            lock.notifyAll()
        }
    }

    fun releaseRealtimeCapture() {
        synchronized(lock) {
            if (realtimeCaptureLeases > 0) realtimeCaptureLeases--
            lock.notifyAll()
        }
    }

    fun isRealtimeCaptureActive(): Boolean = synchronized(lock) { realtimeCaptureLeases > 0 }

    fun tryAcquireWakeCapture(): Boolean = synchronized(lock) {
        if (realtimeCaptureLeases > 0 || wakeCaptureActive) return@synchronized false
        wakeCaptureActive = true
        true
    }

    fun releaseWakeCapture() {
        synchronized(lock) {
            if (!wakeCaptureActive) return
            wakeCaptureActive = false
            lock.notifyAll()
        }
    }

    fun awaitWakeCaptureReleased(timeoutMs: Long): Boolean {
        val timeout = timeoutMs.coerceIn(0L, 2_000L)
        val deadline = System.currentTimeMillis() + timeout
        synchronized(lock) {
            while (wakeCaptureActive) {
                val remaining = deadline - System.currentTimeMillis()
                if (remaining <= 0L) return false
                try {
                    lock.wait(remaining)
                } catch (_: InterruptedException) {
                    Thread.currentThread().interrupt()
                    return false
                }
            }
            return true
        }
    }
}
