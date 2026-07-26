package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "conversations")
data class ConversationEntity(
    @PrimaryKey val id: String,
    val characterId: String?,
    val lastMessagePreview: String? = null,
    val lastMessageAt: Long = 0L,
    val unreadCount: Int = 0
)
