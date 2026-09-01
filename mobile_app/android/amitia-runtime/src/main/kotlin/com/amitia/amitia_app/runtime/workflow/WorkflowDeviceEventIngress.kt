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
import java.util.concurrent.ThreadPoolExecutor
import java.util.concurrent.TimeUnit

class WorkflowDeviceEventIngress(
    context: Context,
) {
    private val client = WorkflowDeviceEventClient(context.applicationContext)

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
                val envelope = JSONObject()
                    .put("eventId", normalizedEventID)
                    .put("source", source.take(128))
                    .put("occurredAt", timestamp())
                    .put("payload", payload)
                completion(client.post(normalizedEventType, envelope))
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
        private val rateWindows = ConcurrentHashMap<String, ArrayDeque<Long>>()
        private val allowedEventTypes = setOf(
            "device.android.intent",
            "device.android.tasker",
            "voice.wake.detected",
            "voice.asr.final",
            "device.app.foreground",
        )
        private const val maxEventsPerMinute = 120

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
