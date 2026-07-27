package com.amitia.feature.memory

import com.amitia.core.model.MemoryDto

data class MemoryTimelineGroup(
    val title: String,
    val items: List<MemoryTimelineEntry>
)

data class MemoryTimelineEntry(
    val id: String,
    val content: String,
    val timestamp: String,
    val source: String,
    val character: String,
    val importance: Int
)

data class MemorySearchFilters(
    val keyword: String = "",
    val semanticEnabled: Boolean = true,
    val semanticAvailable: Boolean = true,
    val characterId: String? = null,
    val minImportance: Int = 0,
    val type: String? = null,
    val source: String? = null
)

data class LongTermMemoryGroup(
    val title: String,
    val items: List<MemoryDto>
)

data class EpisodicMemoryItem(
    val id: String,
    val title: String,
    val cause: String,
    val process: String,
    val result: String,
    val participants: List<String>,
    val timestamp: String
)

data class WorldBookGroup(
    val id: String,
    val name: String,
    val enabled: Boolean,
    val characterName: String,
    val entryCount: Int,
    val lastUpdated: String
)

data class WorldBookEntry(
    val id: String,
    val title: String,
    val keywords: List<String>,
    val content: String,
    val enabled: Boolean,
    val priority: Int,
    val scope: String
)

data class WorldBookDetail(
    val id: String,
    val name: String,
    val description: String,
    val enabled: Boolean,
    val triggerRule: String,
    val priority: Int,
    val scope: String,
    val entries: List<WorldBookEntry>
)

data class MemoryGraphNodeView(
    val id: String,
    val label: String,
    val type: String,
    val x: Float,
    val y: Float
)

data class MemoryGraphEdgeView(
    val source: String,
    val target: String,
    val relation: String
)

data class MemoryGraphView(
    val nodes: List<MemoryGraphNodeView>,
    val edges: List<MemoryGraphEdgeView>,
    val totalNodes: Int,
    val totalEdges: Int
)

enum class GraphNodeFilter { All, Character, Person, Place, Event }

data class PendingMemoryItem(
    val id: String,
    val content: String,
    val source: String,
    val suggestType: String,
    val timestamp: String
)

data class MemoryConflictItem(
    val id: String,
    val field: String,
    val oldValue: String,
    val newValue: String,
    val source: String,
    val timestamp: String,
    val confidence: Float
)

data class ImportFileItem(
    val id: String,
    val name: String,
    val size: String,
    val format: String,
    val selected: Boolean = false
)

data class FieldMappingItem(
    val targetField: String,
    val sourceField: String?,
    val availableSources: List<String>
)

data class ExportConfig(
    val characterId: String? = null,
    val types: List<String> = emptyList(),
    val startTime: String = "",
    val endTime: String = "",
    val desensitize: Boolean = true,
    val includeMedia: Boolean = false,
    val format: String = "json"
)

data class MemorySettingsConfig(
    val autoWrite: Boolean = true,
    val requireConfirm: Boolean = false,
    val mergeStrategy: String = "智能合并",
    val importanceThreshold: Float = 0.3f,
    val vectorRetrieveCount: Int = 5,
    val timeDecay: Boolean = true,
    val graphSync: Boolean = true
)
