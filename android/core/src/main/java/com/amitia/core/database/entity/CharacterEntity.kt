package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "characters")
data class CharacterEntity(
    @PrimaryKey val id: String,
    val name: String,
    val avatar: String? = null,
    val identity: String? = null,
    val personality: String? = null,
    val status: String? = null,
    val isCurrent: Boolean = false,
    val updatedAt: Long = 0L
)
