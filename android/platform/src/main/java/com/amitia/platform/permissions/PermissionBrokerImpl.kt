package com.amitia.platform.permissions

import android.app.Activity
import android.app.Application
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.amitia.core.logging.Logger
import com.amitia.platform.bridge.ActivityResultBridge
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.lang.ref.WeakReference
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume

@Singleton
class PermissionBrokerImpl @Inject constructor(
    @ApplicationContext private val context: Context,
    private val activityResultBridge: ActivityResultBridge,
    private val logger: Logger
) : PermissionBroker {

    private val appContext = context.applicationContext as Application
    private var currentActivity: WeakReference<Activity>? = null
    private val mutex = Mutex()
    private val permissionStateFlow = MutableStateFlow<Map<String, PermissionBroker.PermissionResult>>(emptyMap())

    init {
        appContext.registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityCreated(activity: Activity, savedInstanceState: android.os.Bundle?) {}
            override fun onActivityStarted(activity: Activity) {}
            override fun onActivityResumed(activity: Activity) {
                currentActivity = WeakReference(activity)
            }
            override fun onActivityPaused(activity: Activity) {
                currentActivity?.get()?.let { if (it === activity) currentActivity = null }
            }
            override fun onActivityStopped(activity: Activity) {}
            override fun onActivitySaveInstanceState(activity: Activity, outState: android.os.Bundle) {}
            override fun onActivityDestroyed(activity: Activity) {
                currentActivity?.get()?.let { if (it === activity) currentActivity = null }
            }
        })
    }

    override suspend fun request(permission: String): PermissionBroker.PermissionResult = mutex.withLock {
        if (isGrantedInternal(permission)) {
            updateState(permission, PermissionBroker.PermissionResult.Granted)
            return@withLock PermissionBroker.PermissionResult.Granted
        }

        if (activityResultBridge.hasActivity()) {
            val granted = activityResultBridge.requestPermission(permission)
            val mapped = if (granted) PermissionBroker.PermissionResult.Granted
            else {
                val activity = currentActivity?.get()
                if (activity != null && ActivityCompat.shouldShowRequestPermissionRationale(activity, permission)) {
                    PermissionBroker.PermissionResult.Denied
                } else {
                    PermissionBroker.PermissionResult.PermanentlyDenied
                }
            }
            updateState(permission, mapped)
            return@withLock mapped
        }

        val activity = currentActivity?.get()
            ?: return@withLock PermissionBroker.PermissionResult.Denied

        val granted = suspendCancellableCoroutine { cont ->
            val requestCode = permission.hashCode() and 0xFFFF
            pendingRequests[requestCode] = { g ->
                if (cont.isActive) cont.resume(g)
            }
            ActivityCompat.requestPermissions(activity, arrayOf(permission), requestCode)
            cont.invokeOnCancellation { pendingRequests.remove(requestCode) }
        }
        val mapped = mapResult(permission, granted, activity)
        updateState(permission, mapped)
        mapped
    }

    override suspend fun requestMultiple(permissions: List<String>): Map<String, PermissionBroker.PermissionResult> {
        val results = mutableMapOf<String, PermissionBroker.PermissionResult>()
        if (activityResultBridge.hasActivity() && permissions.isNotEmpty()) {
            val grantedMap = activityResultBridge.requestPermissions(permissions.toTypedArray())
            permissions.forEach { p ->
                val granted = grantedMap[p] ?: isGrantedInternal(p)
                val mapped = if (granted) PermissionBroker.PermissionResult.Granted
                else PermissionBroker.PermissionResult.Denied
                updateState(p, mapped)
                results[p] = mapped
            }
            return results
        }
        for (permission in permissions) {
            results[permission] = request(permission)
        }
        return results
    }

    override suspend fun shouldShowRationale(permission: String): Boolean {
        val activity = currentActivity?.get() ?: return false
        return ActivityCompat.shouldShowRequestPermissionRationale(activity, permission)
    }

    override suspend fun isGranted(permission: String): Boolean = isGrantedInternal(permission)

    override suspend fun openAppSettings(): Boolean {
        if (activityResultBridge.hasActivity()) {
            return activityResultBridge.openSettings()
        }
        return try {
            val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                data = Uri.fromParts("package", context.packageName, null)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
            true
        } catch (t: Throwable) {
            logger.e(TAG, "openAppSettings failed", t)
            false
        }
    }

    override fun observePermissionChanges(): StateFlow<Map<String, PermissionBroker.PermissionResult>> {
        return permissionStateFlow.asStateFlow()
    }

    private fun isGrantedInternal(permission: String): Boolean {
        return ContextCompat.checkSelfPermission(context, permission) ==
            android.content.pm.PackageManager.PERMISSION_GRANTED
    }

    private fun mapResult(
        permission: String,
        granted: Boolean,
        activity: Activity
    ): PermissionBroker.PermissionResult {
        if (granted) return PermissionBroker.PermissionResult.Granted
        val shouldShow = ActivityCompat.shouldShowRequestPermissionRationale(activity, permission)
        return if (shouldShow) PermissionBroker.PermissionResult.Denied
        else PermissionBroker.PermissionResult.PermanentlyDenied
    }

    private fun updateState(permission: String, result: PermissionBroker.PermissionResult) {
        permissionStateFlow.value = permissionStateFlow.value + (permission to result)
    }

    fun handleRequestResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        val callback = pendingRequests.remove(requestCode) ?: return
        val granted = permissions.isNotEmpty() &&
            grantResults.isNotEmpty() &&
            grantResults[0] == android.content.pm.PackageManager.PERMISSION_GRANTED
        callback.invoke(granted)
    }

    companion object {
        private const val TAG = "PermissionBrokerImpl"
        private val pendingRequests = mutableMapOf<Int, (Boolean) -> Unit>()
    }
}
