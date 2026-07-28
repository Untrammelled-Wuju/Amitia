package com.amitia.core.repository

import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.MemoryDto
import com.amitia.core.model.MemoryGraphDto
import com.amitia.core.model.MemorySearchRequest
import com.amitia.core.model.MemoryTimelineItem
import com.amitia.core.model.MemoryUpdateRequest
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MemoryRepository @Inject constructor() {

    private val mockMemories = listOf(
        MemoryDto(
            id = "mem_001",
            content = "用户希望被称呼为小明",
            type = "preference",
            characterId = "mock_char_001",
            importance = 0.9,
            createdAt = "2026-07-01T08:00:00"
        ),
        MemoryDto(
            id = "mem_002",
            content = "用户在上海市浦东新区工作",
            type = "fact",
            characterId = "mock_char_001",
            importance = 0.85,
            createdAt = "2026-07-05T10:00:00"
        ),
        MemoryDto(
            id = "mem_003",
            content = "用户偏好简洁直接的回复风格",
            type = "preference",
            characterId = "mock_char_001",
            importance = 0.8,
            createdAt = "2026-07-10T14:00:00"
        )
    )

    suspend fun list(
        page: Int = 1,
        pageSize: Int = 20,
        characterId: String? = null,
        type: String? = null
    ): List<MemoryDto> {
        return mockMemories
    }

    suspend fun search(request: MemorySearchRequest): List<MemoryDto> {
        return mockMemories.filter { it.content.contains(request.query, ignoreCase = true) }
    }

    suspend fun getTimeline(
        start: String? = null,
        end: String? = null,
        limit: Int = 50
    ): List<MemoryTimelineItem> {
        return mockMemories.map {
            MemoryTimelineItem(
                id = it.id,
                content = it.content,
                timestamp = it.createdAt ?: "",
                type = it.type,
                importance = it.importance
            )
        }
    }

    suspend fun getGraph(
        characterId: String? = null,
        depth: Int = 2
    ): MemoryGraphDto {
        return MemoryGraphDto(nodes = emptyList(), edges = emptyList())
    }

    suspend fun get(id: String): MemoryDto {
        return mockMemories.firstOrNull { it.id == id } ?: mockMemories.first()
    }

    suspend fun create(request: MemoryCreateRequest): MemoryDto {
        return MemoryDto(
            id = UUID.randomUUID().toString(),
            content = request.content,
            type = request.type ?: "general",
            characterId = request.characterId,
            importance = request.importance,
            createdAt = java.text.SimpleDateFormat(
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                java.util.Locale.getDefault()
            ).format(java.util.Date())
        )
    }

    suspend fun update(id: String, request: MemoryUpdateRequest): MemoryDto {
        return get(id)
    }

    suspend fun delete(id: String) {}
}
