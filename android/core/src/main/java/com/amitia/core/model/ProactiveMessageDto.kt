package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class ProactiveMessageDto(
    val id: String,
    val conversationId: String? = null,
    val characterId: String? = null,
    val channel: String? = null,
    val content: String,
    val status: String? = null,
    val scheduledAt: String? = null,
    val sentAt: String? = null,
    val isRead: Boolean = false,
    val createdAt: String? = null
)

@Serializable
data class ProactiveListResponse(
    val items: List<ProactiveMessageDto> = emptyList(),
    val total: Int = 0,
    val page: Int = 1,
    val pageSize: Int = 20
)

@Serializable
data class ProactiveMarkReadRequest(
    val messageIds: List<String>
)
