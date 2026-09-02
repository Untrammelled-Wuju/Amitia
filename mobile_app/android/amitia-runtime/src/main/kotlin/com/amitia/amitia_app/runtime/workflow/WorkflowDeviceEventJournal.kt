package com.amitia.amitia_app.runtime.workflow

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.security.MessageDigest

/**
 * Small persistent ingress journal used before WorkflowDeviceEventOutbox.
 *
 * Android broadcasts/callbacks are at-least-once in practice. Event IDs created
 * by framework callbacks are not always stable across process recreation, so
 * dedupe is based on a content fingerprint and a bounded time window. Accepted
 * events still go through the persistent Outbox; this journal only suppresses
 * duplicate ingress and keeps a monotonic source sequence for diagnostics.
 */
internal class WorkflowDeviceEventJournal(context: Context) {
    private val prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    data class PreparedEvent(
        val fingerprint: String,
        val sourceSequence: Long,
        val duplicate: Boolean,
    )

    fun prepare(eventType: String, source: String, payload: JSONObject, dedupeWindowMs: Long): PreparedEvent = synchronized(lock) {
        val now = System.currentTimeMillis()
        val fingerprint = fingerprint(eventType, source, payload)
        val entries = readLocked().toMutableList()
        val cutoff = now - RETENTION_MS
        entries.removeAll { it.optLong("acceptedAt", 0L) < cutoff }
        val duplicate = dedupeWindowMs > 0L && entries.any {
            it.optString("fingerprint") == fingerprint && now - it.optLong("acceptedAt", 0L) <= dedupeWindowMs
        }
        val nextSequence = prefs.getLong(KEY_SEQUENCE, 0L).let { current ->
            if (current == Long.MAX_VALUE) 1L else current + 1L
        }
        prefs.edit().putLong(KEY_SEQUENCE, nextSequence).commit()
        PreparedEvent(fingerprint = fingerprint, sourceSequence = nextSequence, duplicate = duplicate)
    }

    fun recordAccepted(fingerprint: String, eventID: String) = synchronized(lock) {
        val now = System.currentTimeMillis()
        val entries = readLocked().toMutableList()
        entries.removeAll { it.optString("fingerprint") == fingerprint || now - it.optLong("acceptedAt", 0L) > RETENTION_MS }
        entries += JSONObject()
            .put("fingerprint", fingerprint)
            .put("eventId", eventID)
            .put("acceptedAt", now)
        while (entries.size > MAX_ENTRIES) entries.removeAt(0)
        writeLocked(entries)
    }

    private fun fingerprint(eventType: String, source: String, payload: JSONObject): String {
        val canonical = eventType.trim() + "\u0000" + source.trim() + "\u0000" + canonicalJSON(payload)
        val digest = MessageDigest.getInstance("SHA-256").digest(canonical.toByteArray(Charsets.UTF_8))
        return digest.joinToString("") { "%02x".format(it) }
    }

    private fun canonicalJSON(value: Any?): String = when (value) {
        null, JSONObject.NULL -> "null"
        is JSONObject -> value.keys().asSequence().toList().sorted().joinToString(prefix = "{", postfix = "}", separator = ",") { key ->
            JSONObject.quote(key) + ":" + canonicalJSON(value.opt(key))
        }
        is JSONArray -> (0 until value.length()).joinToString(prefix = "[", postfix = "]", separator = ",") { index ->
            canonicalJSON(value.opt(index))
        }
        is String -> JSONObject.quote(value)
        is Number, is Boolean -> value.toString()
        else -> JSONObject.quote(value.toString())
    }

    private fun readLocked(): List<JSONObject> {
        val raw = prefs.getString(KEY_ENTRIES, null)?.trim().orEmpty()
        if (raw.isEmpty()) return emptyList()
        return try {
            val array = JSONArray(raw)
            buildList {
                for (index in 0 until array.length()) array.optJSONObject(index)?.let(::add)
            }
        } catch (_: Throwable) {
            prefs.edit().remove(KEY_ENTRIES).apply()
            emptyList()
        }
    }

    private fun writeLocked(entries: List<JSONObject>) {
        val array = JSONArray()
        entries.forEach(array::put)
        prefs.edit().putString(KEY_ENTRIES, array.toString()).commit()
    }

    companion object {
        private const val PREFS = "amitia_workflow_device_event_journal"
        private const val KEY_ENTRIES = "entries"
        private const val KEY_SEQUENCE = "source_sequence"
        private const val MAX_ENTRIES = 256
        private const val RETENTION_MS = 24L * 60L * 60L * 1000L
        private val lock = Any()
    }
}
