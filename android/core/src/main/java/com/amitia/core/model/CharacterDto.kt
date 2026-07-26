package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class CharacterDto(
    val id: String,
    val name: String,
    val avatar: String? = null,
    val description: String? = null,
    val personality: String? = null,
    val systemPrompt: String? = null,
    val greeting: String? = null,
    val scenario: String? = null,
    val tags: List<String> = emptyList(),
    val isCurrent: Boolean = false,
    val createdAt: String? = null,
    val updatedAt: String? = null
)

@Serializable
data class CharacterCreateRequest(
    val name: String,
    val avatar: String? = null,
    val description: String? = null,
    val personality: String? = null,
    val systemPrompt: String? = null,
    val greeting: String? = null,
    val scenario: String? = null,
    val tags: List<String> = emptyList()
)

@Serializable
data class CharacterUpdateRequest(
    val name: String? = null,
    val avatar: String? = null,
    val description: String? = null,
    val personality: String? = null,
    val systemPrompt: String? = null,
    val greeting: String? = null,
    val scenario: String? = null,
    val tags: List<String>? = null
)

@Serializable
data class CharacterSwitchRequest(
    val characterId: String
)
