package com.amitia.core.repository

import com.amitia.core.model.ModelConfigDto
import com.amitia.core.model.ModelConfigUpdateRequest
import com.amitia.core.model.ModelDto
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ModelRepository @Inject constructor() {

    private val mockModels = listOf(
        ModelDto(
            id = "mdl_gpt4",
            name = "GPT-4o",
            provider = "openai",
            type = "text",
            enabled = true,
            isDefault = true,
            contextWindow = 128000,
            maxTokens = 4096,
            capabilities = listOf("text", "vision", "function_calling"),
            createdAt = "2026-07-01T08:00:00"
        ),
        ModelDto(
            id = "mdl_gpt4m",
            name = "GPT-4o-mini",
            provider = "openai",
            type = "text",
            enabled = true,
            isDefault = false,
            contextWindow = 128000,
            maxTokens = 4096,
            capabilities = listOf("text"),
            createdAt = "2026-07-01T08:00:00"
        ),
        ModelDto(
            id = "mdl_embed",
            name = "text-embedding-3-small",
            provider = "openai",
            type = "embedding",
            enabled = true,
            isDefault = true,
            capabilities = listOf("embedding"),
            createdAt = "2026-07-01T08:00:00"
        ),
        ModelDto(
            id = "mdl_tts",
            name = "tts-1",
            provider = "openai",
            type = "tts",
            enabled = true,
            isDefault = true,
            capabilities = listOf("tts"),
            createdAt = "2026-07-01T08:00:00"
        )
    )

    suspend fun list(
        type: String? = null,
        provider: String? = null
    ): List<ModelDto> {
        return if (type != null) mockModels.filter { it.type == type }
        else mockModels
    }

    suspend fun get(id: String): ModelDto {
        return mockModels.firstOrNull { it.id == id } ?: mockModels.first()
    }

    suspend fun create(model: ModelDto): ModelDto {
        return model
    }

    suspend fun update(id: String, model: ModelDto): ModelDto {
        return model.copy(id = id)
    }

    suspend fun delete(id: String) {}

    suspend fun getConfig(): ModelConfigDto {
        return ModelConfigDto(
            currentModelId = "mdl_gpt4",
            currentEmbeddingModelId = "mdl_embed",
            currentTtsModelId = "mdl_tts",
            models = mockModels
        )
    }

    suspend fun updateConfig(request: ModelConfigUpdateRequest): ModelConfigDto {
        return getConfig()
    }
}
