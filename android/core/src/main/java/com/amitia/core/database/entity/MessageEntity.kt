package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "messages")
data class MessageEntity(
    @PrimaryKey val id: String,
    val conversationId: String,
    val role: String,
    val content: String,
    val imagesJson: String? = null,
    val audioUrl: String? = null,
    val createdAt: Long = 0L,
    val status: String = "sent",
    val remoteId: String? = null
)
