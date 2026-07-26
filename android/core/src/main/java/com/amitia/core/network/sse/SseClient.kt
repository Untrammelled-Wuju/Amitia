package com.amitia.core.network.sse

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okio.BufferedSource

@Singleton
class SseClient @Inject constructor(
    private val okHttpClient: OkHttpClient
) {

    fun connect(
        url: String,
        body: String,
        headers: Map<String, String> = emptyMap()
    ): Flow<SseEvent> = callbackFlow {
        val mediaType = "application/json; charset=utf-8".toMediaType()
        val requestBody = body.toRequestBody(mediaType)

        val requestBuilder = Request.Builder()
            .url(url)
            .post(requestBody)
            .header(HEADER_ACCEPT, ACCEPT_SSE)
            .header(HEADER_CACHE_CONTROL, CACHE_NO_CACHE)

        headers.forEach { (key, value) ->
            requestBuilder.header(key, value)
        }

        val request = requestBuilder.build()
        val call = okHttpClient.newCall(request)

        var response: Response? = null
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: java.io.IOException) {
                trySend(
                    SseEvent(
                        event = SseEvent.EVENT_ERROR,
                        data = e.message ?: "connection_failure"
                    )
                )
                close()
            }

            override fun onResponse(call: Call, resp: Response) {
                response = resp
                if (!resp.isSuccessful) {
                    trySend(
                        SseEvent(
                            event = SseEvent.EVENT_ERROR,
                            data = "http_${resp.code}"
                        )
                    )
                    close()
                    return
                }
                try {
                    val source = resp.body?.source() ?: run {
                        trySend(
                            SseEvent(
                                event = SseEvent.EVENT_ERROR,
                                data = "empty_body"
                            )
                        )
                        close()
                        return
                    }
                    streamSource(source) { event ->
                        trySend(event)
                        if (event.isTerminal()) {
                            close()
                        }
                    }
                } catch (e: Exception) {
                    trySend(
                        SseEvent(
                            event = SseEvent.EVENT_ERROR,
                            data = e.message ?: "stream_error"
                        )
                    )
                    close()
                }
            }
        })

        awaitClose {
            runCatching { call.cancel() }
            response?.close()
        }
    }.flowOn(Dispatchers.IO)

    private fun streamSource(source: BufferedSource, onEvent: (SseEvent) -> Unit) {
        val buffer = StringBuilder()
        while (!source.exhausted()) {
            val line = source.readUtf8Line() ?: break
            buffer.append(line)
            buffer.append('\n')
            if (line.isEmpty()) {
                val block = buffer.toString().trimEnd('\n')
                if (block.isNotBlank()) {
                    val event = SseParser.parseBlock(block)
                    if (event != null) {
                        onEvent(event)
                    }
                }
                buffer.setLength(0)
            }
        }
        val remaining = buffer.toString().trimEnd('\n')
        if (remaining.isNotBlank()) {
            val event = SseParser.parseBlock(remaining)
            if (event != null) {
                onEvent(event)
            }
        }
    }

    fun connectGet(
        url: String,
        headers: Map<String, String> = emptyMap()
    ): Flow<SseEvent> = callbackFlow {
        val requestBuilder = Request.Builder()
            .url(url)
            .header(HEADER_ACCEPT, ACCEPT_SSE)
            .header(HEADER_CACHE_CONTROL, CACHE_NO_CACHE)

        headers.forEach { (key, value) ->
            requestBuilder.header(key, value)
        }

        val request = requestBuilder.build()
        val call = okHttpClient.newCall(request)

        var response: Response? = null
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: java.io.IOException) {
                trySend(
                    SseEvent(
                        event = SseEvent.EVENT_ERROR,
                        data = e.message ?: "connection_failure"
                    )
                )
                close()
            }

            override fun onResponse(call: Call, resp: Response) {
                response = resp
                if (!resp.isSuccessful) {
                    trySend(
                        SseEvent(
                            event = SseEvent.EVENT_ERROR,
                            data = "http_${resp.code}"
                        )
                    )
                    close()
                    return
                }
                try {
                    val source = resp.body?.source() ?: run {
                        trySend(
                            SseEvent(
                                event = SseEvent.EVENT_ERROR,
                                data = "empty_body"
                            )
                        )
                        close()
                        return
                    }
                    streamSource(source) { event ->
                        trySend(event)
                        if (event.isTerminal()) {
                            close()
                        }
                    }
                } catch (e: Exception) {
                    trySend(
                        SseEvent(
                            event = SseEvent.EVENT_ERROR,
                            data = e.message ?: "stream_error"
                        )
                    )
                    close()
                }
            }
        })

        awaitClose {
            runCatching { call.cancel() }
            response?.close()
        }
    }.flowOn(Dispatchers.IO)

    companion object {
        private const val HEADER_ACCEPT = "Accept"
        private const val ACCEPT_SSE = "text/event-stream"
        private const val HEADER_CACHE_CONTROL = "Cache-Control"
        private const val CACHE_NO_CACHE = "no-cache"
    }
}
