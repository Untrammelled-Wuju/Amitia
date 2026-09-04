package com.amitia.amitia_app.workflow

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.amitia.amitia_app.nativeprovider.devicecontrol.GeofenceAutomationManager
import com.amitia.amitia_app.runtime.AndroidRuntimeModule
import com.amitia.amitia_app.runtime.api.RuntimeOperationCallback
import com.amitia.amitia_app.runtime.api.RuntimeOperationResult
import com.amitia.amitia_app.runtime.api.RuntimeStartReason
import com.amitia.amitia_app.runtime.api.RuntimeStartRequest
import com.amitia.amitia_app.runtime.recovery.AndroidRuntimeDesiredStateStore
import com.amitia.amitia_app.runtime.recovery.PersistentRuntimeRecoveryScheduler
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONObject
import java.security.MessageDigest

class WorkflowManifestEventReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val action = intent?.action ?: return
        val app = context.applicationContext
        WorkflowSystemEventRegistrar.ensureRegistered(app)
        WorkflowAutomationHealthJobService.schedule(app)
        val ingress = WorkflowDeviceEventIngress(app)

        when (action) {
            Intent.ACTION_BOOT_COMPLETED -> {
                emit(ingress, "device.system.boot_completed", JSONObject().put("lockedBoot", false), action)
                GeofenceAutomationManager.restore(app)
                AndroidRuntimeDesiredStateStore(app).incrementBootGeneration()
                restoreRuntimeIfDesired(app)
            }
            Intent.ACTION_MY_PACKAGE_REPLACED -> {
                emit(ingress, "device.app.self_updated", JSONObject().put("packageName", app.packageName), action)
                GeofenceAutomationManager.restore(app)
                restoreRuntimeIfDesired(app)
            }
            Intent.ACTION_PACKAGE_ADDED -> if (!intent.getBooleanExtra(Intent.EXTRA_REPLACING, false)) {
                emitPackage(ingress, "device.app.installed", intent)
            }
            Intent.ACTION_PACKAGE_REMOVED -> if (!intent.getBooleanExtra(Intent.EXTRA_REPLACING, false)) {
                emitPackage(ingress, "device.app.removed", intent)
            }
            Intent.ACTION_PACKAGE_REPLACED -> emitPackage(ingress, "device.app.updated", intent)
            Intent.ACTION_TIME_CHANGED -> emit(ingress, "device.time.changed", JSONObject(), action)
            Intent.ACTION_TIMEZONE_CHANGED -> emit(
                ingress,
                "device.time.timezone_changed",
                JSONObject().put("timeZone", intent.getStringExtra("time-zone").orEmpty()),
                action,
            )
            Intent.ACTION_DATE_CHANGED -> emit(ingress, "device.time.date_changed", JSONObject(), action)
        }
    }

    private fun emitPackage(ingress: WorkflowDeviceEventIngress, eventType: String, intent: Intent) {
        val packageName = intent.data?.schemeSpecificPart.orEmpty()
        if (packageName.isBlank()) return
        emit(
            ingress,
            eventType,
            JSONObject()
                .put("packageName", packageName)
                .put("replacing", intent.getBooleanExtra(Intent.EXTRA_REPLACING, false)),
            intent.action.orEmpty(),
        )
    }

    private fun emit(ingress: WorkflowDeviceEventIngress, type: String, payload: JSONObject, action: String) {
        val raw = "$type\u0000$action\u0000${payload}\u0000${System.currentTimeMillis()}"
        val hash = MessageDigest.getInstance("SHA-256").digest(raw.toByteArray()).joinToString("") { "%02x".format(it) }
        ingress.emit(type, payload, "android.manifest_receiver", "android:${hash.take(40)}")
    }

    private fun restoreRuntimeIfDesired(context: Context) {
        val state = AndroidRuntimeDesiredStateStore(context).snapshot()
        if (!state.desiredRunning) return
        if (!state.recoveryToken.isNullOrBlank() && state.nextRecoveryAt > 0L) {
            PersistentRuntimeRecoveryScheduler.ensureScheduledFromStore(context)
            return
        }
        val pending = goAsync()
        Thread({
            try {
                AndroidRuntimeModule.create(context).controller.start(
                    RuntimeStartRequest(reason = RuntimeStartReason.BACKGROUND_TASK, profile = state.profile),
                    object : RuntimeOperationCallback {
                        override fun onCompleted(operationResult: RuntimeOperationResult) = pending.finish()
                    },
                )
            } catch (_: Throwable) {
                pending.finish()
            }
        }, "amitia-runtime-boot-restore").start()
    }
}
