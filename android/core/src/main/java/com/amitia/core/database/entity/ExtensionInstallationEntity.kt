package com.amitia.core.database.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "extension_installations")
data class ExtensionInstallationEntity(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val extensionId: String,
    val version: String,
    val manifestHash: String,
    val installedAt: Long,
    val status: String,
    val contributionIds: List<String> = emptyList()
)
