package com.amitia.runtime.extension

import kotlinx.serialization.Serializable

enum class ContributionType(val value: String) {
    Tool("tool"),
    AgentSkill("agent_skill"),
    Workflow("workflow"),
    Mcp("mcp"),
    Ui("ui"),
    Hook("hook"),
    EventSubscription("event_subscription"),
    BackgroundTask("background_task"),
    Provider("provider"),
    Asset("asset"),
    Schedule("schedule");

    companion object {
        fun fromValue(value: String): ContributionType? =
            entries.firstOrNull { it.value == value }
    }
}

@Serializable
data class ExtensionContribution(
    val id: String,
    val type: ContributionType,
    val extensionId: String,
    val moduleId: String = "",
    val enabled: Boolean = true,
    val metadata: Map<String, kotlinx.serialization.json.JsonElement> = emptyMap(),
    val toolId: String? = null,
    val agentSkillId: String? = null,
    val workflowId: String? = null,
    val serverId: String? = null,
    val surfaceId: String? = null,
    val event: String? = null,
    val handler: String? = null,
    val eventType: String? = null,
    val filter: String? = null,
    val sourceIds: List<String> = emptyList()
)

@Serializable
data class ToolDefinition(
    val toolId: String,
    val extensionId: String,
    val moduleId: String,
    val name: String,
    val description: String = "",
    val inputSchema: ToolSchema = ToolSchema(),
    val outputSchema: ToolSchema = ToolSchema(),
    val permissions: List<String> = emptyList(),
    val timeout: Long = 30000,
    val entryPoint: String = ""
)

@Serializable
data class ToolSchema(
    val type: String = "object",
    val properties: Map<String, ToolProperty> = emptyMap(),
    val required: List<String> = emptyList()
)

@Serializable
data class ToolProperty(
    val type: String = "string",
    val description: String = "",
    val default: kotlinx.serialization.json.JsonElement? = null,
    val enum: List<String> = emptyList()
)

@Serializable
data class ToolInvocationRequest(
    val toolId: String,
    val extensionId: String,
    val arguments: Map<String, kotlinx.serialization.json.JsonElement> = emptyMap(),
    val timeout: Long = 30000
)

@Serializable
data class ToolInvocationResult(
    val success: Boolean,
    val output: kotlinx.serialization.json.JsonElement? = null,
    val error: String? = null,
    val durationMs: Long = 0
)
