package com.amitia.runtime.api

interface RuntimeFacade {

    val state: kotlinx.coroutines.flow.StateFlow<RuntimeState>

    val events: kotlinx.coroutines.flow.Flow<RuntimeEvent>

    val services: kotlinx.coroutines.flow.StateFlow<RuntimeServices>

    val uptimeMs: kotlinx.coroutines.flow.StateFlow<Long>

    suspend fun start(): Result<RuntimeServices>

    suspend fun stop(): Result<Unit>

    suspend fun restart(): Result<RuntimeServices>

    suspend fun repair(): Result<Unit>

    suspend fun refresh()

    suspend fun update(): Result<Unit>

    suspend fun cleanup(confirm: Boolean): Result<Unit>

    fun snapshot(): RuntimeSnapshot

    data class RuntimeSnapshot(
        val state: RuntimeState,
        val services: RuntimeServices,
        val uptimeMs: Long,
        val rootfsVersion: String?,
        val manifestVersion: String?,
        val crashCounts: Map<String, Int>,
        val lastErrors: Map<String, String?>
    )
}
