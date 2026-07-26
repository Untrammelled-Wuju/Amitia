package com.amitia.core.network.ws

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener

@Singleton
class WsClient @Inject constructor(
    private val okHttpClient: OkHttpClient,
    private val json: Json
) {

    fun connect(
        url: String,
        headers: Map<String, String> = emptyMap()
    ): Flow<WsMessage> = callbackFlow {
        val requestBuilder = Request.Builder().url(url)
        headers.forEach { (key, value) ->
            requestBuilder.header(key, value)
        }

        val request = requestBuilder.build()
        var webSocket: WebSocket? = null

        webSocket = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                trySend(
                    WsMessage(
                        type = EVENT_OPEN,
                        payload = buildJsonObject {
                            put("code", response.code)
                        }
                    )
                )
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                val message = parseMessage(text)
                trySend(message)
            }

            override fun onMessage(webSocket: WebSocket, bytes: okio.ByteString) {
                val text = bytes.utf8()
                val message = parseMessage(text)
                trySend(message)
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                trySend(
                    WsMessage(
                        type = EVENT_CLOSING,
                        payload = buildJsonObject {
                            put("code", code)
                            put("reason", reason)
                        }
                    )
                )
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                trySend(
                    WsMessage(
                        type = EVENT_CLOSED,
                        payload = buildJsonObject {
                            put("code", code)
                            put("reason", reason)
                        }
                    )
                )
                close()
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                trySend(
                    WsMessage(
                        type = EVENT_ERROR,
                        payload = buildJsonObject {
                            put("message", t.message ?: "ws_failure")
                            put("code", response?.code ?: -1)
                        }
                    )
                )
                close(t)
            }
        })

        awaitClose {
            runCatching { webSocket?.close(NORMAL_CLOSURE_CODE, "client_closed") }
        }
    }.flowOn(Dispatchers.IO)

    private fun parseMessage(text: String): WsMessage {
        return runCatching {
            val element = json.parseToJsonElement(text)
            if (element is JsonObject) {
                val type = element[FIELD_TYPE]?.toString()?.trim('"') ?: EVENT_MESSAGE
                val payload = element[FIELD_PAYLOAD] as? JsonObject ?: element
                val id = element[FIELD_ID]?.toString()?.trim('"')
                val timestamp = element[FIELD_TIMESTAMP]?.toString()?.toLongOrNull()
                WsMessage(type = type, payload = payload, id = id, timestamp = timestamp)
            } else {
                WsMessage(type = EVENT_MESSAGE, payload = buildJsonObject { put("raw", text) })
            }
        }.getOrElse {
            WsMessage(
                type = EVENT_MESSAGE,
                payload = buildJsonObject { put("raw", text) }
            )
        }
    }

    companion object {
        private const val FIELD_TYPE = "type"
        private const val FIELD_PAYLOAD = "payload"
        private const val FIELD_ID = "id"
        private const val FIELD_TIMESTAMP = "timestamp"

        const val EVENT_OPEN = "open"
        const val EVENT_MESSAGE = "message"
        const val EVENT_CLOSING = "closing"
        const val EVENT_CLOSED = "closed"
        const val EVENT_ERROR = "error"

        const val NORMAL_CLOSURE_CODE = 1000
    }
}
