package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class MessageDto(
    val id: String,
    val conversationId: String? = null,
    val role: String,
    val channel: String? = null,
    val content: String,
    val contentType: String = "text",
    val status: String? = null,
    val audioUrl: String? = null,
    val duration: Double? = null,
    val imageUrl: String? = null,
    val videoUrl: String? = null,
    val createdAt: String? = null,
    val updatedAt: String? = null
)

@Serializable
data class ConversationDto(
    val id: String,
    val title: String? = null,
    val characterId: String? = null,
    val channel: String? = null,
    val lastMessageAt: String? = null,
    val unreadCount: Int = 0,
    val createdAt: String? = null
)

@Serializable
data class ConversationListResponse(
    val items: List<ConversationDto> = emptyList(),
    val total: Int = 0,
    val page: Int = 1,
    val pageSize: Int = 20
)

@Serializable
data class MessageListResponse(
    val items: List<MessageDto> = emptyList(),
    val total: Int = 0,
    val page: Int = 1,
    val pageSize: Int = 20
)
