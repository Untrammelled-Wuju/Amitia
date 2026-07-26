package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class SendStreamRequest(
    val conversationId: String? = null,
    val characterId: String? = null,
    val content: String,
    val channel: String = "web",
    val useMemory: Boolean = true,
    val useTts: Boolean = false,
    val useVision: Boolean = false,
    val attachments: List<Attachment> = emptyList()
)

@Serializable
data class Attachment(
    val type: String,
    val url: String,
    val mimeType: String? = null,
    val filename: String? = null,
    val size: Long? = null
)
