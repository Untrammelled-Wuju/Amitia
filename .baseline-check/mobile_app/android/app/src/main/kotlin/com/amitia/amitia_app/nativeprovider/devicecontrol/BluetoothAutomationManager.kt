package com.amitia.amitia_app.nativeprovider.devicecontrol

import android.Manifest
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothProfile
import android.bluetooth.BluetoothServerSocket
import android.bluetooth.BluetoothSocket
import android.bluetooth.BluetoothStatusCodes
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.util.Base64
import androidx.core.content.ContextCompat
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONObject
import java.io.IOException
import java.util.ArrayDeque
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

internal class BluetoothAutomationManager(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val classicSessions = ConcurrentHashMap<String, BluetoothSocket>()
    private val classicServers = ConcurrentHashMap<String, BluetoothServerSocket>()
    private val bleSessions = ConcurrentHashMap<String, BleSession>()
    private val ioExecutor = Executors.newCachedThreadPool { runnable ->
        Thread(runnable, "amitia-bluetooth-io").apply { isDaemon = true }
    }
    private val ingress by lazy { WorkflowDeviceEventIngress(appContext) }

    data class Result(
        val value: Map<String, Any?>? = null,
        val code: String? = null,
        val message: String? = null,
    ) {
        val successful: Boolean get() = code == null
    }

    fun classicConnect(adapter: BluetoothAdapter, payload: Map<String, Any?>): Result {
        if (!hasConnectPermission()) return fail("DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        val address = payload.string("address")
        if (!BluetoothAdapter.checkBluetoothAddress(address)) return fail("DEVICE_INVALID_REQUEST", "valid Bluetooth address is required")
        val uuid = parseUUID(payload.string("uuid").ifBlank { SPP_UUID })
            ?: return fail("DEVICE_INVALID_REQUEST", "uuid is invalid")
        val secure = payload.boolean("secure", true)
        val timeoutMs = payload.long("timeoutMs", 12_000L).coerceIn(1000L, 30_000L)
        val device = try {
            adapter.getRemoteDevice(address)
        } catch (t: Throwable) {
            return fail("DEVICE_BLUETOOTH_DEVICE_INVALID", t.message ?: "Bluetooth device is invalid")
        }
        val socket = try {
            if (secure) device.createRfcommSocketToServiceRecord(uuid) else device.createInsecureRfcommSocketToServiceRecord(uuid)
        } catch (t: Throwable) {
            return fail("DEVICE_BLUETOOTH_CONNECT_FAILED", t.message ?: "failed to create RFCOMM socket")
        }
        runCatching { adapter.cancelDiscovery() }
        val future = ioExecutor.submit<Unit> { socket.connect() }
        return try {
            future.get(timeoutMs, TimeUnit.MILLISECONDS)
            val sessionId = "classic:${UUID.randomUUID()}"
            classicSessions[sessionId] = socket
            ok(
                mapOf(
                    "sessionId" to sessionId,
                    "address" to address,
                    "uuid" to uuid.toString(),
                    "secure" to secure,
                    "connected" to socket.isConnected,
                ),
            )
        } catch (t: Throwable) {
            future.cancel(true)
            runCatching { socket.close() }
            fail("DEVICE_BLUETOOTH_CONNECT_FAILED", rootMessage(t, "Bluetooth Classic connection failed"))
        }
    }

    fun classicDisconnect(payload: Map<String, Any?>): Result {
        val sessionId = payload.string("sessionId")
        val socket = classicSessions.remove(sessionId)
            ?: return fail("DEVICE_BLUETOOTH_SESSION_NOT_FOUND", "Bluetooth Classic session was not found")
        return try {
            socket.close()
            ok(mapOf("sessionId" to sessionId, "disconnected" to true))
        } catch (t: Throwable) {
            fail("DEVICE_BLUETOOTH_DISCONNECT_FAILED", t.message ?: "Bluetooth Classic disconnect failed")
        }
    }

    fun classicWrite(payload: Map<String, Any?>): Result {
        val sessionId = payload.string("sessionId")
        val socket = classicSessions[sessionId]
            ?: return fail("DEVICE_BLUETOOTH_SESSION_NOT_FOUND", "Bluetooth Classic session was not found")
        val bytes = decodeValue(payload) ?: return fail("DEVICE_INVALID_REQUEST", "valueBase64 or valueText is required")
        if (bytes.size > MAX_CLASSIC_WRITE_BYTES) return fail("DEVICE_INVALID_REQUEST", "Bluetooth Classic write exceeds $MAX_CLASSIC_WRITE_BYTES bytes")
        return try {
            socket.outputStream.write(bytes)
            socket.outputStream.flush()
            ok(mapOf("sessionId" to sessionId, "bytesWritten" to bytes.size))
        } catch (t: Throwable) {
            classicSessions.remove(sessionId)
            runCatching { socket.close() }
            fail("DEVICE_BLUETOOTH_WRITE_FAILED", t.message ?: "Bluetooth Classic write failed")
        } finally {
            bytes.fill(0)
        }
    }

    fun classicRead(payload: Map<String, Any?>): Result {
        val sessionId = payload.string("sessionId")
        val socket = classicSessions[sessionId]
            ?: return fail("DEVICE_BLUETOOTH_SESSION_NOT_FOUND", "Bluetooth Classic session was not found")
        val maxBytes = payload.int("maxBytes", 4096).coerceIn(1, MAX_CLASSIC_READ_BYTES)
        val timeoutMs = payload.long("timeoutMs", 5000L).coerceIn(100L, 30_000L)
        val input = try {
            socket.inputStream
        } catch (t: Throwable) {
            return fail("DEVICE_BLUETOOTH_READ_FAILED", t.message ?: "Bluetooth Classic input stream unavailable")
        }
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMs)
        try {
            while (System.nanoTime() < deadline) {
                val available = runCatching { input.available() }.getOrElse {
                    return fail("DEVICE_BLUETOOTH_READ_FAILED", it.message ?: "Bluetooth Classic read failed")
                }
                if (available > 0) {
                    val buffer = ByteArray(minOf(maxBytes, available))
                    val count = input.read(buffer)
                    if (count < 0) {
                        classicSessions.remove(sessionId)
                        runCatching { socket.close() }
                        return fail("DEVICE_BLUETOOTH_DISCONNECTED", "Bluetooth Classic peer closed the connection")
                    }
                    val value = buffer.copyOf(count)
                    buffer.fill(0)
                    return try {
                        ok(
                            mapOf(
                                "sessionId" to sessionId,
                                "bytesRead" to count,
                                "valueBase64" to Base64.encodeToString(value, Base64.NO_WRAP),
                                "valueText" to if (payload.boolean("decodeUtf8", false)) value.toString(Charsets.UTF_8) else null,
                            ),
                        )
                    } finally {
                        value.fill(0)
                    }
                }
                Thread.sleep(25L)
            }
            return fail("DEVICE_BLUETOOTH_READ_TIMEOUT", "Bluetooth Classic read timed out")
        } catch (t: Throwable) {
            return fail("DEVICE_BLUETOOTH_READ_FAILED", t.message ?: "Bluetooth Classic read failed")
        }
    }

    fun classicListen(adapter: BluetoothAdapter, payload: Map<String, Any?>): Result {
        if (!hasConnectPermission()) return fail("DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        val uuid = parseUUID(payload.string("uuid").ifBlank { SPP_UUID })
            ?: return fail("DEVICE_INVALID_REQUEST", "uuid is invalid")
        val serviceName = payload.string("serviceName").ifBlank { "Amitia" }.take(80)
        val secure = payload.boolean("secure", true)
        return try {
            val server = if (secure) adapter.listenUsingRfcommWithServiceRecord(serviceName, uuid) else adapter.listenUsingInsecureRfcommWithServiceRecord(serviceName, uuid)
            val serverId = "classic-server:${UUID.randomUUID()}"
            classicServers[serverId] = server
            ok(mapOf("serverId" to serverId, "uuid" to uuid.toString(), "serviceName" to serviceName, "secure" to secure))
        } catch (t: Throwable) {
            fail("DEVICE_BLUETOOTH_LISTEN_FAILED", t.message ?: "Bluetooth Classic listen failed")
        }
    }

    fun classicAccept(payload: Map<String, Any?>): Result {
        val serverId = payload.string("serverId")
        val server = classicServers[serverId]
            ?: return fail("DEVICE_BLUETOOTH_SERVER_NOT_FOUND", "Bluetooth Classic server was not found")
        val timeoutMs = payload.long("timeoutMs", 15_000L).coerceIn(1000L, 30_000L)
        return try {
            val socket = server.accept(timeoutMs.toInt()) ?: return fail("DEVICE_BLUETOOTH_ACCEPT_TIMEOUT", "Bluetooth Classic accept timed out")
            val sessionId = "classic:${UUID.randomUUID()}"
            classicSessions[sessionId] = socket
            ok(
                mapOf(
                    "sessionId" to sessionId,
                    "serverId" to serverId,
                    "connected" to socket.isConnected,
                    "address" to runCatching { socket.remoteDevice.address }.getOrNull(),
                    "name" to runCatching { socket.remoteDevice.name }.getOrNull(),
                ),
            )
        } catch (t: IOException) {
            fail("DEVICE_BLUETOOTH_ACCEPT_FAILED", t.message ?: "Bluetooth Classic accept failed")
        }
    }

    fun classicCloseServer(payload: Map<String, Any?>): Result {
        val serverId = payload.string("serverId")
        val server = classicServers.remove(serverId)
            ?: return fail("DEVICE_BLUETOOTH_SERVER_NOT_FOUND", "Bluetooth Classic server was not found")
        return try {
            server.close()
            ok(mapOf("serverId" to serverId, "closed" to true))
        } catch (t: Throwable) {
            fail("DEVICE_BLUETOOTH_SERVER_CLOSE_FAILED", t.message ?: "Bluetooth Classic server close failed")
        }
    }

    fun bleConnect(adapter: BluetoothAdapter, payload: Map<String, Any?>): Result {
        if (!hasConnectPermission()) return fail("DEVICE_BLUETOOTH_PERMISSION_REQUIRED", "Bluetooth connect permission is required")
        val address = payload.string("address")
        if (!BluetoothAdapter.checkBluetoothAddress(address)) return fail("DEVICE_INVALID_REQUEST", "valid BLE address is required")
        val timeoutMs = payload.long("timeoutMs", 12_000L).coerceIn(1000L, 30_000L)
        val existing = bleSessions.entries.firstOrNull { it.value.address.equals(address, true) && it.value.connected }
        if (existing != null) return ok(existing.value.summary(existing.key))

        val device = try {
            adapter.getRemoteDevice(address)
        } catch (t: Throwable) {
            return fail("DEVICE_BLE_DEVICE_INVALID", t.message ?: "BLE device is invalid")
        }
        val sessionId = "ble:${UUID.randomUUID()}"
        val session = BleSession(sessionId, address)
        bleSessions[sessionId] = session
        return try {
            val gatt = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                device.connectGatt(appContext, false, session, BluetoothDevice.TRANSPORT_LE)
            } else {
                @Suppress("DEPRECATION")
                device.connectGatt(appContext, false, session)
            }
            session.attach(gatt)
            if (!session.awaitConnected(timeoutMs)) {
                bleSessions.remove(sessionId)
                session.close()
                return fail("DEVICE_BLE_CONNECT_TIMEOUT", "BLE connection timed out")
            }
            if (!session.connected) {
                val status = session.connectionStatus.get()
                bleSessions.remove(sessionId)
                session.close()
                return fail("DEVICE_BLE_CONNECT_FAILED", "BLE connection failed with status $status")
            }
            ok(session.summary(sessionId))
        } catch (t: Throwable) {
            bleSessions.remove(sessionId)
            session.close()
            fail("DEVICE_BLE_CONNECT_FAILED", t.message ?: "BLE connection failed")
        }
    }

    fun bleDisconnect(payload: Map<String, Any?>): Result {
        val sessionId = payload.string("sessionId")
        val session = bleSessions.remove(sessionId)
            ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        session.close()
        return ok(mapOf("sessionId" to sessionId, "disconnected" to true))
    }

    fun bleServices(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val timeoutMs = payload.long("timeoutMs", 8000L).coerceIn(1000L, 20_000L)
        if (!session.discoverServices(timeoutMs)) return fail("DEVICE_BLE_DISCOVERY_FAILED", "BLE service discovery failed or timed out")
        val services = session.gatt()?.services.orEmpty().map { service ->
            mapOf(
                "uuid" to service.uuid.toString(),
                "type" to service.type,
                "characteristics" to service.characteristics.map { characteristicMap(it) },
            )
        }
        return ok(mapOf("sessionId" to session.sessionId, "services" to services, "count" to services.size))
    }

    fun bleCharacteristics(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val serviceUUID = parseUUID(payload.string("serviceUuid")) ?: return fail("DEVICE_INVALID_REQUEST", "serviceUuid is required")
        val service = session.gatt()?.getService(serviceUUID) ?: return fail("DEVICE_BLE_SERVICE_NOT_FOUND", "BLE service was not found")
        val characteristics = service.characteristics.map(::characteristicMap)
        return ok(mapOf("sessionId" to session.sessionId, "serviceUuid" to serviceUUID.toString(), "characteristics" to characteristics, "count" to characteristics.size))
    }

    fun bleRead(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val characteristic = findCharacteristic(session, payload) ?: return fail("DEVICE_BLE_CHARACTERISTIC_NOT_FOUND", "BLE characteristic was not found")
        val timeoutMs = payload.long("timeoutMs", 8000L).coerceIn(1000L, 20_000L)
        val value = session.readCharacteristic(characteristic, timeoutMs)
            ?: return fail("DEVICE_BLE_READ_FAILED", "BLE characteristic read failed or timed out")
        return try {
            ok(valueResult(session.sessionId, characteristic, value))
        } finally {
            value.fill(0)
        }
    }

    fun bleWrite(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val characteristic = findCharacteristic(session, payload) ?: return fail("DEVICE_BLE_CHARACTERISTIC_NOT_FOUND", "BLE characteristic was not found")
        val value = decodeValue(payload) ?: return fail("DEVICE_INVALID_REQUEST", "valueBase64 or valueText is required")
        if (value.size > MAX_BLE_WRITE_BYTES) return fail("DEVICE_INVALID_REQUEST", "BLE write exceeds $MAX_BLE_WRITE_BYTES bytes")
        val withoutResponse = payload.boolean("withoutResponse", false)
        val timeoutMs = payload.long("timeoutMs", 8000L).coerceIn(1000L, 20_000L)
        return try {
            if (!session.writeCharacteristic(characteristic, value, withoutResponse, timeoutMs)) {
                fail("DEVICE_BLE_WRITE_FAILED", "BLE characteristic write failed or timed out")
            } else {
                ok(mapOf("sessionId" to session.sessionId, "serviceUuid" to characteristic.service.uuid.toString(), "characteristicUuid" to characteristic.uuid.toString(), "bytesWritten" to value.size, "withoutResponse" to withoutResponse))
            }
        } finally {
            value.fill(0)
        }
    }

    fun bleSubscribe(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val characteristic = findCharacteristic(session, payload) ?: return fail("DEVICE_BLE_CHARACTERISTIC_NOT_FOUND", "BLE characteristic was not found")
        val timeoutMs = payload.long("timeoutMs", 8000L).coerceIn(1000L, 20_000L)
        val indications = payload.boolean("indications", false)
        if (!session.setSubscription(characteristic, enabled = true, indications = indications, timeoutMs = timeoutMs)) {
            return fail("DEVICE_BLE_SUBSCRIBE_FAILED", "BLE subscription failed or timed out")
        }
        return ok(mapOf("sessionId" to session.sessionId, "serviceUuid" to characteristic.service.uuid.toString(), "characteristicUuid" to characteristic.uuid.toString(), "subscribed" to true, "indications" to indications))
    }

    fun bleUnsubscribe(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val characteristic = findCharacteristic(session, payload) ?: return fail("DEVICE_BLE_CHARACTERISTIC_NOT_FOUND", "BLE characteristic was not found")
        val timeoutMs = payload.long("timeoutMs", 8000L).coerceIn(1000L, 20_000L)
        if (!session.setSubscription(characteristic, enabled = false, indications = false, timeoutMs = timeoutMs)) {
            return fail("DEVICE_BLE_UNSUBSCRIBE_FAILED", "BLE unsubscribe failed or timed out")
        }
        return ok(mapOf("sessionId" to session.sessionId, "serviceUuid" to characteristic.service.uuid.toString(), "characteristicUuid" to characteristic.uuid.toString(), "subscribed" to false))
    }

    fun bleReadNotifications(payload: Map<String, Any?>): Result {
        val session = session(payload) ?: return fail("DEVICE_BLE_SESSION_NOT_FOUND", "BLE session was not found")
        val serviceFilterRaw = payload.string("serviceUuid")
        val characteristicFilterRaw = payload.string("characteristicUuid")
        val serviceFilter = if (serviceFilterRaw.isBlank()) null else parseUUID(serviceFilterRaw)
            ?: return fail("DEVICE_INVALID_REQUEST", "serviceUuid must be a valid UUID")
        val characteristicFilter = if (characteristicFilterRaw.isBlank()) null else parseUUID(characteristicFilterRaw)
            ?: return fail("DEVICE_INVALID_REQUEST", "characteristicUuid must be a valid UUID")
        val maxItems = payload.int("maxItems", 50).coerceIn(1, 100)
        val clearAfterRead = payload.boolean("clearAfterRead", true)
        val snapshot = session.readNotifications(serviceFilter, characteristicFilter, maxItems, clearAfterRead)
        return ok(mapOf(
            "sessionId" to session.sessionId,
            "notifications" to snapshot.items,
            "count" to snapshot.items.size,
            "remaining" to snapshot.remaining,
            "clearAfterRead" to clearAfterRead,
            "dropped" to snapshot.dropped,
        ))
    }

    private fun session(payload: Map<String, Any?>): BleSession? = bleSessions[payload.string("sessionId")]

    private fun findCharacteristic(session: BleSession, payload: Map<String, Any?>): BluetoothGattCharacteristic? {
        val serviceUUID = parseUUID(payload.string("serviceUuid")) ?: return null
        val characteristicUUID = parseUUID(payload.string("characteristicUuid")) ?: return null
        return session.gatt()?.getService(serviceUUID)?.getCharacteristic(characteristicUUID)
    }

    private fun characteristicMap(characteristic: BluetoothGattCharacteristic): Map<String, Any?> {
        val properties = mutableListOf<String>()
        if (characteristic.properties and BluetoothGattCharacteristic.PROPERTY_READ != 0) properties += "read"
        if (characteristic.properties and BluetoothGattCharacteristic.PROPERTY_WRITE != 0) properties += "write"
        if (characteristic.properties and BluetoothGattCharacteristic.PROPERTY_WRITE_NO_RESPONSE != 0) properties += "write_without_response"
        if (characteristic.properties and BluetoothGattCharacteristic.PROPERTY_NOTIFY != 0) properties += "notify"
        if (characteristic.properties and BluetoothGattCharacteristic.PROPERTY_INDICATE != 0) properties += "indicate"
        return mapOf(
            "uuid" to characteristic.uuid.toString(),
            "serviceUuid" to characteristic.service.uuid.toString(),
            "properties" to properties,
            "descriptors" to characteristic.descriptors.map { it.uuid.toString() },
        )
    }

    private fun valueResult(sessionId: String, characteristic: BluetoothGattCharacteristic, value: ByteArray): Map<String, Any?> = mapOf(
        "sessionId" to sessionId,
        "serviceUuid" to characteristic.service.uuid.toString(),
        "characteristicUuid" to characteristic.uuid.toString(),
        "length" to value.size,
        "valueBase64" to Base64.encodeToString(value, Base64.NO_WRAP),
    )

    private inner class BleSession(
        val sessionId: String,
        val address: String,
    ) : BluetoothGattCallback() {
        private val attachedGatt = AtomicReference<BluetoothGatt?>(null)
        private val connectLatch = CountDownLatch(1)
        private val opLock = Any()
        private val notificationLock = Any()
        private val notificationBuffer = ArrayDeque<BleNotification>()
        private var notificationSequence = 0L
        private var droppedNotifications = 0L
        private var operationLatch: CountDownLatch? = null
        private var operationStatus: AtomicInteger? = null
        private var operationValue: AtomicReference<ByteArray?>? = null
        @Volatile var connected: Boolean = false
        val connectionStatus = AtomicInteger(BluetoothGatt.GATT_FAILURE)

        fun attach(gatt: BluetoothGatt?) {
            attachedGatt.set(gatt)
        }

        fun gatt(): BluetoothGatt? = attachedGatt.get()

        fun awaitConnected(timeoutMs: Long): Boolean = connectLatch.await(timeoutMs, TimeUnit.MILLISECONDS)

        fun summary(id: String): Map<String, Any?> = mapOf(
            "sessionId" to id,
            "address" to address,
            "connected" to connected,
            "status" to connectionStatus.get(),
            "servicesDiscovered" to (gatt()?.services?.isNotEmpty() == true),
        )

        fun discoverServices(timeoutMs: Long): Boolean = synchronized(opLock) {
            val gatt = gatt() ?: return false
            val latch = beginOperation()
            val started = try { gatt.discoverServices() } catch (_: Throwable) { false }
            if (!started) { clearOperation(); return false }
            val signalled = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            val success = signalled && operationStatus?.get() == BluetoothGatt.GATT_SUCCESS
            clearOperation()
            success
        }

        fun readCharacteristic(characteristic: BluetoothGattCharacteristic, timeoutMs: Long): ByteArray? = synchronized(opLock) {
            val gatt = gatt() ?: return null
            val latch = beginOperation(captureValue = true)
            val started = try { gatt.readCharacteristic(characteristic) } catch (_: Throwable) { false }
            if (!started) { clearOperation(); return null }
            val signalled = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            val success = signalled && operationStatus?.get() == BluetoothGatt.GATT_SUCCESS
            val value = if (success) operationValue?.get()?.copyOf() else null
            clearOperation()
            value
        }

        @Suppress("DEPRECATION")
        fun writeCharacteristic(characteristic: BluetoothGattCharacteristic, value: ByteArray, withoutResponse: Boolean, timeoutMs: Long): Boolean = synchronized(opLock) {
            val gatt = gatt() ?: return false
            val latch = beginOperation()
            val writeType = if (withoutResponse) BluetoothGattCharacteristic.WRITE_TYPE_NO_RESPONSE else BluetoothGattCharacteristic.WRITE_TYPE_DEFAULT
            val started = try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    gatt.writeCharacteristic(characteristic, value.copyOf(), writeType) == BluetoothStatusCodes.SUCCESS
                } else {
                    characteristic.writeType = writeType
                    characteristic.value = value.copyOf()
                    gatt.writeCharacteristic(characteristic)
                }
            } catch (_: Throwable) {
                false
            }
            if (!started) { clearOperation(); return false }
            if (withoutResponse) {
                clearOperation()
                return true
            }
            val signalled = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            val success = signalled && operationStatus?.get() == BluetoothGatt.GATT_SUCCESS
            clearOperation()
            success
        }

        @Suppress("DEPRECATION")
        fun setSubscription(characteristic: BluetoothGattCharacteristic, enabled: Boolean, indications: Boolean, timeoutMs: Long): Boolean = synchronized(opLock) {
            val gatt = gatt() ?: return false
            val local = try { gatt.setCharacteristicNotification(characteristic, enabled) } catch (_: Throwable) { false }
            if (!local) return false
            val descriptor = characteristic.getDescriptor(CLIENT_CHARACTERISTIC_CONFIG_UUID) ?: return true
            val value = when {
                !enabled -> BluetoothGattDescriptor.DISABLE_NOTIFICATION_VALUE
                indications -> BluetoothGattDescriptor.ENABLE_INDICATION_VALUE
                else -> BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE
            }
            val latch = beginOperation()
            val started = try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    gatt.writeDescriptor(descriptor, value) == BluetoothStatusCodes.SUCCESS
                } else {
                    descriptor.value = value
                    gatt.writeDescriptor(descriptor)
                }
            } catch (_: Throwable) {
                false
            }
            if (!started) { clearOperation(); return false }
            val signalled = latch.await(timeoutMs, TimeUnit.MILLISECONDS)
            val success = signalled && operationStatus?.get() == BluetoothGatt.GATT_SUCCESS
            clearOperation()
            success
        }

        fun readNotifications(serviceUuid: UUID?, characteristicUuid: UUID?, maxItems: Int, clearAfterRead: Boolean): NotificationSnapshot = synchronized(notificationLock) {
            val matching = notificationBuffer.filter { item ->
                (serviceUuid == null || item.serviceUuid.equals(serviceUuid.toString(), true)) &&
                    (characteristicUuid == null || item.characteristicUuid.equals(characteristicUuid.toString(), true))
            }.take(maxItems)
            if (clearAfterRead && matching.isNotEmpty()) {
                val consumed = matching.mapTo(hashSetOf()) { it.sequence }
                val retained = notificationBuffer.filterNot { consumed.contains(it.sequence) }
                notificationBuffer.clear()
                retained.forEach(notificationBuffer::addLast)
            }
            NotificationSnapshot(
                items = matching.map(BleNotification::asMap),
                remaining = notificationBuffer.count { item ->
                    (serviceUuid == null || item.serviceUuid.equals(serviceUuid.toString(), true)) &&
                        (characteristicUuid == null || item.characteristicUuid.equals(characteristicUuid.toString(), true))
                },
                dropped = droppedNotifications,
            )
        }

        private fun bufferNotification(characteristic: BluetoothGattCharacteristic, encoded: String, length: Int) = synchronized(notificationLock) {
            notificationSequence += 1
            notificationBuffer.addLast(BleNotification(
                sequence = notificationSequence,
                receivedAtMs = System.currentTimeMillis(),
                serviceUuid = characteristic.service.uuid.toString(),
                characteristicUuid = characteristic.uuid.toString(),
                length = length,
                valueBase64 = encoded,
            ))
            while (notificationBuffer.size > MAX_BLE_NOTIFICATION_BUFFER) {
                notificationBuffer.removeFirst()
                droppedNotifications += 1
            }
        }

        private fun beginOperation(captureValue: Boolean = false): CountDownLatch {
            val latch = CountDownLatch(1)
            operationLatch = latch
            operationStatus = AtomicInteger(Int.MIN_VALUE)
            operationValue = if (captureValue) AtomicReference(null) else null
            return latch
        }

        private fun completeOperation(status: Int, value: ByteArray? = null) {
            operationStatus?.set(status)
            if (value != null) operationValue?.set(value.copyOf())
            operationLatch?.countDown()
        }

        private fun clearOperation() {
            operationValue?.getAndSet(null)?.fill(0)
            operationLatch = null
            operationStatus = null
            operationValue = null
        }

        override fun onConnectionStateChange(gatt: BluetoothGatt, status: Int, newState: Int) {
            attachedGatt.compareAndSet(null, gatt)
            connectionStatus.set(status)
            connected = status == BluetoothGatt.GATT_SUCCESS && newState == BluetoothProfile.STATE_CONNECTED
            connectLatch.countDown()
            if (newState == BluetoothProfile.STATE_DISCONNECTED) {
                connected = false
                completeOperation(status)
            }
        }

        override fun onServicesDiscovered(gatt: BluetoothGatt, status: Int) {
            completeOperation(status)
        }

        @Suppress("DEPRECATION")
        override fun onCharacteristicRead(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic, status: Int) {
            completeOperation(status, characteristic.value ?: ByteArray(0))
        }

        override fun onCharacteristicRead(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic, value: ByteArray, status: Int) {
            completeOperation(status, value)
        }

        override fun onCharacteristicWrite(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic, status: Int) {
            completeOperation(status)
        }

        override fun onDescriptorWrite(gatt: BluetoothGatt, descriptor: BluetoothGattDescriptor, status: Int) {
            completeOperation(status)
        }

        @Suppress("DEPRECATION")
        override fun onCharacteristicChanged(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic) {
            emitCharacteristicChanged(characteristic, characteristic.value ?: ByteArray(0))
        }

        override fun onCharacteristicChanged(gatt: BluetoothGatt, characteristic: BluetoothGattCharacteristic, value: ByteArray) {
            emitCharacteristicChanged(characteristic, value)
        }

        private fun emitCharacteristicChanged(characteristic: BluetoothGattCharacteristic, value: ByteArray) {
            val encoded = Base64.encodeToString(value, Base64.NO_WRAP)
            bufferNotification(characteristic, encoded, value.size)
            val payload = JSONObject()
                .put("sessionId", sessionId)
                .put("address", address)
                .put("serviceUuid", characteristic.service.uuid.toString())
                .put("characteristicUuid", characteristic.uuid.toString())
                .put("length", value.size)
                .put("valueBase64", encoded)
            ingress.emit(
                eventType = "device.ble.characteristic_changed",
                payload = payload,
                source = "android.bluetooth.ble",
                eventID = "ble:${sessionId.substringAfter(':')}:${characteristic.uuid}:${System.nanoTime()}",
            )
        }

        fun close() {
            synchronized(notificationLock) { notificationBuffer.clear() }
            connected = false
            completeOperation(BluetoothGatt.GATT_FAILURE)
            val gatt = attachedGatt.getAndSet(null)
            runCatching { gatt?.disconnect() }
            runCatching { gatt?.close() }
        }
    }

    private data class BleNotification(
        val sequence: Long,
        val receivedAtMs: Long,
        val serviceUuid: String,
        val characteristicUuid: String,
        val length: Int,
        val valueBase64: String,
    ) {
        fun asMap(): Map<String, Any?> = mapOf(
            "sequence" to sequence,
            "receivedAtMs" to receivedAtMs,
            "serviceUuid" to serviceUuid,
            "characteristicUuid" to characteristicUuid,
            "length" to length,
            "valueBase64" to valueBase64,
        )
    }

    private data class NotificationSnapshot(
        val items: List<Map<String, Any?>>,
        val remaining: Int,
        val dropped: Long,
    )

    private fun hasConnectPermission(): Boolean = Build.VERSION.SDK_INT < Build.VERSION_CODES.S ||
        ContextCompat.checkSelfPermission(appContext, Manifest.permission.BLUETOOTH_CONNECT) == PackageManager.PERMISSION_GRANTED

    private fun parseUUID(raw: String): UUID? = runCatching { UUID.fromString(raw.trim()) }.getOrNull()

    private fun decodeValue(payload: Map<String, Any?>): ByteArray? {
        val base64 = payload.string("valueBase64")
        if (base64.isNotBlank()) return runCatching { Base64.decode(base64, Base64.DEFAULT) }.getOrNull()
        val text = payload["valueText"]?.toString() ?: return null
        return text.toByteArray(Charsets.UTF_8)
    }

    private fun ok(value: Map<String, Any?>) = Result(value = value)
    private fun fail(code: String, message: String) = Result(code = code, message = message)

    private fun rootMessage(error: Throwable, fallback: String): String {
        var current: Throwable? = error
        repeat(6) {
            val message = current?.message?.trim().orEmpty()
            if (message.isNotBlank()) return message
            current = current?.cause
        }
        return fallback
    }

    private fun Map<String, Any?>.string(key: String): String = this[key]?.toString()?.trim().orEmpty()
    private fun Map<String, Any?>.boolean(key: String, default: Boolean): Boolean = when (val value = this[key]) {
        is Boolean -> value
        is String -> value.toBooleanStrictOrNull() ?: default
        else -> default
    }
    private fun Map<String, Any?>.long(key: String, default: Long): Long = (this[key] as? Number)?.toLong() ?: this[key]?.toString()?.toLongOrNull() ?: default
    private fun Map<String, Any?>.int(key: String, default: Int): Int = (this[key] as? Number)?.toInt() ?: this[key]?.toString()?.toIntOrNull() ?: default

    companion object {
        private const val SPP_UUID = "00001101-0000-1000-8000-00805F9B34FB"
        private val CLIENT_CHARACTERISTIC_CONFIG_UUID: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
        private const val MAX_CLASSIC_WRITE_BYTES = 64 * 1024
        private const val MAX_CLASSIC_READ_BYTES = 64 * 1024
        private const val MAX_BLE_WRITE_BYTES = 512
        private const val MAX_BLE_NOTIFICATION_BUFFER = 256
    }
}
