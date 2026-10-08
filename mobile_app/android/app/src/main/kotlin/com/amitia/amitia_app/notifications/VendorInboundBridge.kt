package com.amitia.amitia_app.notifications

import android.content.Context
import org.json.JSONObject

/**
 * Single entry point used by optional manufacturer Push adapters.
 *
 * Vendor adapters only deliver a JSON data envelope and token updates here.
 * All OS presentation stays in NotificationRenderer so FCM, manufacturer
 * pass-through, and the local Native Bridge share exactly one renderer.
 */
object VendorInboundBridge {
    @JvmStatic
    fun handleJson(context: Context, provider: String, rawPayload: String?) {
        val raw = rawPayload?.trim().orEmpty()
        if (raw.isEmpty()) return

        val root = runCatching { JSONObject(raw) }.getOrNull() ?: return
        val payloadObject = unwrapPayload(root)
        val data = linkedMapOf<String, String>()
        val keys = payloadObject.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            val value = payloadObject.opt(key)
            if (value == null || value == JSONObject.NULL) continue
            data[key] = when (value) {
                is String -> value
                else -> value.toString()
            }
        }
        data["provider"] = provider.trim().lowercase()
        NotificationRenderer.handleRemoteMessage(context.applicationContext, data)
    }

    private fun unwrapPayload(root: JSONObject): JSONObject {
        if (root.has("type")) return root
        for (key in listOf("data", "payload", "content", "message")) {
            when (val nested = root.opt(key)) {
                is JSONObject -> if (nested.length() > 0) return nested
                is String -> {
                    val value = nested.trim()
                    if (value.startsWith("{")) {
                        val decoded = runCatching { JSONObject(value) }.getOrNull()
                        if (decoded != null) return decoded
                    }
                }
            }
        }
        return root
    }

    @JvmStatic
    fun updateToken(context: Context, provider: String, token: String?) {
        val normalized = token?.trim().orEmpty()
        if (normalized.isEmpty()) return
        VendorPushBootstrap.acceptToken(
            context.applicationContext,
            provider.trim().lowercase(),
            normalized,
        )
    }
}
