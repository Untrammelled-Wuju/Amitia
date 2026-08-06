package com.amitia.amitia_app.runtime.api

data class RuntimeSnapshot(
    val state: RuntimeState,
    val runtimeVersion: String?,
    val activeOperationId: String?,
    val activeOperationType: RuntimeOperationType?,
    val progress: RuntimeProgress,
    val components: List<RuntimeComponentSnapshot>,
    val lastError: RuntimeError?,
    val generation: Long,
    val updatedAtEpochMillis: Long
) {
    init {
        require(generation >= 0) { "generation must not be negative" }
        val hasOperationId = activeOperationId != null
        val hasOperationType = activeOperationType != null
        require(hasOperationId == hasOperationType) {
            "activeOperationId and activeOperationType must both be null or both be non-null"
        }
    }

    companion object {
        fun initial(): RuntimeSnapshot = RuntimeSnapshot(
            state = RuntimeState.UNKNOWN,
            runtimeVersion = null,
            activeOperationId = null,
            activeOperationType = null,
            progress = RuntimeProgress.none(),
            components = emptyList(),
            lastError = null,
            generation = 0,
            updatedAtEpochMillis = 0
        )
    }
}
