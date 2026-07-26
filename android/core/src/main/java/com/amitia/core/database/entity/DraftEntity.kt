package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "drafts")
data class DraftEntity(
    @PrimaryKey val characterId: String,
    val content: String,
    val updatedAt: Long = 0L
)
