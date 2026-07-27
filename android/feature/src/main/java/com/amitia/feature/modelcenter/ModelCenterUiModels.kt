package com.amitia.feature.modelcenter

import com.amitia.core.model.ModelDto

data class ProviderUiModel(
    val id: String,
    val name: String,
    val type: String,
    val authStatus: ProviderAuthStatus,
    val available: Boolean,
    val lastTested: String?,
    val roleCount: Int,
    val baseUrl: String? = null,
    val apiKeyMasked: String? = null,
    val models: List<ModelUiModel> = emptyList()
)

enum class ProviderAuthStatus(val label: String) {
    Authorized("已授权"),
    Unauthorized("未授权"),
    Expired("已过期"),
    Unknown("未知")
}

data class ModelUiModel(
    val id: String,
    val name: String,
    val provider: String,
    val type: ModelType,
    val contextWindow: Int? = null,
    val maxTokens: Int? = null,
    val temperature: Double? = null,
    val enabled: Boolean = false,
    val isDefault: Boolean = false,
    val capabilities: List<String> = emptyList(),
    val endpoint: String? = null,
    val apiKeyMasked: String? = null,
    val dimension: Int? = null,
    val languages: List<String> = emptyList(),
    val streaming: Boolean = false,
    val toolCalling: Boolean = false,
    val visionCompatible: Boolean = false,
    val latency: String? = null,
    val voiceCount: Int? = null,
    val imageLimit: Int? = null,
    val inputTypes: List<String> = emptyList(),
    val qdrantCompatible: Boolean? = null,
    val boundCharacters: List<String> = emptyList(),
    val createdAt: String? = null,
    val updatedAt: String? = null
)

enum class ModelType(val label: String) {
    LLM("文本模型"),
    Vision("视觉模型"),
    TTS("语音合成"),
    STT("语音识别"),
    Embedding("向量模型"),
    Unknown("未分类");

    companion object {
        fun fromString(type: String?): ModelType = when (type?.lowercase()) {
            "llm", "text" -> LLM
            "vision" -> Vision
            "tts" -> TTS
            "stt", "asr" -> STT
            "embedding" -> Embedding
            else -> Unknown
        }
    }
}

fun ModelDto.toUiModel(): ModelUiModel = ModelUiModel(
    id = id,
    name = name,
    provider = provider ?: "未知",
    type = ModelType.fromString(type),
    contextWindow = contextWindow,
    maxTokens = maxTokens,
    temperature = temperature,
    enabled = enabled,
    isDefault = isDefault,
    capabilities = capabilities,
    endpoint = endpoint,
    apiKeyMasked = apiKey?.let { maskKey(it) },
    createdAt = createdAt,
    updatedAt = updatedAt
)

private fun maskKey(key: String): String {
    if (key.length <= 8) return "****"
    return key.take(4) + "****" + key.takeLast(4)
}

data class RoutingConfigUiModel(
    val defaultModelId: String? = null,
    val defaultModelName: String? = null,
    val characterRoutes: List<CharacterRouteUiModel> = emptyList(),
    val taskRoutes: List<TaskRouteUiModel> = emptyList(),
    val priorityMode: RoutingPriority = RoutingPriority.Balanced,
    val localFirst: Boolean = false
)

data class CharacterRouteUiModel(
    val characterId: String,
    val characterName: String,
    val modelId: String,
    val modelName: String
)

data class TaskRouteUiModel(
    val taskType: String,
    val taskLabel: String,
    val modelId: String,
    val modelName: String
)

enum class RoutingPriority(val label: String) {
    CostFirst("成本优先"),
    LatencyFirst("延迟优先"),
    Balanced("均衡"),
    QualityFirst("质量优先")
}

data class FallbackChainUiModel(
    val id: String,
    val name: String,
    val primaryModelId: String,
    val primaryModelName: String,
    val fallbackModels: List<FallbackModelUiModel> = emptyList(),
    val enabled: Boolean = true
)

data class FallbackModelUiModel(
    val modelId: String,
    val modelName: String,
    val triggerErrors: List<String> = emptyList(),
    val order: Int
)

data class ModelTestResultUiModel(
    val success: Boolean,
    val response: String? = null,
    val latencyMs: Long? = null,
    val tokensUsed: Int? = null,
    val errorMessage: String? = null,
    val timestamp: String
)

data class UsageStatsUiModel(
    val totalRequests: Long,
    val totalTokens: Long,
    val avgLatencyMs: Long,
    val failureRate: Float,
    val characterDistribution: List<CharacterUsageUiModel> = emptyList(),
    val timeRange: String
)

data class CharacterUsageUiModel(
    val characterName: String,
    val requestCount: Long,
    val tokenCount: Long,
    val percentage: Float
)

data class DiagnosticItemUiModel(
    val id: String,
    val title: String,
    val category: DiagnosticCategory,
    val status: DiagnosticStatus,
    val description: String,
    val detail: String? = null,
    val suggestion: String? = null,
    val timestamp: String? = null
)

enum class DiagnosticCategory(val label: String) {
    Auth("认证"),
    RateLimit("限流"),
    ContextLength("上下文长度"),
    ToolCall("工具调用"),
    Voice("语音冲突"),
    Fallback("回退链")
}

enum class DiagnosticStatus(val label: String) {
    Pass("正常"),
    Warning("警告"),
    Failed("异常"),
    Skipped("已跳过")
}
