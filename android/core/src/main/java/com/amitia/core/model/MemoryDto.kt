package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class MemoryDto(
    val id: String,
    val content: String,
    val type: String? = null,
    val scope: String? = null,
    val characterId: String? = null,
    val importance: Double? = null,
    val tags: List<String> = emptyList(),
    val embedding: List<Float>? = null,
    val createdAt: String? = null,
    val updatedAt: String? = null
)

@Serializable
data class MemoryCreateRequest(
    val content: String,
    val type: String? = null,
    val scope: String? = null,
    val characterId: String? = null,
    val importance: Double? = null,
    val tags: List<String> = emptyList()
)

@Serializable
data class MemoryUpdateRequest(
    val content: String? = null,
    val importance: Double? = null,
    val tags: List<String>? = null
)

@Serializable
data class MemorySearchRequest(
    val query: String,
    val limit: Int = 10,
    val threshold: Double = 0.5,
    val characterId: String? = null
)

@Serializable
data class MemoryTimelineItem(
    val id: String,
    val content: String,
    val timestamp: String,
    val type: String? = null,
    val importance: Double? = null
)

@Serializable
data class MemoryGraphDto(
    val nodes: List<MemoryGraphNode> = emptyList(),
    val edges: List<MemoryGraphEdge> = emptyList()
)

@Serializable
data class MemoryGraphNode(
    val id: String,
    val label: String,
    val type: String? = null,
    val properties: Map<String, String> = emptyMap()
)

@Serializable
data class MemoryGraphEdge(
    val source: String,
    val target: String,
    val relation: String,
    val weight: Double? = null
)
