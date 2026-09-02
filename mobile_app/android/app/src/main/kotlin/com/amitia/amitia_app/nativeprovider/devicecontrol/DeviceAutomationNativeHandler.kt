package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.Manifest
import android.accessibilityservice.AccessibilityService
import android.app.ActivityManager
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothManager
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanResult
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.ApplicationInfo
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.media.AudioManager
import android.net.Uri
import android.os.BatteryManager
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.os.PowerManager
import android.os.StatFs
import android.provider.Settings
import android.view.KeyEvent
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.Locale
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

internal class DeviceAutomationNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val bluetoothSessions = BluetoothAutomationManager(context)
    private val locationExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "amitia-location-callback").apply { isDaemon = true }
    }

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_DEVICE_INFO,
        OP_GLOBAL_ACTION,
        OP_PRESS_KEY,
        OP_LOCATION_CURRENT,
        OP_GEOFENCE_ADD,
        OP_GEOFENCE_REMOVE,
        OP_GEOFENCE_LIST,
        OP_SETTINGS_GET,
        OP_SETTINGS_SET,
        OP_APP_LIST,
        OP_APP_INFO,
        OP_APP_OPEN,
        OP_APP_STOP,
        OP_APP_INSTALL,
        OP_APP_UNINSTALL,
        OP_BLUETOOTH_STATUS,
        OP_BLUETOOTH_REQUEST_ENABLE,
        OP_BLUETOOTH_PAIR,
        OP_BLUETOOTH_PAIRED,
        OP_BLUETOOTH_SCAN,
        OP_BLE_SCAN,
        OP_BLUETOOTH_CLASSIC_CONNECT,
        OP_BLUETOOTH_CLASSIC_DISCONNECT,
        OP_BLUETOOTH_CLASSIC_READ,
        OP_BLUETOOTH_CLASSIC_WRITE,
        OP_BLUETOOTH_CLASSIC_LISTEN,
        OP_BLUETOOTH_CLASSIC_ACCEPT,
        OP_BLUETOOTH_CLASSIC_CLOSE_SERVER,
        OP_BLE_CONNECT,
        OP_BLE_DISCONNECT,
        OP_BLE_SERVICES,
        OP_BLE_CHARACTERISTICS,
        OP_BLE_READ,
        OP_BLE_WRITE,
        OP_BLE_SUBSCRIBE,
        OP_BLE_UNSUBSCRIBE,
        OP_TASKER_RUN_TASK,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse = when (request.operation) {
        OP_STATUS -> status(request)
        OP_DEVICE_INFO -> deviceInfo(request)
        OP_GLOBAL_ACTION -> globalAction(request)
        OP_PRESS_KEY -> pressKey(request)
        OP_LOCATION_CURRENT -> currentLocation(request)
        OP_GEOFENCE_ADD -> geofenceAdd(request)
        OP_GEOFENCE_REMOVE -> geofenceRemove(request)
        OP_GEOFENCE_LIST -> geofenceList(request)
        OP_SETTINGS_GET -> settingsGet(request)
        OP_SETTINGS_SET -> settingsSet(request)
        OP_APP_LIST -> appList(request)
        OP_APP_INFO -> appInfo(request)
        OP_APP_OPEN -> appOpen(request)
        OP_APP_STOP -> appStop(request)
        OP_APP_INSTALL -> appInstall(request)
        OP_APP_UNINSTALL -> appUninstall(request)
        OP_BLUETOOTH_STATUS -> bluetoothStatus(request)
        OP_BLUETOOTH_REQUEST_ENABLE -> bluetoothRequestEnable(request)
        OP_BLUETOOTH_PAIR -> bluetoothPair(request)
        OP_BLUETOOTH_PAIRED -> bluetoothPaired(request)
        OP_BLUETOOTH_SCAN -> bluetoothScan(request)
        OP_BLE_SCAN -> bleScan(request)
        OP_BLUETOOTH_CLASSIC_CONNECT -> bluetoothResult(request, bluetoothAdapterRequired(request) { bluetoothSessions.classicConnect(it, request.payload) })
        OP_BLUETOOTH_CLASSIC_DISCONNECT -> bluetoothResult(request, bluetoothSessions.classicDisconnect(request.payload))
        OP_BLUETOOTH_CLASSIC_READ -> bluetoothResult(request, bluetoothSessions.classicRead(request.payload))
        OP_BLUETOOTH_CLASSIC_WRITE -> bluetoothResult(request, bluetoothSessions.classicWrite(request.payload))
        OP_BLUETOOTH_CLASSIC_LISTEN -> bluetoothResult(request, bluetoothAdapterRequired(request) { bluetoothSessions.classicListen(it, request.payload) })
        OP_BLUETOOTH_CLASSIC_ACCEPT -> bluetoothResult(request, bluetoothSessions.classicAccept(request.payload))
        OP_BLUETOOTH_CLASSIC_CLOSE_SERVER -> bluetoothResult(request, bluetoothSessions.classicCloseServer(request.payload))
        OP_BLE_CONNECT -> bluetoothResult(request, bluetoothAdapterRequired(request) { bluetoothSessions.bleConnect(it, request.payload) })
        OP_BLE_DISCONNECT -> bluetoothResult(request, bluetoothSessions.bleDisconnect(request.payload))
        OP_BLE_SERVICES -> bluetoothResult(request, bluetoothSessions.bleServices(request.payload))
        OP_BLE_CHARACTERISTICS -> bluetoothResult(request, bluetoothSessions.bleCharacteristics(request.payload))
        OP_BLE_READ -> bluetoothResult(request, bluetoothSessions.bleRead(request.payload))
        OP_BLE_WRITE -> bluetoothResult(request, bluetoothSessions.bleWrite(request.payload))
        OP_BLE_SUBSCRIBE -> bluetoothResult(request, bluetoothSessions.bleSubscribe(request.payload))
        OP_BLE_UNSUBSCRIBE -> bluetoothResult(request, bluetoothSessions.bleUnsubscribe(request.payload))
        OP_TASKER_RUN_TASK -> taskerRunTask(request)
        else -> error(request, "DEVICE_OPERATION_NOT_SUPPORTED", "unsupported device operation: ${request.operation}")
    }

    private fun status(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter()
        val powerManager = context.getSystemService(Context.POWER_SERVICE) as? PowerManager
        val interactionState = DeviceInteractionStateReader(context).read()
        return success(
            request,
            mapOf(
                "supported" to true,
                "sdkInt" to Build.VERSION.SDK_INT,
                "accessibilityConnected" to (AccessibilityServiceRegistry.current() != null),
                "locationPermission" to hasAnyLocationPermission(),
                "bluetoothAvailable" to (adapter != null),
                "bluetoothEnabled" to safeBluetoothEnabled(adapter),
                "bluetoothScanPermission" to hasBluetoothScanPermission(),
                "bluetoothConnectPermission" to hasBluetoothConnectPermission(),
                "canWriteSystemSettings" to Settings.System.canWrite(context),
                "canRequestPackageInstalls" to canRequestPackageInstalls(),
                "taskerPermission" to hasPermission(TASKER_PERMISSION_RUN_TASKS),
                "ignoringBatteryOptimizations" to (powerManager?.isIgnoringBatteryOptimizations(context.packageName) ?: false),
            ) + interactionState.asMap(),
        )
    }

    private fun deviceInfo(request: NativeBridgeRequest): NativeBridgeResponse {
        val activityManager = context.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
        val memory = ActivityManager.MemoryInfo().also(activityManager::getMemoryInfo)
        val stat = StatFs(Environment.getDataDirectory().absolutePath)
        val batteryManager = context.getSystemService(Context.BATTERY_SERVICE) as BatteryManager
        val displayMetrics = context.resources.displayMetrics
        val appInfo = context.applicationInfo
        return success(
            request,
            mapOf(
                "manufacturer" to Build.MANUFACTURER,
                "brand" to Build.BRAND,
                "model" to Build.MODEL,
                "device" to Build.DEVICE,
                "product" to Build.PRODUCT,
                "sdkInt" to Build.VERSION.SDK_INT,
                "release" to Build.VERSION.RELEASE,
                "securityPatch" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) Build.VERSION.SECURITY_PATCH else "",
                "supportedAbis" to Build.SUPPORTED_ABIS.toList(),
                "is64Bit" to Build.SUPPORTED_64_BIT_ABIS.isNotEmpty(),
                "screenWidthPx" to displayMetrics.widthPixels,
                "screenHeightPx" to displayMetrics.heightPixels,
                "density" to displayMetrics.density,
                "densityDpi" to displayMetrics.densityDpi,
                "memoryTotalBytes" to memory.totalMem,
                "memoryAvailableBytes" to memory.availMem,
                "memoryLow" to memory.lowMemory,
                "storageTotalBytes" to stat.totalBytes,
                "storageAvailableBytes" to stat.availableBytes,
                "batteryPercent" to batteryManager.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY),
                "packageName" to context.packageName,
                "debuggable" to ((appInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE) != 0),
            ),
        )
    }

    private fun globalAction(request: NativeBridgeRequest): NativeBridgeResponse {
        val service = AccessibilityServiceRegistry.current()
            ?: return error(request, "DEVICE_ACCESSIBILITY_REQUIRED", "accessibility service is not connected")
        val actionName = request.string("action").lowercase(Locale.US)
        val action = when (actionName) {
            "back" -> AccessibilityService.GLOBAL_ACTION_BACK
            "home" -> AccessibilityService.GLOBAL_ACTION_HOME
            "recents" -> AccessibilityService.GLOBAL_ACTION_RECENTS
            "notifications" -> AccessibilityService.GLOBAL_ACTION_NOTIFICATIONS
            "quick_settings" -> AccessibilityService.GLOBAL_ACTION_QUICK_SETTINGS
            "power_dialog" -> AccessibilityService.GLOBAL_ACTION_POWER_DIALOG
            "split_screen" -> AccessibilityService.GLOBAL_ACTION_TOGGLE_SPLIT_SCREEN
            "lock_screen" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) AccessibilityService.GLOBAL_ACTION_LOCK_SCREEN else -1
            "take_screenshot" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) AccessibilityService.GLOBAL_ACTION_TAKE_SCREENSHOT else -1
            else -> return error(request, "DEVICE_INVALID_ACTION", "unsupported global action: $actionName")
        }
        if (action < 0) {
            return error(request, "DEVICE_API_UNAVAILABLE", "global action $actionName is unavailable on this Android version")
        }
        return try {
            val performed = service.performGlobalAction(action)
            if (!performed) {
                error(request, "DEVICE_GLOBAL_ACTION_FAILED", "Android rejected global action: $actionName")
            } else {
                success(request, mapOf("performed" to true, "action" to actionName))
            }
        } catch (t: Throwable) {
            error(request, "DEVICE_GLOBAL_ACTION_FAILED", t.message ?: "global action failed")
        }
    }

    private fun pressKey(request: NativeBridgeRequest): NativeBridgeResponse {
        val key = request.string("key").lowercase(Locale.US)
        if (key in setOf("back", "home", "recents", "notifications", "quick_settings", "power_dialog")) {
            val forwarded = request.copy(payload = mapOf("action" to key))
            return globalAction(forwarded)
        }
        val audio = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
        return try {
            when (key) {
                "volume_up" -> audio.adjustVolume(AudioManager.ADJUST_RAISE, AudioManager.FLAG_SHOW_UI)
                "volume_down" -> audio.adjustVolume(AudioManager.ADJUST_LOWER, AudioManager.FLAG_SHOW_UI)
                "mute" -> audio.adjustVolume(AudioManager.ADJUST_TOGGLE_MUTE, AudioManager.FLAG_SHOW_UI)
                "media_play_pause" -> dispatchMediaKey(audio, KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE)
                "media_next" -> dispatchMediaKey(audio, KeyEvent.KEYCODE_MEDIA_NEXT)
                "media_previous" -> dispatchMediaKey(audio, KeyEvent.KEYCODE_MEDIA_PREVIOUS)
                else -> return error(
                    request,
                    "DEVICE_KEY_UNSUPPORTED",
                    "arbitrary key injection is not available to an ordinary Android app; use an explicitly authorized Shizuku/ADB/Root provider",
                )
            }
            success(request, mapOf("performed" to true, "key" to key, "strategy" to "native"))
        } catch (t: Throwable) {
            error(request, "DEVICE_KEY_FAILED", t.message ?: "key operation failed")
        }
    }

    private fun dispatchMediaKey(audio: AudioManager, keyCode: Int) {
        val down = KeyEvent(KeyEvent.ACTION_DOWN, keyCode)
        val up = KeyEvent(KeyEvent.ACTION_UP, keyCode)
        audio.dispatchMediaKeyEvent(down)
        audio.dispatchMediaKeyEvent(up)
    }

    private fun currentLocation(request: NativeBridgeRequest): NativeBridgeResponse {
        if (!hasAnyLocationPermission()) {
            return error(request, "DEVICE_LOCATION_PERMISSION_REQUIRED", "location permission is required")
        }
        val manager = context.getSystemService(Context.LOCATION_SERVICE) as LocationManager
        val timeoutMs = request.long("timeoutMs", 6000L).coerceIn(500L, 15_000L)
        val maxAgeMs = request.long("maxAgeMs", 120_000L).coerceAtLeast(0L)
        val freshRequired = request.payload["fresh"] == true
        val providers = preferredLocationProviders(manager)
        val now = System.currentTimeMillis()
        val cached = providers.mapNotNull { provider ->
            runCatching { manager.getLastKnownLocation(provider) }.getOrNull()
        }.maxByOrNull { it.time }
        if (!freshRequired && cached != null && now - cached.time <= maxAgeMs) {
            return success(request, locationResult(cached, "last_known"))
        }

        val fresh = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            getCurrentLocationApi30(manager, providers.firstOrNull(), timeoutMs)
        } else {
            getCurrentLocationLegacy(manager, providers.firstOrNull(), timeoutMs)
        }
        if (fresh != null) {
            return success(request, locationResult(fresh, "current"))
        }
        if (freshRequired) {
            return error(request, "DEVICE_LOCATION_TIMEOUT", "fresh location timed out")
        }
        if (cached != null) {
            return success(request, locationResult(cached, "stale_last_known") + mapOf("warning" to "fresh location timed out"))
        }
        return error(request, "DEVICE_LOCATION_UNAVAILABLE", "no location fix is available")
    }

    private fun preferredLocationProviders(manager: LocationManager): List<String> {
        val all = runCatching { manager.getProviders(true) }.getOrDefault(emptyList())
        return listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER, LocationManager.PASSIVE_PROVIDER)
            .filter { it in all }
    }

    private fun getCurrentLocationApi30(manager: LocationManager, provider: String?, timeoutMs: Long): Location? {
        if (provider == null) return null
        val result = AtomicReference<Location?>(null)
        val latch = CountDownLatch(1)
        return try {
            manager.getCurrentLocation(provider, null, locationExecutor) { location ->
                result.set(location)
                latch.countDown()
            }
            latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            result.get()
        } catch (_: SecurityException) {
            null
        } catch (_: Throwable) {
            null
        }
    }

    @Suppress("DEPRECATION")
    private fun getCurrentLocationLegacy(manager: LocationManager, provider: String?, timeoutMs: Long): Location? {
        if (provider == null) return null
        val result = AtomicReference<Location?>(null)
        val latch = CountDownLatch(1)
        val listener = object : LocationListener {
            override fun onLocationChanged(location: Location) {
                result.set(location)
                latch.countDown()
            }
            override fun onStatusChanged(provider: String?, status: Int, extras: Bundle?) = Unit
            override fun onProviderEnabled(provider: String) = Unit
            override fun onProviderDisabled(provider: String) = Unit
        }
        return try {
            manager.requestSingleUpdate(provider, listener, android.os.Looper.getMainLooper())
            latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            runCatching { manager.removeUpdates(listener) }
            result.get()
        } catch (_: SecurityException) {
            null
        } catch (_: Throwable) {
            runCatching { manager.removeUpdates(listener) }
            null
        }
    }

    private fun locationResult(location: Location, source: String): Map<String, Any?> = mapOf(
        "latitude" to location.latitude,
        "longitude" to location.longitude,
        "accuracyMeters" to if (location.hasAccuracy()) location.accuracy.toDouble() else null,
        "altitudeMeters" to if (location.hasAltitude()) location.altitude else null,
        "bearingDegrees" to if (location.hasBearing()) location.bearing.toDouble() else null,
        "speedMetersPerSecond" to if (location.hasSpeed()) location.speed.toDouble() else null,
        "provider" to location.provider,
        "timeMs" to location.time,
        "source" to source,
        "mock" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) location.isMock else false,
    )

    private fun geofenceAdd(request: NativeBridgeRequest): NativeBridgeResponse {
        val id = request.string("id")
        val latitude = request.double("latitude", Double.NaN)
        val longitude = request.double("longitude", Double.NaN)
        val radiusMeters = request.double("radiusMeters", 100.0).toFloat()
        val expirationMs = request.long("expirationMs", -1L)
        val result = GeofenceAutomationManager.add(context, id, latitude, longitude, radiusMeters, expirationMs)
        return result.fold(
            onSuccess = { fence -> success(request, fence.toMap()) },
            onFailure = { t ->
                val message = t.message ?: "geofence registration failed"
                val code = when {
                    message.contains("BACKGROUND_LOCATION", ignoreCase = true) -> "DEVICE_BACKGROUND_LOCATION_PERMISSION_REQUIRED"
                    message.contains("FINE_LOCATION", ignoreCase = true) -> "DEVICE_LOCATION_PERMISSION_REQUIRED"
                    else -> "DEVICE_GEOFENCE_FAILED"
                }
                error(request, code, message)
            },
        )
    }

    private fun geofenceRemove(request: NativeBridgeRequest): NativeBridgeResponse {
        val result = GeofenceAutomationManager.remove(context, request.string("id"))
        return result.fold(
            onSuccess = { success(request, mapOf("removed" to it, "id" to request.string("id"))) },
            onFailure = { error(request, "DEVICE_GEOFENCE_FAILED", it.message ?: "geofence removal failed") },
        )
    }

    private fun geofenceList(request: NativeBridgeRequest): NativeBridgeResponse {
        val items = GeofenceAutomationManager.list(context).map { it.toMap() }
        return success(request, mapOf("items" to items, "count" to items.size))
    }

    private fun settingsGet(request: NativeBridgeRequest): NativeBridgeResponse {
        val namespace = request.string("namespace").lowercase(Locale.US)
        val key = request.string("key")
        if (key.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "key is required")
        val value = try {
            when (namespace) {
                "system" -> Settings.System.getString(context.contentResolver, key)
                "secure" -> Settings.Secure.getString(context.contentResolver, key)
                "global" -> Settings.Global.getString(context.contentResolver, key)
                else -> return error(request, "DEVICE_INVALID_REQUEST", "namespace must be system, secure or global")
            }
        } catch (t: Throwable) {
            return error(request, "DEVICE_SETTINGS_READ_FAILED", t.message ?: "settings read failed")
        }
        return success(request, mapOf("namespace" to namespace, "key" to key, "value" to value, "present" to (value != null)))
    }

    private fun settingsSet(request: NativeBridgeRequest): NativeBridgeResponse {
        val namespace = request.string("namespace").lowercase(Locale.US)
        val key = request.string("key")
        val value = request.payload["value"]?.toString()
        if (key.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "key is required")

        val allowed = when (namespace) {
            "system" -> Settings.System.canWrite(context)
            "secure", "global" -> hasPermission(Manifest.permission.WRITE_SECURE_SETTINGS)
            else -> return error(request, "DEVICE_INVALID_REQUEST", "namespace must be system, secure or global")
        }
        if (!allowed) {
            val code = if (namespace == "system") "DEVICE_USER_ACTION_REQUIRED" else "DEVICE_PRIVILEGE_REQUIRED"
            return error(request, code, "write access is not granted for $namespace settings")
        }
        val changed = try {
            when (namespace) {
                "system" -> Settings.System.putString(context.contentResolver, key, value)
                "secure" -> Settings.Secure.putString(context.contentResolver, key, value)
                "global" -> Settings.Global.putString(context.contentResolver, key, value)
                else -> false
            }
        } catch (t: Throwable) {
            return error(request, "DEVICE_SETTINGS_WRITE_FAILED", t.message ?: "settings write failed")
        }
        if (!changed) return error(request, "DEVICE_SETTINGS_WRITE_FAILED", "Android rejected settings change")
        return success(request, mapOf("changed" to true, "namespace" to namespace, "key" to key))
    }

    private fun appList(request: NativeBridgeRequest): NativeBridgeResponse {
        val query = request.string("query").trim()
        val limit = request.int("limit", 100).coerceIn(1, 500)
        val pm = context.packageManager
        val launcher = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        val apps = try {
            pm.queryIntentActivities(launcher, 0)
                .asSequence()
                .map { info ->
                    val pkg = info.activityInfo.packageName
                    val label = info.loadLabel(pm).toString()
                    mapOf(
                        "packageName" to pkg,
                        "label" to label,
                        "mainActivity" to info.activityInfo.name,
                        "systemApp" to ((info.activityInfo.applicationInfo.flags and ApplicationInfo.FLAG_SYSTEM) != 0),
                    )
                }
                .distinctBy { it["packageName"] }
                .filter { item ->
                    query.isBlank() || item["packageName"].toString().contains(query, true) || item["label"].toString().contains(query, true)
                }
                .take(limit)
                .toList()
        } catch (t: Throwable) {
            return error(request, "DEVICE_APP_LIST_FAILED", t.message ?: "app list failed")
        }
        return success(request, mapOf("apps" to apps, "count" to apps.size, "visibility" to "launcher_visible"))
    }

    @Suppress("DEPRECATION")
    private fun packageInfo(packageName: String): PackageInfo = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
        context.packageManager.getPackageInfo(packageName, PackageManager.PackageInfoFlags.of(0))
    } else {
        context.packageManager.getPackageInfo(packageName, 0)
    }

    private fun appInfo(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.string("packageName")
        if (packageName.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "packageName is required")
        val pm = context.packageManager
        return try {
            val info = packageInfo(packageName)
            val app = info.applicationInfo ?: throw PackageManager.NameNotFoundException(packageName)
            @Suppress("DEPRECATION")
            val versionCode = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) info.longVersionCode else info.versionCode.toLong()
            success(
                request,
                mapOf(
                    "packageName" to packageName,
                    "label" to pm.getApplicationLabel(app).toString(),
                    "versionName" to info.versionName,
                    "versionCode" to versionCode,
                    "enabled" to app.enabled,
                    "systemApp" to ((app.flags and ApplicationInfo.FLAG_SYSTEM) != 0),
                    "debuggable" to ((app.flags and ApplicationInfo.FLAG_DEBUGGABLE) != 0),
                    "targetSdk" to app.targetSdkVersion,
                    "sourceDir" to app.sourceDir,
                    "launchable" to (pm.getLaunchIntentForPackage(packageName) != null),
                ),
            )
        } catch (_: PackageManager.NameNotFoundException) {
            error(request, "DEVICE_APP_NOT_FOUND", "application is not visible or not installed: $packageName")
        } catch (t: Throwable) {
            error(request, "DEVICE_APP_INFO_FAILED", t.message ?: "app info failed")
        }
    }

    private fun appOpen(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.string("packageName")
        if (packageName.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "packageName is required")
        val intent = context.packageManager.getLaunchIntentForPackage(packageName)
            ?: return error(request, "DEVICE_APP_NOT_LAUNCHABLE", "application has no launchable activity: $packageName")
        return try {
            context.startActivity(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            success(request, mapOf("launched" to true, "packageName" to packageName))
        } catch (t: Throwable) {
            error(request, "DEVICE_APP_OPEN_FAILED", t.message ?: "app open failed")
        }
    }

    private fun appStop(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.string("packageName")
        if (packageName.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "packageName is required")
        if (!hasPermission(Manifest.permission.KILL_BACKGROUND_PROCESSES)) {
            return error(request, "DEVICE_PERMISSION_REQUIRED", "KILL_BACKGROUND_PROCESSES permission is unavailable")
        }
        return try {
            val manager = context.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
            manager.killBackgroundProcesses(packageName)
            success(
                request,
                mapOf(
                    "requested" to true,
                    "packageName" to packageName,
                    "forceStop" to false,
                    "warning" to "Android public API can only request background-process termination; true force-stop requires an authorized privileged provider",
                ),
            )
        } catch (t: Throwable) {
            error(request, "DEVICE_APP_STOP_FAILED", t.message ?: "app stop failed")
        }
    }

    private fun appInstall(request: NativeBridgeRequest): NativeBridgeResponse {
        val uriRaw = request.string("uri")
        if (uriRaw.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "content uri is required")
        val uri = runCatching { Uri.parse(uriRaw) }.getOrNull()
            ?: return error(request, "DEVICE_INVALID_REQUEST", "invalid install uri")
        if (uri.scheme != "content") {
            return error(request, "DEVICE_INVALID_REQUEST", "install requires a content:// URI so Android can grant scoped read access")
        }
        if (!canRequestPackageInstalls()) {
            return error(request, "DEVICE_USER_ACTION_REQUIRED", "install unknown apps access is not granted for Amitia")
        }
        val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
            data = uri
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION)
            putExtra(Intent.EXTRA_NOT_UNKNOWN_SOURCE, true)
        }
        return try {
            context.startActivity(intent)
            success(request, mapOf("started" to true, "requiresUserConfirmation" to true, "uri" to uri.toString()))
        } catch (t: Throwable) {
            error(request, "DEVICE_APP_INSTALL_FAILED", t.message ?: "package installer could not be opened")
        }
    }

    private fun appUninstall(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.string("packageName")
        if (packageName.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "packageName is required")
        val intent = Intent(Intent.ACTION_DELETE, Uri.parse("package:$packageName")).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        return try {
            context.startActivity(intent)
            success(request, mapOf("started" to true, "requiresUserConfirmation" to true, "packageName" to packageName))
        } catch (t: Throwable) {
            error(request, "DEVICE_APP_UNINSTALL_FAILED", t.message ?: "uninstaller could not be opened")
        }
    }

    private fun bluetoothStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter()
            ?: return success(request, mapOf("available" to false, "enabled" to false))
        return success(
            request,
            mapOf(
                "available" to true,
                "enabled" to safeBluetoothEnabled(adapter),
                "scanPermission" to hasBluetoothScanPermission(),
                "connectPermission" to hasBluetoothConnectPermission(),
                "state" to safeBluetoothState(adapter),
                "name" to if (hasBluetoothConnectPermission()) runCatching { adapter.name }.getOrNull() else null,
                "address" to if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || hasBluetoothConnectPermission()) runCatching { adapter.address }.getOrNull() else null,
            ),
        )
    }

    private fun bluetoothRequestEnable(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter() ?: return error(request, "DEVICE_BLUETOOTH_UNAVAILABLE", "Bluetooth adapter is unavailable")
        if (!hasBluetoothConnectPermission()) {
            return error(request, "DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        }
        if (safeBluetoothEnabled(adapter)) {
            return success(request, mapOf("alreadyEnabled" to true, "started" to false, "requiresUserConfirmation" to false))
        }
        return try {
            context.startActivity(Intent(BluetoothAdapter.ACTION_REQUEST_ENABLE).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            success(request, mapOf("alreadyEnabled" to false, "started" to true, "requiresUserConfirmation" to true))
        } catch (t: Throwable) {
            error(request, "DEVICE_BLUETOOTH_ENABLE_FAILED", t.message ?: "Bluetooth enable confirmation could not be opened")
        }
    }

    private fun bluetoothPair(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter() ?: return error(request, "DEVICE_BLUETOOTH_UNAVAILABLE", "Bluetooth adapter is unavailable")
        if (!hasBluetoothConnectPermission()) {
            return error(request, "DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        }
        if (!safeBluetoothEnabled(adapter)) return error(request, "DEVICE_BLUETOOTH_DISABLED", "Bluetooth is disabled")
        val address = request.string("address").uppercase(Locale.US)
        if (!BluetoothAdapter.checkBluetoothAddress(address)) return error(request, "DEVICE_INVALID_REQUEST", "invalid Bluetooth address")
        val timeoutMs = request.long("timeoutMs", 15_000L).coerceIn(1000L, 30_000L)
        val device = runCatching { adapter.getRemoteDevice(address) }.getOrElse {
            return error(request, "DEVICE_BLUETOOTH_DEVICE_INVALID", it.message ?: "Bluetooth device could not be resolved")
        }
        if (runCatching { device.bondState == BluetoothDevice.BOND_BONDED }.getOrDefault(false)) {
            return success(request, mapOf("address" to address, "requested" to false, "bonded" to true, "bondState" to BluetoothDevice.BOND_BONDED))
        }
        val latch = CountDownLatch(1)
        val finalState = AtomicReference(runCatching { device.bondState }.getOrDefault(BluetoothDevice.BOND_NONE))
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                if (intent?.action != BluetoothDevice.ACTION_BOND_STATE_CHANGED) return
                val changed = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE, BluetoothDevice::class.java)
                } else {
                    @Suppress("DEPRECATION") intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE)
                } ?: return
                if (!runCatching { changed.address.equals(address, true) }.getOrDefault(false)) return
                val state = intent.getIntExtra(BluetoothDevice.EXTRA_BOND_STATE, BluetoothDevice.BOND_NONE)
                finalState.set(state)
                if (state == BluetoothDevice.BOND_BONDED || state == BluetoothDevice.BOND_NONE) latch.countDown()
            }
        }
        return try {
            registerReceiverCompat(receiver, IntentFilter(BluetoothDevice.ACTION_BOND_STATE_CHANGED))
            val requested = runCatching { device.createBond() }.getOrElse {
                return error(request, "DEVICE_BLUETOOTH_PAIR_FAILED", it.message ?: "Bluetooth bonding request failed")
            }
            if (!requested) {
                val state = runCatching { device.bondState }.getOrDefault(finalState.get())
                return success(request, mapOf("address" to address, "requested" to false, "bonded" to (state == BluetoothDevice.BOND_BONDED), "bondState" to state))
            }
            latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            val state = runCatching { device.bondState }.getOrDefault(finalState.get())
            success(request, mapOf("address" to address, "requested" to true, "bonded" to (state == BluetoothDevice.BOND_BONDED), "bondState" to state, "pending" to (state == BluetoothDevice.BOND_BONDING)))
        } catch (t: Throwable) {
            error(request, "DEVICE_BLUETOOTH_PAIR_FAILED", t.message ?: "Bluetooth pairing failed")
        } finally {
            runCatching { context.unregisterReceiver(receiver) }
        }
    }

    private fun bluetoothPaired(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter() ?: return error(request, "DEVICE_BLUETOOTH_UNAVAILABLE", "Bluetooth adapter is unavailable")
        if (!hasBluetoothConnectPermission()) {
            return error(request, "DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        }
        val devices = try {
            adapter.bondedDevices.orEmpty().map(::bluetoothDeviceMap)
        } catch (t: Throwable) {
            return error(request, "DEVICE_BLUETOOTH_FAILED", t.message ?: "failed to read paired devices")
        }
        return success(request, mapOf("devices" to devices, "count" to devices.size))
    }

    private fun bluetoothScan(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter() ?: return error(request, "DEVICE_BLUETOOTH_UNAVAILABLE", "Bluetooth adapter is unavailable")
        if (!hasBluetoothScanPermission()) {
            return error(request, "DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth scan permission is required")
        }
        if (!safeBluetoothEnabled(adapter)) return error(request, "DEVICE_BLUETOOTH_DISABLED", "Bluetooth is disabled")
        val timeoutMs = request.long("timeoutMs", 6000L).coerceIn(1000L, 12_000L)
        val found = linkedMapOf<String, Map<String, Any?>>()
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                if (intent?.action != BluetoothDevice.ACTION_FOUND) return
                val device = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE, BluetoothDevice::class.java)
                } else {
                    @Suppress("DEPRECATION") intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE)
                } ?: return
                val mapped = bluetoothDeviceMap(device)
                val key = mapped["address"]?.toString().orEmpty().ifBlank { mapped["name"]?.toString().orEmpty() }
                if (key.isNotBlank()) synchronized(found) { found[key] = mapped }
            }
        }
        return try {
            registerReceiverCompat(receiver, IntentFilter(BluetoothDevice.ACTION_FOUND))
            adapter.cancelDiscovery()
            val started = adapter.startDiscovery()
            if (!started) return error(request, "DEVICE_BLUETOOTH_SCAN_FAILED", "Android did not start classic Bluetooth discovery")
            Thread.sleep(timeoutMs)
            adapter.cancelDiscovery()
            val devices = synchronized(found) { found.values.toList() }
            success(request, mapOf("devices" to devices, "count" to devices.size, "durationMs" to timeoutMs))
        } catch (t: Throwable) {
            error(request, "DEVICE_BLUETOOTH_SCAN_FAILED", t.message ?: "Bluetooth discovery failed")
        } finally {
            runCatching { context.unregisterReceiver(receiver) }
        }
    }

    private fun bleScan(request: NativeBridgeRequest): NativeBridgeResponse {
        val adapter = bluetoothAdapter() ?: return error(request, "DEVICE_BLUETOOTH_UNAVAILABLE", "Bluetooth adapter is unavailable")
        if (!hasBluetoothScanPermission()) {
            return error(request, "DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth scan permission is required")
        }
        if (!safeBluetoothEnabled(adapter)) return error(request, "DEVICE_BLUETOOTH_DISABLED", "Bluetooth is disabled")
        val scanner = runCatching { adapter.bluetoothLeScanner }.getOrNull()
            ?: return error(request, "DEVICE_BLE_UNAVAILABLE", "BLE scanner is unavailable")
        val timeoutMs = request.long("timeoutMs", 6000L).coerceIn(1000L, 12_000L)
        val found = linkedMapOf<String, Map<String, Any?>>()
        val callback = object : ScanCallback() {
            override fun onScanResult(callbackType: Int, result: ScanResult?) {
                result ?: return
                val device = result.device
                val address = runCatching { device.address }.getOrNull().orEmpty()
                val row = bluetoothDeviceMap(device).toMutableMap().apply {
                    put("rssi", result.rssi)
                    put("connectable", if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) result.isConnectable else null)
                    put("advertisingSid", if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) result.advertisingSid else null)
                    put("serviceUuids", result.scanRecord?.serviceUuids?.map { it.uuid.toString() }.orEmpty())
                }
                synchronized(found) { found[address.ifBlank { row["name"].toString() }] = row }
            }
        }
        return try {
            scanner.startScan(callback)
            Thread.sleep(timeoutMs)
            scanner.stopScan(callback)
            val devices = synchronized(found) { found.values.toList() }
            success(request, mapOf("devices" to devices, "count" to devices.size, "durationMs" to timeoutMs))
        } catch (t: Throwable) {
            runCatching { scanner.stopScan(callback) }
            error(request, "DEVICE_BLE_SCAN_FAILED", t.message ?: "BLE scan failed")
        }
    }

    private fun bluetoothAdapterRequired(
        request: NativeBridgeRequest,
        block: (BluetoothAdapter) -> BluetoothAutomationManager.Result,
    ): BluetoothAutomationManager.Result {
        val adapter = bluetoothAdapter()
            ?: return BluetoothAutomationManager.Result(code = "DEVICE_BLUETOOTH_UNAVAILABLE", message = "Bluetooth adapter is unavailable")
        if (!safeBluetoothEnabled(adapter)) {
            return BluetoothAutomationManager.Result(code = "DEVICE_BLUETOOTH_DISABLED", message = "Bluetooth is disabled or connect permission is missing")
        }
        return block(adapter)
    }

    private fun bluetoothResult(request: NativeBridgeRequest, result: BluetoothAutomationManager.Result): NativeBridgeResponse {
        return if (result.successful) {
            success(request, result.value.orEmpty())
        } else {
            error(request, result.code ?: "DEVICE_BLUETOOTH_FAILED", result.message ?: "Bluetooth operation failed")
        }
    }

    private fun taskerRunTask(request: NativeBridgeRequest): NativeBridgeResponse {
        val taskName = request.string("taskName")
        if (taskName.isBlank()) return error(request, "DEVICE_INVALID_REQUEST", "taskName is required")
        if (!hasPermission(TASKER_PERMISSION_RUN_TASKS)) {
            return error(
                request,
                "DEVICE_TASKER_PERMISSION_REQUIRED",
                "Tasker RUN_TASKS permission is not granted; Tasker also requires Allow External Access",
            )
        }
        val packageName = installedTaskerPackage()
            ?: return error(request, "DEVICE_TASKER_NOT_INSTALLED", "Tasker is not installed")
        val names = arrayListOf<String>()
        val values = arrayListOf<String>()
        val parameters = request.payload["parameters"] as? List<*>
        parameters?.take(32)?.forEachIndexed { index, value ->
            names += "%par${index + 1}"
            values += value?.toString().orEmpty()
        }
        val variables = request.payload["variables"] as? Map<*, *>
        variables?.entries?.take(64)?.forEach { (key, value) ->
            var normalized = key?.toString()?.trim().orEmpty()
            if (!normalized.startsWith("%")) normalized = "%$normalized"
            if (normalized.matches(Regex("^%[a-z][a-z0-9_]{2,63}$")) && !names.contains(normalized)) {
                names += normalized
                values += value?.toString().orEmpty()
            }
        }
        val intent = Intent(TASKER_ACTION_TASK).apply {
            setPackage(packageName)
            data = Uri.parse("id:${java.util.UUID.randomUUID()}")
            putExtra(TASKER_EXTRA_INTENT_VERSION, TASKER_INTENT_VERSION)
            putExtra(TASKER_EXTRA_TASK_NAME, taskName)
            putStringArrayListExtra(TASKER_EXTRA_VAR_NAMES, names)
            putStringArrayListExtra(TASKER_EXTRA_VAR_VALUES, values)
        }
        return try {
            context.sendBroadcast(intent, TASKER_PERMISSION_RUN_TASKS)
            success(
                request,
                mapOf(
                    "sent" to true,
                    "taskName" to taskName,
                    "taskerPackage" to packageName,
                    "parameterCount" to (parameters?.take(32)?.size ?: 0),
                    "variableCount" to (names.size - (parameters?.take(32)?.size ?: 0)),
                    "requiresTaskerExternalAccess" to true,
                ),
            )
        } catch (t: Throwable) {
            error(request, "DEVICE_TASKER_FAILED", t.message ?: "Tasker broadcast failed")
        }
    }

    private fun installedTaskerPackage(): String? {
        val pm = context.packageManager
        return listOf(TASKER_PACKAGE_MARKET, TASKER_PACKAGE_DIRECT).firstOrNull { packageName ->
            runCatching {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    pm.getPackageInfo(packageName, PackageManager.PackageInfoFlags.of(0))
                } else {
                    @Suppress("DEPRECATION") pm.getPackageInfo(packageName, 0)
                }
            }.isSuccess
        }
    }

    private fun bluetoothDeviceMap(device: BluetoothDevice): Map<String, Any?> {
        val canConnect = hasBluetoothConnectPermission()
        val name = if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || canConnect) runCatching { device.name }.getOrNull() else null
        val alias = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R && canConnect) runCatching { device.alias }.getOrNull() else null
        val bondState = if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || canConnect) runCatching { device.bondState }.getOrNull() else null
        val type = if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || canConnect) runCatching { device.type }.getOrNull() else null
        return mapOf(
            "address" to runCatching { device.address }.getOrNull(),
            "name" to name,
            "alias" to alias,
            "bondState" to bondState,
            "type" to type,
        )
    }

    private fun bluetoothAdapter(): BluetoothAdapter? {
        val manager = context.getSystemService(Context.BLUETOOTH_SERVICE) as? BluetoothManager
        return manager?.adapter
    }

    private fun safeBluetoothEnabled(adapter: BluetoothAdapter?): Boolean {
        if (adapter == null) return false
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S && !hasBluetoothConnectPermission()) return false
        return runCatching { adapter.isEnabled }.getOrDefault(false)
    }

    private fun safeBluetoothState(adapter: BluetoothAdapter): Int? {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S && !hasBluetoothConnectPermission()) return null
        return runCatching { adapter.state }.getOrNull()
    }

    private fun hasBluetoothScanPermission(): Boolean = when {
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> hasPermission(Manifest.permission.BLUETOOTH_SCAN)
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.M -> hasAnyLocationPermission()
        else -> true
    }

    private fun hasBluetoothConnectPermission(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.S || hasPermission(Manifest.permission.BLUETOOTH_CONNECT)

    private fun hasAnyLocationPermission(): Boolean =
        hasPermission(Manifest.permission.ACCESS_FINE_LOCATION) || hasPermission(Manifest.permission.ACCESS_COARSE_LOCATION)

    private fun hasPermission(permission: String): Boolean =
        ContextCompat.checkSelfPermission(context, permission) == PackageManager.PERMISSION_GRANTED

    private fun canRequestPackageInstalls(): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.O || context.packageManager.canRequestPackageInstalls()

    private fun registerReceiverCompat(receiver: BroadcastReceiver, filter: IntentFilter) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            context.registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION") context.registerReceiver(receiver, filter)
        }
    }

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun error(request: NativeBridgeRequest, code: String, message: String): NativeBridgeResponse = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message),
    )

    private fun NativeBridgeRequest.string(key: String): String = payload[key]?.toString()?.trim().orEmpty()
    private fun NativeBridgeRequest.long(key: String, default: Long): Long = (payload[key] as? Number)?.toLong() ?: default
    private fun NativeBridgeRequest.double(key: String, default: Double): Double = (payload[key] as? Number)?.toDouble() ?: default
    private fun NativeBridgeRequest.int(key: String, default: Int): Int = (payload[key] as? Number)?.toInt() ?: default

    companion object {
        const val OP_STATUS = "device.status"
        const val OP_DEVICE_INFO = "device.info"
        const val OP_GLOBAL_ACTION = "device.global_action"
        const val OP_PRESS_KEY = "device.press_key"
        const val OP_LOCATION_CURRENT = "device.location_current"
        const val OP_GEOFENCE_ADD = "device.geofence_add"
        const val OP_GEOFENCE_REMOVE = "device.geofence_remove"
        const val OP_GEOFENCE_LIST = "device.geofence_list"
        const val OP_SETTINGS_GET = "device.settings_get"
        const val OP_SETTINGS_SET = "device.settings_set"
        const val OP_APP_LIST = "device.app_list"
        const val OP_APP_INFO = "device.app_info"
        const val OP_APP_OPEN = "device.app_open"
        const val OP_APP_STOP = "device.app_stop"
        const val OP_APP_INSTALL = "device.app_install"
        const val OP_APP_UNINSTALL = "device.app_uninstall"
        const val OP_BLUETOOTH_STATUS = "device.bluetooth_status"
        const val OP_BLUETOOTH_REQUEST_ENABLE = "device.bluetooth_request_enable"
        const val OP_BLUETOOTH_PAIR = "device.bluetooth_pair"
        const val OP_BLUETOOTH_PAIRED = "device.bluetooth_paired"
        const val OP_BLUETOOTH_SCAN = "device.bluetooth_scan"
        const val OP_BLE_SCAN = "device.ble_scan"
        const val OP_BLUETOOTH_CLASSIC_CONNECT = "device.bluetooth_classic_connect"
        const val OP_BLUETOOTH_CLASSIC_DISCONNECT = "device.bluetooth_classic_disconnect"
        const val OP_BLUETOOTH_CLASSIC_READ = "device.bluetooth_classic_read"
        const val OP_BLUETOOTH_CLASSIC_WRITE = "device.bluetooth_classic_write"
        const val OP_BLUETOOTH_CLASSIC_LISTEN = "device.bluetooth_classic_listen"
        const val OP_BLUETOOTH_CLASSIC_ACCEPT = "device.bluetooth_classic_accept"
        const val OP_BLUETOOTH_CLASSIC_CLOSE_SERVER = "device.bluetooth_classic_close_server"
        const val OP_BLE_CONNECT = "device.ble_connect"
        const val OP_BLE_DISCONNECT = "device.ble_disconnect"
        const val OP_BLE_SERVICES = "device.ble_services"
        const val OP_BLE_CHARACTERISTICS = "device.ble_characteristics"
        const val OP_BLE_READ = "device.ble_read"
        const val OP_BLE_WRITE = "device.ble_write"
        const val OP_BLE_SUBSCRIBE = "device.ble_subscribe"
        const val OP_BLE_UNSUBSCRIBE = "device.ble_unsubscribe"
        const val OP_TASKER_RUN_TASK = "device.tasker_run_task"

        private const val TASKER_PACKAGE_MARKET = "net.dinglisch.android.taskerm"
        private const val TASKER_PACKAGE_DIRECT = "net.dinglisch.android.tasker"
        private const val TASKER_ACTION_TASK = "net.dinglisch.android.tasker.ACTION_TASK"
        private const val TASKER_EXTRA_TASK_NAME = "task_name"
        private const val TASKER_EXTRA_INTENT_VERSION = "version_number"
        private const val TASKER_INTENT_VERSION = "1.1"
        private const val TASKER_EXTRA_VAR_NAMES = "varNames"
        private const val TASKER_EXTRA_VAR_VALUES = "varValues"
        private const val TASKER_PERMISSION_RUN_TASKS = "net.dinglisch.android.tasker.PERMISSION_RUN_TASKS"
    }
}
