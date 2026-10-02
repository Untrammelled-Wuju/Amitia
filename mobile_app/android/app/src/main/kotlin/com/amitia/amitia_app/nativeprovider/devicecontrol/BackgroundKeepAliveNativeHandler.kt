package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.content.Context
import android.content.Intent
import android.os.PowerManager
import android.os.Build
import android.provider.Settings
import com.amitia.amitia_app.MainActivity
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import com.amitia.amitia_app.runtime.service.BackgroundKeepAlive
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull

internal class BackgroundKeepAliveNativeHandler(private val context: Context) : AndroidNativeOperationHandler {
    override val operations = setOf("device.keep_alive.status", "device.keep_alive.set", "device.keep_alive.battery_settings")

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        when (request.operation) {
            "device.keep_alive.set" -> {
                val enabled = request.payload["enabled"] as? Boolean
                    ?: return error(request, "INVALID_REQUEST", "enabled must be a boolean")
                if (enabled && MainActivity.currentActivity() == null) return error(request, "FOREGROUND_REQUIRED", "请在应用前台开启后台保活")
                val previous = BackgroundKeepAlive.enabled(context)
                try {
                    withContext(Dispatchers.IO) { BackgroundKeepAlive.setEnabled(context, enabled) }
                    if (enabled) {
                        val ready = withTimeoutOrNull(5000) {
                            while (!BackgroundKeepAlive.active && BackgroundKeepAlive.lastError == null) delay(50)
                            BackgroundKeepAlive.active
                        } == true
                        check(ready) { BackgroundKeepAlive.lastError ?: "系统限制了后台保活，请检查省电设置" }
                    }
                } catch (failure: Exception) {
                    withContext(Dispatchers.IO) { runCatching { BackgroundKeepAlive.setEnabled(context, previous) } }
                    return error(request, "KEEP_ALIVE_FAILED", failure.message ?: "后台保活设置失败")
                }
            }
            "device.keep_alive.battery_settings" -> {
                if (MainActivity.currentActivity() == null) return error(request, "FOREGROUND_REQUIRED", "请在应用前台打开省电设置")
                withContext(Dispatchers.Main.immediate) {
                val activity = MainActivity.currentActivity() ?: error("应用已进入后台")
                activity.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
                }
            }
            "device.keep_alive.status" -> Unit
            else -> return error(request, "OPERATION_NOT_SUPPORTED", "unsupported keep alive operation")
        }
        val power = context.getSystemService(Context.POWER_SERVICE) as PowerManager
        return NativeBridgeResponse(NativeBridgeProtocol.PROTOCOL_VERSION, request.requestId, NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf("enabled" to BackgroundKeepAlive.enabled(context), "active" to BackgroundKeepAlive.active,
                "batteryUnrestricted" to (Build.VERSION.SDK_INT < Build.VERSION_CODES.M || power.isIgnoringBatteryOptimizations(context.packageName)), "lastError" to BackgroundKeepAlive.lastError))
    }

    private fun error(request: NativeBridgeRequest, code: String, message: String) = NativeBridgeResponse(
        NativeBridgeProtocol.PROTOCOL_VERSION, request.requestId, NativeBridgeProtocol.STATUS_ERROR, error = NativeBridgeError(code, message))
}
