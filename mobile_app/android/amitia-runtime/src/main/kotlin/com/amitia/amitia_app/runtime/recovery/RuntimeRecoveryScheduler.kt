package com.amitia.amitia_app.runtime.recovery

import android.app.PersistableBundle
import android.app.job.JobInfo
import android.app.job.JobScheduler
import android.content.ComponentName
import android.content.Context
import java.util.concurrent.atomic.AtomicBoolean

data class RuntimeRecoveryScheduleRequest(
    val delayMillis: Long,
    val failedGeneration: Long,
    val profile: String,
    val reason: String,
)

interface RuntimeRecoveryScheduler {
    fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob

    fun schedule(request: RuntimeRecoveryScheduleRequest, action: () -> Unit): RuntimeRecoveryJob =
        schedule(request.delayMillis, action)

    /** Cancel a persisted pending recovery owned by this scheduler, if any. */
    fun cancelPending() {}
}

interface RuntimeRecoveryJob {
    fun cancel()
    val isCancelled: Boolean
}

/** Test/JVM fallback. Android production wiring uses PersistentRuntimeRecoveryScheduler. */
internal class ExecutorRuntimeRecoveryScheduler : RuntimeRecoveryScheduler {
    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob {
        val cancelled = AtomicBoolean(false)
        val thread = Thread {
            try {
                Thread.sleep(delayMillis)
            } catch (_: InterruptedException) {
                return@Thread
            }
            if (!cancelled.get()) action()
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
            override val isCancelled: Boolean get() = cancelled.get()
        }
    }
}

class PersistentRuntimeRecoveryScheduler(
    context: Context,
    private val desiredStateStore: RuntimeDesiredStateStore,
) : RuntimeRecoveryScheduler {
    private val app = context.applicationContext
    private val scheduler = app.getSystemService(Context.JOB_SCHEDULER_SERVICE) as JobScheduler
    private val fallback = ExecutorRuntimeRecoveryScheduler()

    override fun schedule(delayMillis: Long, action: () -> Unit): RuntimeRecoveryJob =
        fallback.schedule(delayMillis, action)

    override fun schedule(request: RuntimeRecoveryScheduleRequest, action: () -> Unit): RuntimeRecoveryJob {
        scheduler.cancel(JOB_ID)
        val state = desiredStateStore.scheduleRecovery(
            failedGeneration = request.failedGeneration,
            delayMillis = request.delayMillis,
            reason = request.reason,
        )
        val token = state.recoveryToken ?: throw IllegalStateException("recovery token was not persisted")
        val extras = PersistableBundle().apply {
            putString(EXTRA_TOKEN, token)
            putLong(EXTRA_FAILED_GENERATION, request.failedGeneration)
            putString(EXTRA_PROFILE, request.profile)
        }
        val info = JobInfo.Builder(JOB_ID, ComponentName(app, RuntimeRecoveryJobService::class.java))
            .setPersisted(true)
            .setMinimumLatency(request.delayMillis.coerceAtLeast(0L))
            .setOverrideDeadline((request.delayMillis.coerceAtLeast(0L) + DEADLINE_SLACK_MS).coerceAtLeast(1L))
            .setExtras(extras)
            .build()
        val result = scheduler.schedule(info)
        if (result != JobScheduler.RESULT_SUCCESS) {
            desiredStateStore.clearScheduledRecovery(token)
            throw IllegalStateException("JobScheduler rejected runtime recovery job")
        }
        return PersistentRecoveryJob(scheduler, desiredStateStore, token)
    }

    override fun cancelPending() {
        val token = desiredStateStore.snapshot().recoveryToken
        if (token.isNullOrBlank()) return
        scheduler.cancel(JOB_ID)
        desiredStateStore.clearScheduledRecovery(token)
    }

    private class PersistentRecoveryJob(
        private val scheduler: JobScheduler,
        private val store: RuntimeDesiredStateStore,
        private val token: String,
    ) : RuntimeRecoveryJob {
        private val cancelled = AtomicBoolean(false)
        override fun cancel() {
            if (cancelled.compareAndSet(false, true)) {
                scheduler.cancel(JOB_ID)
                store.clearScheduledRecovery(token)
            }
        }
        override val isCancelled: Boolean get() = cancelled.get()
    }

    companion object {
        internal const val JOB_ID = 0x414D5252 // AMRR
        internal const val EXTRA_TOKEN = "recoveryToken"
        internal const val EXTRA_FAILED_GENERATION = "failedGeneration"
        internal const val EXTRA_PROFILE = "profile"
        private const val DEADLINE_SLACK_MS = 30_000L

        fun ensureScheduledFromStore(context: Context): Boolean {
            val app = context.applicationContext
            val store = AndroidRuntimeDesiredStateStore(app)
            val state = store.snapshot()
            if (!state.desiredRunning || state.recoveryToken.isNullOrBlank() || state.nextRecoveryAt <= 0L) return false
            val scheduler = app.getSystemService(Context.JOB_SCHEDULER_SERVICE) as? JobScheduler ?: return false
            val remaining = (state.nextRecoveryAt - System.currentTimeMillis()).coerceAtLeast(0L)
            val extras = PersistableBundle().apply {
                putString(EXTRA_TOKEN, state.recoveryToken)
                putLong(EXTRA_FAILED_GENERATION, state.lastFailureGeneration)
                putString(EXTRA_PROFILE, state.profile)
            }
            val info = JobInfo.Builder(JOB_ID, ComponentName(app, RuntimeRecoveryJobService::class.java))
                .setPersisted(true)
                .setMinimumLatency(remaining)
                .setOverrideDeadline((remaining + DEADLINE_SLACK_MS).coerceAtLeast(1L))
                .setExtras(extras)
                .build()
            return scheduler.schedule(info) == JobScheduler.RESULT_SUCCESS
        }

        fun cancel(context: Context) {
            val app = context.applicationContext
            (app.getSystemService(Context.JOB_SCHEDULER_SERVICE) as? JobScheduler)?.cancel(JOB_ID)
            AndroidRuntimeDesiredStateStore(app).clearScheduledRecovery()
        }
    }
}
