package com.amitia.core.repository

import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ConversationListResponse
import com.amitia.core.model.MessageDto
import com.amitia.core.model.MessageListResponse
import com.amitia.core.model.SendStreamRequest
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.emptyFlow

@Singleton
class ChatRepository @Inject constructor() {

    private val mockConversation = ConversationDto(
        id = "mock_conv_001",
        title = "默认对话",
        characterId = "mock_char_001",
        channel = "web",
        lastMessageAt = "2026-07-28T10:00:00",
        unreadCount = 0,
        createdAt = "2026-07-28T10:00:00"
    )

    private val mockMessages = listOf(
        MessageDto(
            id = "mock_msg_001",
            conversationId = "mock_conv_001",
            role = "assistant",
            channel = "web",
            content = "你好！我是AI助手，有什么可以帮你的？",
            contentType = "text",
            status = "completed",
            createdAt = "2026-07-28T10:00:00"
        ),
        MessageDto(
            id = "mock_msg_002",
            conversationId = "mock_conv_001",
            role = "user",
            channel = "web",
            content = "你好，请介绍一下你自己",
            contentType = "text",
            status = "sent",
            createdAt = "2026-07-28T10:00:30"
        ),
        MessageDto(
            id = "mock_msg_003",
            conversationId = "mock_conv_001",
            role = "assistant",
            channel = "web",
            content = "我是一个智能对话助手，可以帮你解答问题、处理任务、提供建议。",
            contentType = "text",
            status = "completed",
            createdAt = "2026-07-28T10:00:35"
        )
    )

    suspend fun getHistory(
        conversationId: String,
        page: Int = 1,
        pageSize: Int = 50
    ): MessageListResponse {
        return MessageListResponse(
            items = if (page == 1) mockMessages else emptyList(),
            total = mockMessages.size,
            page = page,
            pageSize = pageSize
        )
    }

    suspend fun listConversations(page: Int = 1, pageSize: Int = 20): ConversationListResponse {
        return ConversationListResponse(
            items = listOf(mockConversation),
            total = 1,
            page = page,
            pageSize = pageSize
        )
    }

    suspend fun createConversation(
        title: String? = null,
        characterId: String? = null,
        channel: String = "web"
    ): ConversationDto {
        return mockConversation.copy(
            title = title ?: mockConversation.title,
            characterId = characterId ?: mockConversation.characterId,
            channel = channel
        )
    }

    suspend fun deleteConversation(conversationId: String) {}

    suspend fun deleteMessage(messageId: String) {}

    fun sendStream(request: SendStreamRequest): Flow<com.amitia.core.network.sse.SseEvent> {
        return emptyFlow()
    }

    fun retryMessage(messageId: String): Flow<com.amitia.core.network.sse.SseEvent> {
        return emptyFlow()
    }

    suspend fun send(request: SendStreamRequest): MessageDto {
        return MessageDto(
            id = UUID.randomUUID().toString(),
            role = "assistant",
            content = "这是模拟回复",
            createdAt = java.text.SimpleDateFormat(
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                java.util.Locale.getDefault()
            ).format(java.util.Date())
        )
    }

    fun mapError(throwable: Throwable): com.amitia.core.network.client.AmitiaApiException {
        return if (throwable is com.amitia.core.network.client.AmitiaApiException) throwable
        else com.amitia.core.network.client.AmitiaApiException.Unknown(throwable)
    }
}
