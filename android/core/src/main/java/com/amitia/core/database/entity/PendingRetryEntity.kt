package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "pending_retries")
data class PendingRetryEntity(
    @PrimaryKey val id: String,
    val type: String,
    val payload: String,
    val retryCount: Int = 0,
    val nextRetryAt: Long = 0L
)
