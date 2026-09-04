package com.amitia.amitia_app.workflow

import android.app.job.JobInfo
import android.app.job.JobParameters
import android.app.job.JobScheduler
import android.app.job.JobService
import android.content.ComponentName
import android.content.Context
import android.os.Build
import com.amitia.amitia_app.nativeprovider.devicecontrol.GeofenceAutomationManager
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.recovery.AndroidRuntimeDesiredStateStore
import com.amitia.amitia_app.runtime.recovery.PersistentRuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import java.util.concurrent.atomic.AtomicBoolean

class WorkflowAutomationHealthJobService : JobService() {
    override fun onStartJob(params: JobParameters): Boolean {
        val app = applicationContext
        val finished = AtomicBoolean(false)
        val finish: () -> Unit = {
            if (finished.compareAndSet(false, true)) jobFinished(params, false)
        }
        WorkflowSystemEventRegistrar.ensureRegistered(app)
        GeofenceAutomationManager.restore(app)
        val state = AndroidRuntimeDesiredStateStore(app).snapshot()
        if (!state.desiredRunning) {
            WorkflowDeviceEventIngress(app).flushPending { finish() }
            return true
        }
        if (!state.recoveryToken.isNullOrBlank() && state.nextRecoveryAt > 0L) {
            PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(app)
            WorkflowDeviceEventIngress(app).flushPending { finish() }
            return true
        }
        return try {
            AndroidRuntimeModule.create(app).controller.start(
                RuntimeStartRequest(reason = RuntimeStartReason.BACKGROUND_TASK, profile = state.profile),
                object : RuntimeOperationCallback {
                    override fun onCompleted(operationResult: RuntimeOperationResult) {
                        WorkflowDeviceEventIngress(app).flushPending { finish() }
                    }
                },
            )
            true
        } catch (_: Throwable) {
            WorkflowDeviceEventIngress(app).flushPending { finish() }
            true
        }
    }

    override fun onStopJob(params: JobParameters): Boolean = true

    companion object {
        private const val JOB_ID = 0x414D4954
        private const val PERIOD_MS = 15L * 60L * 1000L

        fun schedule(context: Context) {
            val app = context.applicationContext
            val scheduler = app.getSystemService(Context.JOB_SCHEDULER_SERVICE) as? JobScheduler ?: return
            val builder = JobInfo.Builder(JOB_ID, ComponentName(app, WorkflowAutomationHealthJobService::class.java))
                .setPersisted(true)
                .setPeriodic(PERIOD_MS)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) builder.setRequiresBatteryNotLow(false)
            runCatching { scheduler.schedule(builder.build()) }
            PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(app)
        }
    }
}
