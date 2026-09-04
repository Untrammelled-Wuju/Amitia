package com.amitia.amitia_app.workflow

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import com.amitia.amitia_app.runtime.workflow.WorkflowDeviceEventIngress
import org.json.JSONArray
import org.json.JSONObject
import java.security.MessageDigest
import java.util.UUID
import java.util.concurrent.atomic.AtomicBoolean

class WorkflowIntentReceiver : BroadcastReceiver() {
    private data class SanitizeBudget(
        var remainingValues: Int = MAX_SANITIZED_VALUES,
        var remainingChars: Int = MAX_SANITIZED_CHARS,
    )

    override fun onReceive(context: Context, intent: Intent) {
        val action = intent.action?.trim().orEmpty()
        if (action.isEmpty()) return
        val pending = goAsync()
        val finished = AtomicBoolean(false)
        fun finishOnce() {
            if (finished.compareAndSet(false, true)) {
                runCatching { pending.finish() }
            }
        }
        Handler(Looper.getMainLooper()).postDelayed(::finishOnce, PENDING_RESULT_TIMEOUT_MS)
        try {
            val ingress = WorkflowDeviceEventIngress(context.applicationContext)
            val sanitized = sanitizeExtras(intent.extras)
            if (action == TASKER_ACTION) {
                val eventName = sanitized.optString("eventName").trim().take(128)
                val secret = sanitized.optString("secret").take(4096)
                if (eventName.isEmpty() || secret.isEmpty()) {
                    finishOnce()
                    return
                }
                val variables = taskerVariables(sanitized)
                val eventID = safeEventID(sanitized.optString("eventId")) ?: deterministicTaskerEventID(eventName, variables)
                val payload = JSONObject()
                    .put("eventName", eventName)
                    .put("secret", secret)
                    .put("variables", variables)
                ingress.emit("device.android.tasker", payload, "android.tasker", eventID) { finishOnce() }
                return
            }
            val categories = JSONArray()
            intent.categories?.toList()?.sorted()?.take(MAX_ARRAY_ITEMS)?.forEach { categories.put(it.take(MAX_STRING_LENGTH)) }
            val payload = JSONObject()
                .put("action", action.take(MAX_STRING_LENGTH))
                .put("categories", categories)
                .put("dataScheme", intent.data?.scheme?.take(MAX_STRING_LENGTH).orEmpty())
                .put("mimeType", intent.type?.take(MAX_STRING_LENGTH).orEmpty())
                .put("packageName", intent.`package`?.take(MAX_STRING_LENGTH).orEmpty())
                .put("componentName", intent.component?.flattenToShortString()?.take(MAX_STRING_LENGTH).orEmpty())
                .put("extras", sanitized)
            val eventID = "intent:${UUID.randomUUID()}"
            ingress.emit("device.android.intent", payload, "android.broadcast", eventID) { finishOnce() }
        } catch (_: Exception) {
            finishOnce()
        }
    }

    private fun sanitizeExtras(bundle: Bundle?): JSONObject {
        val result = JSONObject()
        if (bundle == null) return result
        val budget = SanitizeBudget()
        val keys = runCatching { bundle.keySet().toList().sorted() }.getOrDefault(emptyList())
        for (key in keys.take(MAX_EXTRA_KEYS)) {
            if (budget.remainingValues <= 0 || budget.remainingChars <= 0) break
            val safeKey = key.take(MAX_KEY_LENGTH)
            val value = runCatching { bundle.get(key) }.getOrNull() ?: continue
            sanitizeValue(value, budget, 0)?.let { result.put(safeKey, it) }
        }
        return result
    }

    private fun sanitizeValue(value: Any, budget: SanitizeBudget, depth: Int): Any? {
        if (depth > MAX_NESTING_DEPTH || budget.remainingValues <= 0) return null
        budget.remainingValues--
        fun boundedString(raw: String): String? {
            if (budget.remainingChars <= 0) return null
            val length = minOf(raw.length, MAX_STRING_LENGTH, budget.remainingChars)
            if (length <= 0) return null
            budget.remainingChars -= length
            return raw.take(length)
        }
        return when (value) {
            is String -> boundedString(value)
            is CharSequence -> boundedString(value.toString())
            is Boolean, is Int, is Long -> value
            is Double -> value.takeIf { it.isFinite() }
            is Float -> value.takeIf { it.isFinite() }
            is Byte -> value.toInt()
            is Short -> value.toInt()
            is Array<*> -> {
                val array = JSONArray()
                for (item in value.take(MAX_ARRAY_ITEMS)) {
                    if (budget.remainingValues <= 0) break
                    item?.let { sanitizeValue(it, budget, depth + 1) }?.let { array.put(it) }
                }
                array
            }
            is IntArray -> JSONArray(value.take(minOf(MAX_ARRAY_ITEMS, budget.remainingValues)).also { budget.remainingValues -= it.size })
            is LongArray -> JSONArray(value.take(minOf(MAX_ARRAY_ITEMS, budget.remainingValues)).also { budget.remainingValues -= it.size })
            is DoubleArray -> {
                val values = value.take(minOf(MAX_ARRAY_ITEMS, budget.remainingValues)).filter { it.isFinite() }
                budget.remainingValues -= values.size
                JSONArray(values)
            }
            is FloatArray -> {
                val values = value.take(minOf(MAX_ARRAY_ITEMS, budget.remainingValues)).filter { it.isFinite() }.map { it.toDouble() }
                budget.remainingValues -= values.size
                JSONArray(values)
            }
            is BooleanArray -> JSONArray(value.take(minOf(MAX_ARRAY_ITEMS, budget.remainingValues)).also { budget.remainingValues -= it.size })
            else -> null
        }
    }

    private fun taskerVariables(extras: JSONObject): JSONObject {
        val variables = JSONObject()
        val encoded = extras.optString("variables")
        if (encoded.isNotBlank()) {
            runCatching { JSONObject(encoded) }.getOrNull()?.let { parsed ->
                val names = parsed.keys().asSequence().toList().sorted().take(MAX_EXTRA_KEYS)
                for (name in names) {
                    sanitizeJsonValue(parsed.opt(name), 0)?.let { variables.put(name.take(MAX_KEY_LENGTH), it) }
                }
            }
        }
        val names = extras.keys().asSequence().toList().sorted().take(MAX_EXTRA_KEYS)
        for (name in names) {
            if (name == "eventName" || name == "secret" || name == "eventId" || name == "variables") continue
            sanitizeJsonValue(extras.opt(name), 0)?.let { variables.put(name.take(MAX_KEY_LENGTH), it) }
        }
        return variables
    }

    private fun sanitizeJsonValue(value: Any?, depth: Int): Any? {
        if (value == null || value == JSONObject.NULL || depth > MAX_NESTING_DEPTH) return null
        return when (value) {
            is String -> value.take(MAX_STRING_LENGTH)
            is Boolean, is Int, is Long -> value
            is Double -> value.takeIf { it.isFinite() }
            is Float -> value.takeIf { it.isFinite() }?.toDouble()
            is JSONArray -> {
                val array = JSONArray()
                for (index in 0 until minOf(value.length(), MAX_ARRAY_ITEMS)) {
                    sanitizeJsonValue(value.opt(index), depth + 1)?.let { array.put(it) }
                }
                array
            }
            else -> null
        }
    }

    private fun safeEventID(value: String): String? {
        val candidate = value.trim()
        if (candidate.isEmpty() || candidate.length > 200) return null
        if (!candidate.all { it in 'a'..'z' || it in 'A'..'Z' || it in '0'..'9' || it == '.' || it == '_' || it == ':' || it == '-' }) return null
        return candidate
    }

    private fun deterministicTaskerEventID(eventName: String, variables: JSONObject): String {
        val bucket = System.currentTimeMillis() / 2000L
        val canonicalVariables = variables.keys().asSequence().toList().sorted().joinToString("\n") { name ->
            "$name=${variables.opt(name)}"
        }
        val digest = MessageDigest.getInstance("SHA-256")
            .digest("$eventName\n$canonicalVariables\n$bucket".toByteArray(Charsets.UTF_8))
            .take(12)
            .joinToString("") { "%02x".format(it) }
        return "tasker:$bucket:$digest"
    }

    companion object {
        const val TASKER_ACTION = "com.amitia.workflow.TASKER"
        private const val MAX_EXTRA_KEYS = 32
        private const val MAX_ARRAY_ITEMS = 32
        private const val MAX_KEY_LENGTH = 128
        private const val MAX_STRING_LENGTH = 4096
        private const val MAX_NESTING_DEPTH = 4
        private const val MAX_SANITIZED_VALUES = 256
        private const val MAX_SANITIZED_CHARS = 64 * 1024
        private const val PENDING_RESULT_TIMEOUT_MS = 8_000L
    }
}
