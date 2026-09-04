package com.amitia.amitia_app.workflow

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothProfile
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.net.wifi.WifiManager
import android.os.BatteryManager
import android.os.Build
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONObject
import java.security.MessageDigest
import java.util.concurrent.atomic.AtomicBoolean

internal object WorkflowSystemEventRegistrar {
    private val registered = AtomicBoolean(false)
    private val networkRegistered = AtomicBoolean(false)
    private var receiver: BroadcastReceiver? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    fun ensureRegistered(context: Context) {
        val app = context.applicationContext
        val ingress = WorkflowDeviceEventIngress(app)
        if (registered.compareAndSet(false, true)) {
            registerBroadcastReceiver(app, ingress)
        }
        registerNetworkCallback(app, ingress)
    }

    private fun registerBroadcastReceiver(app: Context, ingress: WorkflowDeviceEventIngress) {
        val localReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                val action = intent?.action ?: return
                when (action) {
                    Intent.ACTION_BATTERY_CHANGED -> emitBattery(ingress, intent)
                    Intent.ACTION_BATTERY_LOW -> emitSimple(ingress, "device.power.battery_low", action)
                    Intent.ACTION_BATTERY_OKAY -> emitSimple(ingress, "device.power.battery_okay", action)
                    Intent.ACTION_POWER_CONNECTED -> emitSimple(ingress, "device.power.connected", action)
                    Intent.ACTION_POWER_DISCONNECTED -> emitSimple(ingress, "device.power.disconnected", action)
                    Intent.ACTION_SCREEN_ON -> emitSimple(ingress, "device.screen.on", action)
                    Intent.ACTION_SCREEN_OFF -> emitSimple(ingress, "device.screen.off", action)
                    Intent.ACTION_USER_PRESENT -> emitSimple(ingress, "device.user.present", action)
                    Intent.ACTION_HEADSET_PLUG -> emitHeadset(ingress, intent)
                    BluetoothAdapter.ACTION_STATE_CHANGED -> emitBluetoothState(ingress, intent)
                    BluetoothAdapter.ACTION_CONNECTION_STATE_CHANGED -> emitBluetoothConnection(ingress, intent)
                    WifiManager.WIFI_STATE_CHANGED_ACTION -> emitWifiState(ingress, intent)
                    WifiManager.NETWORK_STATE_CHANGED_ACTION -> emitWifiNetwork(ingress, intent)
                }
            }
        }
        val filter = IntentFilter().apply {
            addAction(Intent.ACTION_BATTERY_CHANGED)
            addAction(Intent.ACTION_BATTERY_LOW)
            addAction(Intent.ACTION_BATTERY_OKAY)
            addAction(Intent.ACTION_POWER_CONNECTED)
            addAction(Intent.ACTION_POWER_DISCONNECTED)
            addAction(Intent.ACTION_SCREEN_ON)
            addAction(Intent.ACTION_SCREEN_OFF)
            addAction(Intent.ACTION_USER_PRESENT)
            addAction(Intent.ACTION_HEADSET_PLUG)
            addAction(BluetoothAdapter.ACTION_STATE_CHANGED)
            addAction(BluetoothAdapter.ACTION_CONNECTION_STATE_CHANGED)
            addAction(WifiManager.WIFI_STATE_CHANGED_ACTION)
            addAction(WifiManager.NETWORK_STATE_CHANGED_ACTION)
        }
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                app.registerReceiver(localReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
            } else {
                @Suppress("DEPRECATION") app.registerReceiver(localReceiver, filter)
            }
            receiver = localReceiver
        } catch (_: Throwable) {
            registered.set(false)
        }
    }

    private fun registerNetworkCallback(context: Context, ingress: WorkflowDeviceEventIngress) {
        if (!networkRegistered.compareAndSet(false, true)) return
        val manager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
        if (manager == null) {
            networkRegistered.set(false)
            return
        }
        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                emitNetwork(ingress, manager, network, "device.network.available")
            }

            override fun onLost(network: Network) {
                ingress.emit(
                    "device.network.lost",
                    JSONObject().put("network", network.toString()),
                    "android.connectivity",
                    eventID("network-lost", network.toString(), System.currentTimeMillis().toString()),
                )
            }

            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                emitNetwork(ingress, manager, network, "device.network.changed", capabilities)
            }
        }
        try {
            manager.registerNetworkCallback(NetworkRequest.Builder().build(), callback)
            networkCallback = callback
        } catch (_: Throwable) {
            networkRegistered.set(false)
        }
    }

    private fun emitBattery(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val level = intent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
        val scale = intent.getIntExtra(BatteryManager.EXTRA_SCALE, -1)
        val percent = if (level >= 0 && scale > 0) (level * 100 / scale) else -1
        val status = intent.getIntExtra(BatteryManager.EXTRA_STATUS, BatteryManager.BATTERY_STATUS_UNKNOWN)
        val plugged = intent.getIntExtra(BatteryManager.EXTRA_PLUGGED, 0)
        val payload = JSONObject()
            .put("level", level)
            .put("scale", scale)
            .put("percent", percent)
            .put("status", status)
            .put("plugged", plugged)
            .put("charging", status == BatteryManager.BATTERY_STATUS_CHARGING || status == BatteryManager.BATTERY_STATUS_FULL)
        ingress.emit(
            "device.power.battery_changed",
            payload,
            "android.broadcast",
            eventID("battery", percent.toString(), status.toString(), plugged.toString(), System.currentTimeMillis().div(5000).toString()),
        )
    }

    private fun emitHeadset(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val state = intent.getIntExtra("state", 0)
        ingress.emit(
            if (state == 1) "device.audio.headset_connected" else "device.audio.headset_disconnected",
            JSONObject()
                .put("state", state)
                .put("name", intent.getStringExtra("name").orEmpty())
                .put("microphone", intent.getIntExtra("microphone", 0) == 1),
            "android.broadcast",
            eventID("headset", state.toString(), System.currentTimeMillis().toString()),
        )
    }

    private fun emitBluetoothState(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val state = intent.getIntExtra(BluetoothAdapter.EXTRA_STATE, BluetoothAdapter.ERROR)
        val previous = intent.getIntExtra(BluetoothAdapter.EXTRA_PREVIOUS_STATE, BluetoothAdapter.ERROR)
        ingress.emit(
            "device.bluetooth.state_changed",
            JSONObject().put("state", state).put("previousState", previous),
            "android.bluetooth",
            eventID("bt-state", state.toString(), previous.toString(), System.currentTimeMillis().toString()),
        )
    }

    private fun emitBluetoothConnection(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val state = intent.getIntExtra(BluetoothAdapter.EXTRA_CONNECTION_STATE, BluetoothProfile.STATE_DISCONNECTED)
        val previous = intent.getIntExtra(BluetoothAdapter.EXTRA_PREVIOUS_CONNECTION_STATE, BluetoothProfile.STATE_DISCONNECTED)
        ingress.emit(
            if (state == BluetoothProfile.STATE_CONNECTED) "device.bluetooth.connected" else "device.bluetooth.disconnected",
            JSONObject().put("state", state).put("previousState", previous),
            "android.bluetooth",
            eventID("bt-connection", state.toString(), previous.toString(), System.currentTimeMillis().toString()),
        )
    }

    private fun emitWifiState(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val state = intent.getIntExtra(WifiManager.EXTRA_WIFI_STATE, WifiManager.WIFI_STATE_UNKNOWN)
        val previous = intent.getIntExtra(WifiManager.EXTRA_PREVIOUS_WIFI_STATE, WifiManager.WIFI_STATE_UNKNOWN)
        val eventType = when (state) {
            WifiManager.WIFI_STATE_ENABLED -> "device.wifi.enabled"
            WifiManager.WIFI_STATE_DISABLED -> "device.wifi.disabled"
            else -> "device.wifi.state_changed"
        }
        ingress.emit(
            eventType,
            JSONObject().put("state", state).put("previousState", previous),
            "android.wifi",
            eventID("wifi-state", state.toString(), previous.toString(), System.currentTimeMillis().toString()),
        )
    }

    @Suppress("DEPRECATION")
    private fun emitWifiNetwork(ingress: WorkflowDeviceEventIngress, intent: Intent) {
        val info = intent.getParcelableExtra<android.net.NetworkInfo>(WifiManager.EXTRA_NETWORK_INFO)
        val connected = info?.isConnected == true
        ingress.emit(
            if (connected) "device.wifi.connected" else "device.wifi.disconnected",
            JSONObject()
                .put("connected", connected)
                .put("detailedState", info?.detailedState?.name.orEmpty()),
            "android.wifi",
            eventID("wifi-network", connected.toString(), info?.detailedState?.name.orEmpty(), System.currentTimeMillis().div(2000).toString()),
        )
    }

    private fun emitNetwork(
        ingress: WorkflowDeviceEventIngress,
        manager: ConnectivityManager,
        network: Network,
        eventType: String,
        supplied: NetworkCapabilities? = null,
    ) {
        val capabilities = supplied ?: manager.getNetworkCapabilities(network)
        val transports = mutableListOf<String>()
        if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) == true) transports += "wifi"
        if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) == true) transports += "cellular"
        if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) == true) transports += "ethernet"
        if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) == true) transports += "vpn"
        if (capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_BLUETOOTH) == true) transports += "bluetooth"
        val payload = JSONObject()
            .put("network", network.toString())
            .put("transports", org.json.JSONArray(transports))
            .put("internet", capabilities?.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) == true)
            .put("validated", capabilities?.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) == true)
            .put("metered", manager.isActiveNetworkMetered)
        ingress.emit(
            eventType,
            payload,
            "android.connectivity",
            eventID("network", eventType, network.toString(), transports.joinToString(","), System.currentTimeMillis().div(2000).toString()),
        )
    }

    private fun emitSimple(ingress: WorkflowDeviceEventIngress, eventType: String, action: String) {
        ingress.emit(
            eventType,
            JSONObject().put("action", action),
            "android.broadcast",
            eventID(eventType, System.currentTimeMillis().toString()),
        )
    }

    private fun eventID(vararg parts: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
            .digest(parts.joinToString("\u0000").toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it) }
        return "android:${digest.take(40)}"
    }
}
