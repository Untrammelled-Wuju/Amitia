package com.amitia.amitia_app.runtime.workflow

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone
import java.util.ArrayDeque
import java.util.UUID
import java.util.concurrent.ArrayBlockingQueue
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.ThreadPoolExecutor
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class WorkflowDeviceEventIngress(
    context: Context,
) {
    private val appContext = context.applicationContext
    private val client = WorkflowDeviceEventClient(appContext)
    private val outbox = WorkflowDeviceEventOutbox(appContext)
    private val journal = WorkflowDeviceEventJournal(appContext)

    fun reportCapabilities(
        items: List<Map<String, Any?>>,
        completion: (Result<Unit>) -> Unit = {},
    ) {
        executeReport(completion) {
            val array = JSONArray()
            items.forEach { item ->
                val value = JSONObject()
                item.forEach { (key, raw) -> value.put(key, raw) }
                array.put(value)
            }
            client.postCapabilityStatus(JSONObject().put("items", array))
        }
    }

    fun flushPending(completion: (Result<Int>) -> Unit = {}) {
        try {
            reportExecutor.execute {
                completion(runCatching { outbox.flush(client) })
            }
        } catch (_: RejectedExecutionException) {
            completion(Result.failure(IllegalStateException("workflow device report queue is full")))
        }
    }

    fun reportAndroidRuntimeHealth(
        status: Map<String, Any?>,
        completion: (Result<Unit>) -> Unit = {},
    ) {
        executeReport(completion) {
            val body = JSONObject()
            status.forEach { (key, raw) -> body.put(key, raw) }
            client.postAndroidRuntimeHealth(body)
        }
    }

    fun reportAppCatalog(
        items: List<Map<String, String>>,
        completion: (Result<Unit>) -> Unit = {},
    ) {
        executeReport(completion) {
            val array = JSONArray()
            items.forEach { item ->
                array.put(
                    JSONObject()
                        .put("packageName", item["packageName"].orEmpty())
                        .put("label", item["label"].orEmpty()),
                )
            }
            client.postAppCatalog(JSONObject().put("items", array))
        }
    }

    fun emit(
        eventType: String,
        payload: JSONObject,
        source: String,
        eventID: String = UUID.randomUUID().toString(),
        dedupeWindowMs: Long = defaultDedupeWindowMs(eventType),
        completion: (Result<Unit>) -> Unit = {},
    ) {
        val normalizedEventType = eventType.trim()
        if (normalizedEventType !in allowedEventTypes) {
            completion(Result.failure(IllegalArgumentException("unsupported workflow device event type")))
            return
        }
        val normalizedEventID = eventID.trim()
        if (normalizedEventID.isEmpty() || normalizedEventID.length > 200 || !normalizedEventID.all(::isEventIDCharacter)) {
            completion(Result.failure(IllegalArgumentException("invalid workflow device event id")))
            return
        }
        if (!allow(normalizedEventType)) {
            completion(Result.failure(IllegalStateException("workflow device event rate limit exceeded")))
            return
        }
        try {
            eventExecutor.execute {
                val prepared = journal.prepare(
                    eventType = normalizedEventType,
                    source = source,
                    payload = payload,
                    dedupeWindowMs = dedupeWindowMs.coerceIn(0L, MAX_DEDUPE_WINDOW_MS),
                )
                if (prepared.duplicate) {
                    completion(Result.success(Unit))
                    return@execute
                }
                val envelope = JSONObject()
                    .put("eventId", normalizedEventID)
                    .put("eventFingerprint", prepared.fingerprint)
                    .put("sourceSequence", prepared.sourceSequence)
                    .put("source", source.take(128))
                    .put("occurredAt", timestamp())
                    .put("payload", payload)
                outbox.flush(client)
                val delivered = client.post(normalizedEventType, envelope)
                if (delivered.isSuccess) {
                    journal.recordAccepted(prepared.fingerprint, normalizedEventID)
                    completion(delivered)
                } else if (outbox.enqueue(normalizedEventType, envelope)) {
                    journal.recordAccepted(prepared.fingerprint, normalizedEventID)
                    scheduleRetry()
                    completion(Result.success(Unit))
                } else {
                    completion(delivered)
                }
            }
        } catch (_: RejectedExecutionException) {
            completion(Result.failure(IllegalStateException("workflow device event queue is full")))
        }
    }

    private fun executeReport(
        completion: (Result<Unit>) -> Unit,
        block: () -> Result<Unit>,
    ) {
        try {
            reportExecutor.execute { completion(block()) }
        } catch (_: RejectedExecutionException) {
            completion(Result.failure(IllegalStateException("workflow device report queue is full")))
        }
    }

    private fun scheduleRetry() {
        if (!retryScheduled.compareAndSet(false, true)) return
        retryExecutor.schedule({
            try {
                outbox.flush(client)
            } finally {
                retryScheduled.set(false)
                if (outbox.size() > 0) scheduleRetry()
            }
        }, retryDelaySeconds, TimeUnit.SECONDS)
    }

    companion object {
        private val eventExecutor = ThreadPoolExecutor(
            2,
            2,
            0L,
            TimeUnit.MILLISECONDS,
            ArrayBlockingQueue(32),
            { runnable -> Thread(runnable, "amitia-workflow-device-events").apply { isDaemon = true } },
            ThreadPoolExecutor.AbortPolicy(),
        )
        private val reportExecutor = ThreadPoolExecutor(
            1,
            1,
            0L,
            TimeUnit.MILLISECONDS,
            ArrayBlockingQueue(4),
            { runnable -> Thread(runnable, "amitia-workflow-device-reports").apply { isDaemon = true } },
            ThreadPoolExecutor.AbortPolicy(),
        )
        private val retryExecutor = ScheduledThreadPoolExecutor(
            1,
            { runnable -> Thread(runnable, "amitia-workflow-device-event-retry").apply { isDaemon = true } },
        ).apply { removeOnCancelPolicy = true }
        private val retryScheduled = AtomicBoolean(false)
        private val rateWindows = ConcurrentHashMap<String, ArrayDeque<Long>>()
        private const val retryDelaySeconds = 3L
        private val allowedEventTypes = setOf(
            "device.android.intent",
            "device.android.tasker",
            "voice.wake.detected",
            "voice.asr.final",
            "device.app.foreground",
            "device.notification.posted",
            "device.notification.removed",
            "device.power.battery_changed",
            "device.power.battery_low",
            "device.power.battery_okay",
            "device.power.connected",
            "device.power.disconnected",
            "device.screen.on",
            "device.screen.off",
            "device.user.present",
            "device.audio.headset_connected",
            "device.audio.headset_disconnected",
            "device.bluetooth.state_changed",
            "device.bluetooth.connected",
            "device.bluetooth.disconnected",
            "device.ble.characteristic_changed",
            "device.network.available",
            "device.network.lost",
            "device.network.changed",
            "device.wifi.enabled",
            "device.wifi.disabled",
            "device.wifi.state_changed",
            "device.wifi.connected",
            "device.wifi.disconnected",
            "device.location.geofence.enter",
            "device.location.geofence.exit",
            "device.system.boot_completed",
            "device.app.installed",
            "device.app.removed",
            "device.app.updated",
            "device.app.self_updated",
            "device.time.changed",
            "device.time.timezone_changed",
            "device.time.date_changed",
        )
        private const val maxEventsPerMinute = 120
        private const val MAX_DEDUPE_WINDOW_MS = 10L * 60L * 1000L

        private fun defaultDedupeWindowMs(eventType: String): Long = when (eventType.trim()) {
            "device.system.boot_completed",
            "device.app.self_updated",
            "device.app.installed",
            "device.app.removed",
            "device.app.updated" -> 10_000L
            "device.notification.posted",
            "device.notification.removed",
            "voice.wake.detected",
            "voice.asr.final" -> 500L
            else -> 2_000L
        }

        private fun isEventIDCharacter(value: Char): Boolean =
            value in 'a'..'z' || value in 'A'..'Z' || value in '0'..'9' ||
                value == '.' || value == '_' || value == ':' || value == '-'

        private fun timestamp(): String {
            return SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'", Locale.US).apply {
                timeZone = TimeZone.getTimeZone("UTC")
            }.format(Date())
        }

        private fun allow(eventType: String): Boolean {
            val now = System.currentTimeMillis()
            val cutoff = now - 60_000L
            val queue = rateWindows.computeIfAbsent(eventType) { ArrayDeque() }
            synchronized(queue) {
                while (queue.isNotEmpty() && queue.first() < cutoff) {
                    queue.removeFirst()
                }
                if (queue.size >= maxEventsPerMinute) {
                    return false
                }
                queue.addLast(now)
                return true
            }
        }
    }
}
