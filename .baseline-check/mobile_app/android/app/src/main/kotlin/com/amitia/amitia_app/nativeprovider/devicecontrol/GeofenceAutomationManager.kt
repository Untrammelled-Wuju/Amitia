package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.Manifest
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationManager
import android.os.Build
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONArray
import org.json.JSONObject
import java.security.MessageDigest
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

internal object GeofenceAutomationManager {
    private const val PREFS = "amitia_android_geofences"
    private const val KEY_ITEMS = "items"
    const val ACTION_GEOFENCE = "com.amitia.amitia_app.workflow.GEOFENCE_TRANSITION"
    const val EXTRA_FENCE_ID = "fenceId"

    data class Fence(
        val id: String,
        val latitude: Double,
        val longitude: Double,
        val radiusMeters: Float,
        val expirationAt: Long,
    ) {
        fun toMap(): Map<String, Any?> = mapOf(
            "id" to id,
            "latitude" to latitude,
            "longitude" to longitude,
            "radiusMeters" to radiusMeters,
            "expirationAt" to expirationAt.takeIf { it > 0L },
        )
    }

    fun add(
        context: Context,
        id: String,
        latitude: Double,
        longitude: Double,
        radiusMeters: Float,
        expirationMs: Long,
    ): Result<Fence> = runCatching {
        require(id.matches(Regex("^[A-Za-z0-9._:-]{1,128}$"))) { "geofence id contains unsupported characters" }
        require(latitude in -90.0..90.0) { "latitude must be between -90 and 90" }
        require(longitude in -180.0..180.0) { "longitude must be between -180 and 180" }
        require(radiusMeters in 1f..100_000f) { "radiusMeters must be between 1 and 100000" }
        require(expirationMs == -1L || expirationMs in 60_000L..(365L * 24 * 60 * 60 * 1000)) {
            "expirationMs must be -1 or between 60000 and 31536000000"
        }
        requireLocationPermission(context)
        requireBackgroundPermission(context)
        val now = System.currentTimeMillis()
        val expirationAt = if (expirationMs < 0L) 0L else now + expirationMs
        val fence = Fence(id, latitude, longitude, radiusMeters, expirationAt)
        install(context.applicationContext, fence)
        val current = load(context).associateBy { it.id }.toMutableMap()
        current[id] = fence
        persist(context, current.values.sortedBy { it.id })
        fence
    }

    fun remove(context: Context, id: String): Result<Boolean> = runCatching {
        require(id.isNotBlank()) { "geofence id is required" }
        val app = context.applicationContext
        val manager = app.getSystemService(Context.LOCATION_SERVICE) as LocationManager
        pendingIntent(app, id, PendingIntent.FLAG_NO_CREATE)?.let { existing ->
            @Suppress("MissingPermission", "DEPRECATION")
            runCatching { manager.removeProximityAlert(existing) }
        }
        val current = load(app).filterNot { it.id == id }
        persist(app, current)
        true
    }

    fun list(context: Context): List<Fence> = load(context).filter { it.expirationAt == 0L || it.expirationAt > System.currentTimeMillis() }

    fun restore(context: Context) {
        val app = context.applicationContext
        if (!hasFineLocation(app) || !hasBackgroundLocation(app)) return
        val now = System.currentTimeMillis()
        val active = load(app).filter { it.expirationAt == 0L || it.expirationAt > now }
        if (active.size != load(app).size) persist(app, active)
        active.forEach { runCatching { install(app, it) } }
    }

    private fun install(context: Context, fence: Fence) {
        requireLocationPermission(context)
        requireBackgroundPermission(context)
        val manager = context.getSystemService(Context.LOCATION_SERVICE) as LocationManager
        val remaining = if (fence.expirationAt == 0L) -1L else (fence.expirationAt - System.currentTimeMillis()).coerceAtLeast(1L)
        @Suppress("MissingPermission", "DEPRECATION")
        manager.addProximityAlert(
            fence.latitude,
            fence.longitude,
            fence.radiusMeters,
            remaining,
            pendingIntent(context, fence.id, PendingIntent.FLAG_UPDATE_CURRENT),
        )
    }

    private fun pendingIntent(context: Context, id: String, baseFlag: Int): PendingIntent? {
        val intent = Intent(context, WorkflowGeofenceReceiver::class.java).apply {
            action = ACTION_GEOFENCE
            putExtra(EXTRA_FENCE_ID, id)
        }
        val flags = baseFlag or PendingIntent.FLAG_IMMUTABLE
        return PendingIntent.getBroadcast(context, stableRequestCode(id), intent, flags)
    }

    private fun stableRequestCode(id: String): Int {
        val digest = MessageDigest.getInstance("SHA-256").digest(id.toByteArray(Charsets.UTF_8))
        return ((digest[0].toInt() and 0xff) shl 24) or
            ((digest[1].toInt() and 0xff) shl 16) or
            ((digest[2].toInt() and 0xff) shl 8) or
            (digest[3].toInt() and 0xff)
    }

    private fun load(context: Context): List<Fence> {
        val raw = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY_ITEMS, "[]") ?: "[]"
        return runCatching {
            val array = JSONArray(raw)
            buildList {
                for (index in 0 until array.length()) {
                    val item = array.optJSONObject(index) ?: continue
                    val id = item.optString("id").trim()
                    if (id.isBlank()) continue
                    add(
                        Fence(
                            id = id,
                            latitude = item.optDouble("latitude"),
                            longitude = item.optDouble("longitude"),
                            radiusMeters = item.optDouble("radiusMeters").toFloat(),
                            expirationAt = item.optLong("expirationAt", 0L),
                        ),
                    )
                }
            }
        }.getOrDefault(emptyList())
    }

    private fun persist(context: Context, fences: Collection<Fence>) {
        val array = JSONArray()
        fences.take(128).forEach { fence ->
            array.put(
                JSONObject().apply {
                    put("id", fence.id)
                    put("latitude", fence.latitude)
                    put("longitude", fence.longitude)
                    put("radiusMeters", fence.radiusMeters.toDouble())
                    put("expirationAt", fence.expirationAt)
                },
            )
        }
        context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_ITEMS, array.toString())
            .commit()
    }

    private fun requireLocationPermission(context: Context) {
        check(hasFineLocation(context)) { "ACCESS_FINE_LOCATION permission is required" }
    }

    private fun requireBackgroundPermission(context: Context) {
        check(hasBackgroundLocation(context)) { "ACCESS_BACKGROUND_LOCATION permission is required for reliable geofencing" }
    }

    private fun hasFineLocation(context: Context): Boolean =
        ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED

    private fun hasBackgroundLocation(context: Context): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.Q ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_BACKGROUND_LOCATION) == PackageManager.PERMISSION_GRANTED
}

internal class WorkflowGeofenceReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action != GeofenceAutomationManager.ACTION_GEOFENCE) return
        val fenceId = intent.getStringExtra(GeofenceAutomationManager.EXTRA_FENCE_ID)?.trim().orEmpty()
        if (fenceId.isEmpty()) return
        val pending = goAsync()
        EVENT_EXECUTOR.execute {
            try {
                val entering = intent.getBooleanExtra(LocationManager.KEY_PROXIMITY_ENTERING, false)
                @Suppress("DEPRECATION")
                val location = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    intent.getParcelableExtra(LocationManager.KEY_LOCATION_CHANGED, Location::class.java)
                } else {
                    intent.getParcelableExtra(LocationManager.KEY_LOCATION_CHANGED) as? Location
                }
                WorkflowDeviceEventIngress(context.applicationContext).emit(
                    eventType = if (entering) "device.location.geofence.enter" else "device.location.geofence.exit",
                    eventId = "geofence:$fenceId:${if (entering) "enter" else "exit"}:${System.currentTimeMillis()}",
                    payload = buildMap {
                        put("fenceId", fenceId)
                        put("entering", entering)
                        if (location != null) {
                            put("latitude", location.latitude)
                            put("longitude", location.longitude)
                            put("accuracyMeters", location.accuracy)
                            put("provider", location.provider.orEmpty())
                        }
                    },
                )
            } finally {
                pending.finish()
            }
        }
    }

    companion object {
        private val EVENT_EXECUTOR: ExecutorService = Executors.newSingleThreadExecutor { runnable ->
            Thread(runnable, "amitia-geofence-event").apply { isDaemon = true }
        }
    }
}
