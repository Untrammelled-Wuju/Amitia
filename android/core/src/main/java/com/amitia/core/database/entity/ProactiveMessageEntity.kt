package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "proactive_messages")
data class ProactiveMessageEntity(
    @PrimaryKey val id: String,
    val characterId: String?,
    val content: String,
    val createdAt: Long = 0L,
    val isRead: Boolean = false,
    val isNotified: Boolean = false
)
