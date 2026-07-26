package com.amitia.platform.bridge.provider

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.res.Configuration
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.BatteryManager
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class SystemThemeProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "system_theme"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val mode = context.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK
        val theme = when (mode) {
            Configuration.UI_MODE_NIGHT_YES -> "dark"
            Configuration.UI_MODE_NIGHT_NO -> "light"
            else -> "system"
        }
        return NativeActionResult.Success(mapOf("theme" to theme))
    }
}

@Singleton
class NetworkStateProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "network_state"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
        val caps = cm.getNetworkCapabilities(network)
        val connected = caps != null
        val type = when {
            caps == null -> "none"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
            caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
            else -> "other"
        }
        return NativeActionResult.Success(
            mapOf(
                "connected" to connected.toString(),
                "type" to type
            )
        )
    }
}

@Singleton
class BatteryStateProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "battery_state"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val filter = IntentFilter(Intent.ACTION_BATTERY_CHANGED)
        val intent = context.registerReceiver(null as BroadcastReceiver?, filter)
            ?: return NativeActionResult.Failed("battery_unknown")
        val level = intent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
        val scale = intent.getIntExtra(BatteryManager.EXTRA_SCALE, -1)
        val percent = if (level >= 0 && scale > 0) (level * 100 / scale) else -1
        val charging = intent.getIntExtra(BatteryManager.EXTRA_STATUS, -1) == BatteryManager.BATTERY_STATUS_CHARGING
        return NativeActionResult.Success(
            mapOf(
                "level" to percent.toString(),
                "charging" to charging.toString()
            )
        )
    }
}

@Singleton
class ForegroundStateProvider @Inject constructor() : CapabilityProvider {

    override fun action(): String = "foreground_state"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        return NativeActionResult.Success(mapOf("foreground" to "unknown"))
    }
}

@Singleton
class AppDirProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "app_dir"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        return NativeActionResult.Success(
            mapOf(
                "files_dir" to context.filesDir.absolutePath,
                "cache_dir" to context.cacheDir.absolutePath,
                "package" to context.packageName
            )
        )
    }
}
