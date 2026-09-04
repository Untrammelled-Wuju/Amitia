package com.amitia.amitia_app.runtime.recovery

import android.app.job.JobParameters
import android.app.job.JobService
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import java.util.concurrent.atomic.AtomicBoolean

/** Survives process death/reboot and resumes only the currently fenced recovery request. */
class RuntimeRecoveryJobService : JobService() {
    override fun onStartJob(params: JobParameters): Boolean {
        val token = params.extras.getString(PersistentRuntimeRecoveryScheduler.EXTRA_TOKEN)?.trim().orEmpty()
        val failedGeneration = params.extras.getLong(PersistentRuntimeRecoveryScheduler.EXTRA_FAILED_GENERATION, 0L)
        val requestedProfile = params.extras.getString(PersistentRuntimeRecoveryScheduler.EXTRA_PROFILE)?.trim().orEmpty()
        val store = AndroidRuntimeDesiredStateStore(applicationContext)
        val state = store.snapshot()
        if (!state.desiredRunning || token.isEmpty() || token != state.recoveryToken || failedGeneration != state.lastFailureGeneration) {
            return false
        }
        if (state.nextRecoveryAt > System.currentTimeMillis()) {
            PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(applicationContext)
            return false
        }

        // Keep the token until controller.start() has actually been accepted.
        // markStarted() clears it. If the process dies before that point, the
        // persisted token remains and JobScheduler can safely retry it.
        val finished = AtomicBoolean(false)
        val finish: () -> Unit = {
            if (finished.compareAndSet(false, true)) jobFinished(params, false)
        }
        return try {
            AndroidRuntimeModule.create(applicationContext).controller.start(
                RuntimeStartRequest(
                    reason = RuntimeStartReason.RECOVERY,
                    profile = requestedProfile.ifEmpty { state.profile.ifEmpty { "local" } },
                ),
                object : RuntimeOperationCallback {
                    override fun onCompleted(operationResult: RuntimeOperationResult) = finish()
                },
            )
            true
        } catch (_: Throwable) {
            // If start was never accepted, the same token is still persisted.
            // Re-schedule explicitly because returning false tells Android this
            // particular JobService invocation has finished.
            PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(applicationContext)
            false
        }
    }

    override fun onStopJob(params: JobParameters): Boolean {
        val token = params.extras.getString(PersistentRuntimeRecoveryScheduler.EXTRA_TOKEN)?.trim().orEmpty()
        val state = AndroidRuntimeDesiredStateStore(applicationContext).snapshot()
        // Retry only while this exact fenced request is still pending. If
        // markStarted() already cleared it, a duplicate recovery must not run.
        return state.desiredRunning && token.isNotEmpty() && token == state.recoveryToken
    }
}
