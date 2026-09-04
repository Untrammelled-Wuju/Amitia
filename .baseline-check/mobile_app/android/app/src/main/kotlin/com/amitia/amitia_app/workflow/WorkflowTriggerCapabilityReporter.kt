package com.amitia.amitia_app.workflow

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.text.TextUtils
import com.amitia.amitia_app.nativeprovider.AndroidNativeHost
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityHealthMonitor
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityStateReader
import com.amitia.amitia_app.nativeprovider.devicecontrol.DeviceInteractionStateReader
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.runtime.recovery.AndroidRuntimeDesiredStateStore
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress

internal object WorkflowTriggerCapabilityReporter {
    fun report(context: Context) {
        val appContext = context.applicationContext
        val microphoneAvailable = Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
            appContext.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        val accessibilityAvailable = AccessibilityServiceRegistry.isServiceConnected()
        val notificationListenerAvailable = notificationListenerEnabled(appContext)
        val foregroundLocationAvailable = Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
            appContext.checkSelfPermission(Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED
        val bluetoothPermissionAvailable = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            appContext.checkSelfPermission(Manifest.permission.BLUETOOTH_CONNECT) == PackageManager.PERMISSION_GRANTED &&
                appContext.checkSelfPermission(Manifest.permission.BLUETOOTH_SCAN) == PackageManager.PERMISSION_GRANTED
        } else {
            foregroundLocationAvailable
        }
        val backgroundLocationAvailable = Build.VERSION.SDK_INT < Build.VERSION_CODES.Q ||
            appContext.checkSelfPermission(Manifest.permission.ACCESS_BACKGROUND_LOCATION) == PackageManager.PERMISSION_GRANTED
        val geofenceAvailable = foregroundLocationAvailable && backgroundLocationAvailable
        val ingress = WorkflowDeviceEventIngress(appContext)
        ingress.reportCapabilities(
            listOf(
                mapOf(
                    "id" to "workflow.trigger.android_intent.v1",
                    "supported" to true,
                    "available" to true,
                    "permissionRequired" to false,
                    "permission" to "",
                    "reason" to "",
                ),
                mapOf(
                    "id" to "workflow.trigger.tasker.v1",
                    "supported" to true,
                    "available" to true,
                    "permissionRequired" to false,
                    "permission" to "",
                    "reason" to "",
                ),
                mapOf(
                    "id" to "workflow.trigger.notification.v1",
                    "supported" to true,
                    "available" to notificationListenerAvailable,
                    "permissionRequired" to true,
                    "permission" to "android.permission.BIND_NOTIFICATION_LISTENER_SERVICE",
                    "reason" to if (notificationListenerAvailable) "" else "Notification access is not granted",
                ),
                mapOf(
                    "id" to "workflow.trigger.system_event.v1",
                    "supported" to true,
                    "available" to true,
                    "permissionRequired" to false,
                    "permission" to "",
                    "reason" to "",
                ),
                mapOf(
                    "id" to "workflow.trigger.network.v1",
                    "supported" to true,
                    "available" to true,
                    "permissionRequired" to false,
                    "permission" to Manifest.permission.ACCESS_NETWORK_STATE,
                    "reason" to "",
                ),
                mapOf(
                    "id" to "workflow.trigger.bluetooth.v1",
                    "supported" to true,
                    "available" to bluetoothPermissionAvailable,
                    "permissionRequired" to (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S),
                    "permission" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                        "${Manifest.permission.BLUETOOTH_CONNECT},${Manifest.permission.BLUETOOTH_SCAN}"
                    } else {
                        Manifest.permission.BLUETOOTH
                    },
                    "reason" to if (bluetoothPermissionAvailable) "" else "Bluetooth permissions are not granted",
                ),
                mapOf(
                    "id" to "workflow.trigger.location.v1",
                    "supported" to true,
                    "available" to geofenceAvailable,
                    "permissionRequired" to true,
                    "permission" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                        "${Manifest.permission.ACCESS_FINE_LOCATION},${Manifest.permission.ACCESS_BACKGROUND_LOCATION}"
                    } else {
                        Manifest.permission.ACCESS_FINE_LOCATION
                    },
                    "reason" to if (geofenceAvailable) "" else "Foreground and background location permissions are required for geofence triggers",
                ),
                mapOf(
                    "id" to "workflow.trigger.voice_wake.v1",
                    "supported" to true,
                    "available" to microphoneAvailable,
                    "permissionRequired" to true,
                    "permission" to Manifest.permission.RECORD_AUDIO,
                    "reason" to if (microphoneAvailable) "" else "Microphone permission is not granted",
                ),
                mapOf(
                    "id" to "workflow.trigger.voice_phrase.v1",
                    "supported" to true,
                    "available" to microphoneAvailable,
                    "permissionRequired" to true,
                    "permission" to Manifest.permission.RECORD_AUDIO,
                    "reason" to if (microphoneAvailable) "" else "Microphone permission is not granted",
                ),
                mapOf(
                    "id" to "workflow.trigger.app_foreground.v1",
                    "supported" to true,
                    "available" to accessibilityAvailable,
                    "permissionRequired" to true,
                    "permission" to "android.accessibilityservice.AccessibilityService",
                    "reason" to if (accessibilityAvailable) "" else "Accessibility service is not connected",
                ),
            ),
        ) { result ->
            if (result.isSuccess) {
                synchronized(WorkflowTriggerCapabilityReporter) {
                    capabilityRetryAttempts = 0
                }
            } else {
                scheduleCapabilityRetry(appContext)
            }
        }
        reportAndroidRuntimeHealth(appContext, ingress)
        reportLauncherAppCatalog(appContext, ingress)
    }

    private fun reportAndroidRuntimeHealth(context: Context, ingress: WorkflowDeviceEventIngress) {
        val accessibilityState = AccessibilityStateReader(context).readState()
        val accessibilityHealth = AccessibilityHealthMonitor.snapshot(
            configured = accessibilityState.serviceDeclared,
            enabled = accessibilityState.enabledInSettings,
        )
        val interaction = DeviceInteractionStateReader(context).read()
        val runtimeDesiredState = AndroidRuntimeDesiredStateStore(context).snapshot()
        val nativeHealth = AndroidNativeHost.shared(context).health()
        val nativeBridgeReady = nativeHealth.status == NativeBridgeProtocol.HEALTH_READY
        val microphoneReady = Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
            context.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        val screenCaptureReady = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R &&
            accessibilityHealth.connected &&
            nativeHealth.capabilities["screen_capture.capture"] == true
        val uiAgentReady = accessibilityHealth.connected && nativeBridgeReady &&
            nativeHealth.capabilities["ui_tree.snapshot"] == true &&
            nativeHealth.capabilities["interaction.click"] == true
        ingress.reportAndroidRuntimeHealth(
            mapOf(
                "runtimeReady" to true,
                "nativeBridgeReady" to nativeBridgeReady,
                "accessibilityConfigured" to accessibilityHealth.configured,
                "accessibilityEnabled" to accessibilityHealth.enabled,
                "accessibilityReady" to accessibilityHealth.connected,
                "accessibilityGeneration" to accessibilityHealth.generation,
                "screenCaptureReady" to screenCaptureReady,
                "microphoneReady" to microphoneReady,
                "uiAgentReady" to uiAgentReady,
                "backgroundRestricted" to interaction.backgroundRestricted,
                "deviceIdleMode" to interaction.deviceIdleMode,
                "powerSaveMode" to interaction.powerSaveMode,
                "screenOn" to interaction.screenOn,
                "interactive" to interaction.interactive,
                "keyguardLocked" to interaction.keyguardLocked,
                "interactionState" to interaction.availability.name,
                "lastRuntimeFailureAtMs" to runtimeDesiredState.lastFailureAt,
                "lastRuntimeFailureGeneration" to runtimeDesiredState.lastFailureGeneration,
                "lastRuntimeFailureCode" to runtimeDesiredState.lastFailureCode.orEmpty(),
                "recoveryAttempt" to runtimeDesiredState.recoveryAttempt,
                "nextRecoveryAtMs" to runtimeDesiredState.nextRecoveryAt,
                "recoveryExhausted" to runtimeDesiredState.recoveryExhausted,
            ),
        ) {
            scheduleHealthHeartbeat(context)
        }
    }

    @Synchronized
    private fun scheduleHealthHeartbeat(context: Context) {
        if (healthHeartbeatPending) return
        healthHeartbeatPending = true
        val appContext = context.applicationContext
        retryHandler.postDelayed({
            synchronized(WorkflowTriggerCapabilityReporter) {
                healthHeartbeatPending = false
            }
            reportAndroidRuntimeHealth(appContext, WorkflowDeviceEventIngress(appContext))
        }, HEALTH_HEARTBEAT_MS)
    }

    @Synchronized
    private fun reportLauncherAppCatalog(context: Context, ingress: WorkflowDeviceEventIngress) {
        val now = System.currentTimeMillis()
        if (now - lastCatalogReportAt < APP_CATALOG_REFRESH_MS) return
        if (now - lastCatalogAttemptAt < APP_CATALOG_RETRY_MS) return
        lastCatalogAttemptAt = now
        val packageManager = context.packageManager
        val launcherIntent = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        val items = runCatching {
            packageManager.queryIntentActivities(launcherIntent, PackageManager.MATCH_DEFAULT_ONLY)
                .asSequence()
                .mapNotNull { resolveInfo ->
                    val packageName = resolveInfo.activityInfo?.packageName?.trim().orEmpty()
                    if (packageName.isEmpty()) return@mapNotNull null
                    val label = runCatching { resolveInfo.loadLabel(packageManager).toString().trim() }.getOrDefault("")
                    mapOf("packageName" to packageName, "label" to label.take(256))
                }
                .distinctBy { it["packageName"] }
                .sortedWith(compareBy<Map<String, String>> { it["label"].orEmpty() }.thenBy { it["packageName"].orEmpty() })
                .take(MAX_APP_CATALOG_ITEMS)
                .toList()
        }.getOrElse { return }
        ingress.reportAppCatalog(items) { result ->
            if (result.isSuccess) {
                synchronized(WorkflowTriggerCapabilityReporter) {
                    lastCatalogReportAt = System.currentTimeMillis()
                    catalogRetryAttempts = 0
                }
            } else {
                scheduleCatalogRetry(context)
            }
        }
    }

    @Synchronized
    private fun scheduleCapabilityRetry(context: Context) {
        if (capabilityRetryPending || capabilityRetryAttempts >= MAX_CAPABILITY_RETRIES) return
        val delay = minOf(60_000L, 5_000L shl capabilityRetryAttempts.coerceAtMost(3))
        capabilityRetryAttempts += 1
        capabilityRetryPending = true
        val appContext = context.applicationContext
        retryHandler.postDelayed({
            synchronized(WorkflowTriggerCapabilityReporter) {
                capabilityRetryPending = false
            }
            report(appContext)
        }, delay)
    }

    @Synchronized
    private fun scheduleCatalogRetry(context: Context) {
        if (catalogRetryPending || catalogRetryAttempts >= MAX_CATALOG_RETRIES) return
        catalogRetryAttempts += 1
        catalogRetryPending = true
        val appContext = context.applicationContext
        retryHandler.postDelayed({
            synchronized(WorkflowTriggerCapabilityReporter) {
                catalogRetryPending = false
            }
            report(appContext)
        }, APP_CATALOG_RETRY_MS)
    }

    private fun notificationListenerEnabled(context: Context): Boolean {
        val enabled = runCatching {
            Settings.Secure.getString(context.contentResolver, "enabled_notification_listeners")
        }.getOrNull().orEmpty()
        if (enabled.isBlank()) return false
        val packageName = context.packageName
        return enabled.split(':').any { flattened ->
            val componentPackage = flattened.substringBefore('/').trim()
            TextUtils.equals(componentPackage, packageName)
        }
    }

    private const val APP_CATALOG_REFRESH_MS = 5 * 60 * 1000L
    private const val APP_CATALOG_RETRY_MS = 30 * 1000L
    private const val MAX_APP_CATALOG_ITEMS = 2000
    private const val MAX_CAPABILITY_RETRIES = 6
    private const val MAX_CATALOG_RETRIES = 6
    private const val HEALTH_HEARTBEAT_MS = 30_000L
    private val retryHandler = Handler(Looper.getMainLooper())
    private var healthHeartbeatPending = false
    private var capabilityRetryPending = false
    private var capabilityRetryAttempts = 0
    private var catalogRetryPending = false
    private var catalogRetryAttempts = 0
    private var lastCatalogAttemptAt: Long = 0L
    private var lastCatalogReportAt: Long = 0L
}
