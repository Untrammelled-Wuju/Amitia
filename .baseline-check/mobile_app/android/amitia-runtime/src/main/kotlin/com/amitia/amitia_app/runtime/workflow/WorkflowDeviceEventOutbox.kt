package com.amitia.amitia_app.runtime.workflow

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject

internal class WorkflowDeviceEventOutbox(context: Context) {
    private val prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun enqueue(eventType: String, envelope: JSONObject): Boolean = synchronized(lock) {
        val encoded = JSONObject()
            .put("eventType", eventType)
            .put("envelope", envelope)
            .toString()
        if (encoded.toByteArray(Charsets.UTF_8).size > MAX_EVENT_BYTES) return false
        val entries = readLocked().toMutableList()
        if (entries.any { it.optJSONObject("envelope")?.optString("eventId") == envelope.optString("eventId") }) {
            return true
        }
        entries += JSONObject(encoded)
        while (entries.size > MAX_EVENTS) entries.removeAt(0)
        writeLocked(entries)
        true
    }

    fun flush(client: WorkflowDeviceEventClient): Int = synchronized(lock) {
        val entries = readLocked().toMutableList()
        if (entries.isEmpty()) return 0
        var delivered = 0
        while (entries.isNotEmpty()) {
            val item = entries.first()
            val eventType = item.optString("eventType").trim()
            val envelope = item.optJSONObject("envelope") ?: JSONObject()
            if (eventType.isEmpty() || envelope.optString("eventId").isBlank()) {
                entries.removeAt(0)
                continue
            }
            val result = client.post(eventType, envelope)
            if (result.isFailure) break
            entries.removeAt(0)
            delivered++
        }
        writeLocked(entries)
        delivered
    }

    fun size(): Int = synchronized(lock) { readLocked().size }

    private fun readLocked(): List<JSONObject> {
        val raw = prefs.getString(KEY_EVENTS, null)?.trim().orEmpty()
        if (raw.isEmpty()) return emptyList()
        return try {
            val array = JSONArray(raw)
            buildList {
                for (i in 0 until array.length()) {
                    array.optJSONObject(i)?.let(::add)
                }
            }
        } catch (_: Throwable) {
            prefs.edit().remove(KEY_EVENTS).apply()
            emptyList()
        }
    }

    private fun writeLocked(entries: List<JSONObject>) {
        val array = JSONArray()
        entries.forEach(array::put)
        prefs.edit().putString(KEY_EVENTS, array.toString()).commit()
    }

    companion object {
        private const val PREFS = "amitia_workflow_device_event_outbox"
        private const val KEY_EVENTS = "events"
        private const val MAX_EVENTS = 64
        private const val MAX_EVENT_BYTES = 64 * 1024
        private val lock = Any()
    }
}
