package com.amitia.core.repository

import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.CharacterSwitchRequest
import com.amitia.core.model.CharacterUpdateRequest
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CharacterRepository @Inject constructor() {

    private val mockCharacters = listOf(
        CharacterDto(
            id = "mock_char_001",
            name = "艾米",
            avatar = null,
            description = "温柔知性的陪伴者",
            personality = "温柔体贴、善解人意",
            greeting = "你好！我是艾米，很高兴认识你～",
            isCurrent = true,
            createdAt = "2026-07-01T08:00:00"
        ),
        CharacterDto(
            id = "mock_char_002",
            name = "星野",
            avatar = null,
            description = "活泼开朗的助手",
            personality = "活泼开朗、幽默风趣",
            greeting = "嗨！我是星野，今天有什么有趣的事吗？",
            isCurrent = false,
            createdAt = "2026-07-15T10:00:00"
        ),
        CharacterDto(
            id = "mock_char_003",
            name = "云溪",
            avatar = null,
            description = "沉稳冷静的顾问",
            personality = "理性沉稳、思维缜密",
            greeting = "你好，我是云溪。有什么问题需要分析吗？",
            isCurrent = false,
            createdAt = "2026-07-20T14:00:00"
        )
    )

    suspend fun list(page: Int = 1, pageSize: Int = 20): List<CharacterDto> {
        return mockCharacters
    }

    suspend fun get(id: String): CharacterDto {
        return mockCharacters.firstOrNull { it.id == id } ?: mockCharacters.first()
    }

    suspend fun create(request: CharacterCreateRequest): CharacterDto {
        return CharacterDto(
            id = UUID.randomUUID().toString(),
            name = request.name,
            avatar = request.avatar,
            description = request.description,
            personality = request.personality,
            greeting = request.greeting,
            isCurrent = false,
            createdAt = java.text.SimpleDateFormat(
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                java.util.Locale.getDefault()
            ).format(java.util.Date())
        )
    }

    suspend fun update(id: String, request: CharacterUpdateRequest): CharacterDto {
        val existing = get(id)
        return existing.copy(
            name = request.name ?: existing.name,
            avatar = request.avatar ?: existing.avatar,
            description = request.description ?: existing.description
        )
    }

    suspend fun delete(id: String) {}

    suspend fun switchCurrent(characterId: String): CharacterDto {
        return get(characterId)
    }

    suspend fun getCurrent(): CharacterDto {
        return mockCharacters.first { it.isCurrent }
    }
}
