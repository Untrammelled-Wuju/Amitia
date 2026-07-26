package com.amitia.core.repository

import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.MemoryDto
import com.amitia.core.model.MemoryGraphDto
import com.amitia.core.model.MemorySearchRequest
import com.amitia.core.model.MemoryTimelineItem
import com.amitia.core.model.MemoryUpdateRequest
import com.amitia.core.network.api.MemoryApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MemoryRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: MemoryApi by lazy { apiClient.service(MemoryApi::class.java) }

    suspend fun list(
        page: Int = 1,
        pageSize: Int = 20,
        characterId: String? = null,
        type: String? = null
    ): List<MemoryDto> {
        return api.listMemories(page, pageSize, characterId, type)
    }

    suspend fun search(request: MemorySearchRequest): List<MemoryDto> {
        return api.search(request)
    }

    suspend fun getTimeline(
        start: String? = null,
        end: String? = null,
        limit: Int = 50
    ): List<MemoryTimelineItem> {
        return api.getTimeline(start, end, limit)
    }

    suspend fun getGraph(
        characterId: String? = null,
        depth: Int = 2
    ): MemoryGraphDto {
        return api.getGraph(characterId, depth)
    }

    suspend fun get(id: String): MemoryDto {
        return api.getMemory(id)
    }

    suspend fun create(request: MemoryCreateRequest): MemoryDto {
        return api.createMemory(request)
    }

    suspend fun update(id: String, request: MemoryUpdateRequest): MemoryDto {
        return api.updateMemory(id, request)
    }

    suspend fun delete(id: String) {
        api.deleteMemory(id)
    }
}
