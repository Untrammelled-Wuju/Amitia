package com.amitia.core.repository

import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ConversationListResponse
import com.amitia.core.model.MessageDto
import com.amitia.core.model.MessageListResponse
import com.amitia.core.model.SendStreamRequest
import com.amitia.core.network.api.ChatApi
import com.amitia.core.network.api.ConversationCreateRequest
import com.amitia.core.network.client.AmitiaApiClient
import com.amitia.core.network.client.AmitiaApiException
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.network.sse.SseClient
import com.amitia.core.network.sse.SseEvent
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.serialization.json.Json

@Singleton
class ChatRepository @Inject constructor(
    private val apiClient: AmitiaApiClient,
    private val sseClient: SseClient,
    private val endpointProvider: RuntimeEndpointProvider,
    private val json: Json
) {

    private val api: ChatApi by lazy { apiClient.service(ChatApi::class.java) }

    suspend fun getHistory(
        conversationId: String,
        page: Int = 1,
        pageSize: Int = 50
    ): MessageListResponse {
        return api.listMessages(conversationId, page, pageSize)
    }

    suspend fun listConversations(page: Int = 1, pageSize: Int = 20): ConversationListResponse {
        return api.listConversations(page, pageSize)
    }

    suspend fun createConversation(
        title: String? = null,
        characterId: String? = null,
        channel: String = "web"
    ): ConversationDto {
        return api.createConversation(
            ConversationCreateRequest(
                title = title,
                characterId = characterId,
                channel = channel
            )
        )
    }

    suspend fun deleteConversation(conversationId: String) {
        api.deleteConversation(conversationId)
    }

    suspend fun deleteMessage(messageId: String) {
        api.deleteMessage(messageId)
    }

    fun sendStream(request: SendStreamRequest): Flow<SseEvent> {
        val endpoint = endpointProvider.currentEndpoint.value
        val url = endpoint.baseUrl() + "/api/web-chat/send-stream"
        val body = json.encodeToString(SendStreamRequest.serializer(), request)
        val headers = buildMap {
            val token = endpoint.authHeader()
            if (!token.isNullOrBlank()) {
                put("Authorization", "Bearer $token")
            }
        }
        return sseClient.connect(url, body, headers)
    }

    fun retryMessage(messageId: String): Flow<SseEvent> {
        val endpoint = endpointProvider.currentEndpoint.value
        val url = endpoint.baseUrl() + "/api/web-chat/messages/$messageId/retry"
        val headers = buildMap {
            val token = endpoint.authHeader()
            if (!token.isNullOrBlank()) {
                put("Authorization", "Bearer $token")
            }
        }
        return sseClient.connect(url, "{}", headers)
    }

    suspend fun send(request: SendStreamRequest): MessageDto {
        return api.send(request)
    }

    fun mapError(throwable: Throwable): AmitiaApiException {
        return if (throwable is AmitiaApiException) throwable
        else AmitiaApiException.Unknown(throwable)
    }
}
