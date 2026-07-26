package com.amitia.core.network.ws

import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

data class WsMessage(
    val type: String,
    val payload: JsonObject = JsonObject(emptyMap()),
    val id: String? = null,
    val timestamp: Long? = null
) {

    fun field(key: String): JsonElement? = payload[key]
}
