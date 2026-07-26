package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "runtime_state")
data class RuntimeStateEntity(
    @PrimaryKey val id: Int = 1,
    val state: String,
    val services: String? = null,
    val updatedAt: Long = 0L,
    val snapshot: String? = null
)
