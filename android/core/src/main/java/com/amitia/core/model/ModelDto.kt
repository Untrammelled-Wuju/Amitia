package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class ModelDto(
    val id: String,
    val name: String,
    val provider: String? = null,
    val type: String? = null,
    val endpoint: String? = null,
    val apiKey: String? = null,
    val enabled: Boolean = false,
    val isDefault: Boolean = false,
    val contextWindow: Int? = null,
    val maxTokens: Int? = null,
    val temperature: Double? = null,
    val capabilities: List<String> = emptyList(),
    val createdAt: String? = null,
    val updatedAt: String? = null
)

@Serializable
data class ModelConfigDto(
    val currentModelId: String? = null,
    val currentEmbeddingModelId: String? = null,
    val currentTtsModelId: String? = null,
    val currentAsrModelId: String? = null,
    val currentVisionModelId: String? = null,
    val currentImageGenModelId: String? = null,
    val models: List<ModelDto> = emptyList()
)

@Serializable
data class ModelConfigUpdateRequest(
    val currentModelId: String? = null,
    val currentEmbeddingModelId: String? = null,
    val currentTtsModelId: String? = null,
    val currentAsrModelId: String? = null,
    val currentVisionModelId: String? = null,
    val currentImageGenModelId: String? = null
)
