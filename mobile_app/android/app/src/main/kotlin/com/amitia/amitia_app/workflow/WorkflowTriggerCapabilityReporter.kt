package com.amitia.amitia_app.workflow

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress

internal object WorkflowTriggerCapabilityReporter {
    fun report(context: Context) {
        val appContext = context.applicationContext
        val microphoneAvailable = Build.VERSION.SDK_INT < Build.VERSION_CODES.M ||
            appContext.checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        val accessibilityAvailable = AccessibilityServiceRegistry.isServiceConnected()
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
                    "id" to "workflow.trigger.voice_wake.v1",
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
        reportLauncherAppCatalog(appContext, ingress)
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

    private const val APP_CATALOG_REFRESH_MS = 5 * 60 * 1000L
    private const val APP_CATALOG_RETRY_MS = 30 * 1000L
    private const val MAX_APP_CATALOG_ITEMS = 2000
    private const val MAX_CAPABILITY_RETRIES = 6
    private const val MAX_CATALOG_RETRIES = 6
    private val retryHandler = Handler(Looper.getMainLooper())
    private var capabilityRetryPending = false
    private var capabilityRetryAttempts = 0
    private var catalogRetryPending = false
    private var catalogRetryAttempts = 0
    private var lastCatalogAttemptAt: Long = 0L
    private var lastCatalogReportAt: Long = 0L
}
