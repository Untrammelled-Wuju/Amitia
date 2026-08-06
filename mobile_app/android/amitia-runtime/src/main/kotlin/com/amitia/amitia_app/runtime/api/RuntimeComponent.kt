package com.amitia.amitia_app.runtime.api

enum class RuntimeComponentState {
    UNKNOWN,
    NOT_INSTALLED,
    INSTALLED,
    STARTING,
    READY,
    DEGRADED,
    STOPPING,
    STOPPED,
    FAILED,
    DISABLED
}

data class RuntimeComponentSnapshot(
    val id: String,
    val state: RuntimeComponentState,
    val required: Boolean,
    val version: String?,
    val errorCode: String?,
    val updatedAtEpochMillis: Long
)
