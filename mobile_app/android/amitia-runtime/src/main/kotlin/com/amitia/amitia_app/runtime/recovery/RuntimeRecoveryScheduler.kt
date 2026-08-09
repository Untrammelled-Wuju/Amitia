package com.amitia.amitia_app.runtime.recovery

import java.util.concurrent.atomic.AtomicBoolean

internal interface RuntimeRecoveryScheduler {
    fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob
}

internal interface RuntimeRecoveryJob {
    fun cancel()
    val isCancelled: Boolean
}

internal class ExecutorRuntimeRecoveryScheduler : RuntimeRecoveryScheduler {
    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        val cancelled = AtomicBoolean(false)
        val thread = Thread {
            try {
                Thread.sleep(delayMillis)
            } catch (_: InterruptedException) {
                return@Thread
            }
            if (!cancelled.get()) {
                action()
            }
        }.apply {
            isDaemon = true
            name = "runtime-recovery-scheduler"
            start()
        }
        return object : RuntimeRecoveryJob {
            override fun cancel() {
                cancelled.set(true)
                thread.interrupt()
            }
            override val isCancelled: Boolean
                get() = cancelled.get()
        }
    }
}
