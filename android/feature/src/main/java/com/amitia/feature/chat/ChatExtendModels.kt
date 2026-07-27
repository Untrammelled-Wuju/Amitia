package com.amitia.feature.chat

import com.amitia.core.designsystem.component.AmitiaStatusType
import com.amitia.core.designsystem.component.MessageStatus
import com.amitia.core.model.CharacterDto
import com.amitia.core.model.MessageDto

data class ConversationListItem(
    val id: String,
    val title: String,
    val characterId: String?,
    val characterName: String,
    val characterAvatar: String? = null,
    val lastMessage: String,
    val lastMessageAt: String,
    val channel: String,
    val unreadCount: Int = 0,
    val pinned: Boolean = false,
    val muted: Boolean = false,
    val archived: Boolean = false,
    val isCurrent: Boolean = false
)

enum class ConversationSortOrder { LastMessage, UnreadFirst, PinnedFirst }

data class AttachmentOption(
    val id: String,
    val label: String,
    val iconType: AttachmentIcon,
    val enabled: Boolean = true,
    val permissionRequired: Boolean = false
)

enum class AttachmentIcon { Image, Camera, File, Memory, Location, Contact }

data class ChatFileInfo(
    val id: String,
    val name: String,
    val mimeType: String,
    val sizeBytes: Long,
    val url: String,
    val uploadedAt: String,
    val uploadStatus: FileUploadStatus,
    val modelRead: Boolean = false,
    val thumbnailUrl: String? = null
)

enum class FileUploadStatus(val label: String, val status: AmitiaStatusType) {
    Uploading("上传中", AmitiaStatusType.Pending),
    Uploaded("已上传", AmitiaStatusType.Running),
    Failed("上传失败", AmitiaStatusType.Failed)
}

data class ToolExecutionDetail(
    val id: String,
    val toolName: String,
    val purpose: String,
    val inputSummary: String,
    val outputSummary: String,
    val status: MessageStatus,
    val duration: String,
    val approved: Boolean,
    val requiresApproval: Boolean,
    val errorMessage: String? = null,
    val sensitiveFields: List<String> = emptyList()
)

data class ContextItem(
    val id: String,
    val title: String,
    val preview: String,
    val type: ContextType,
    val tokenCount: Int,
    val included: Boolean = true,
    val removable: Boolean = true
)

enum class ContextType(val label: String) {
    Character("角色设定"),
    WorldBook("世界书"),
    RecentMessage("近期消息"),
    LongTermMemory("长期记忆"),
    FileContext("文件上下文"),
    ToolResult("工具结果")
}

data class ContextSummary(
    val totalTokens: Int,
    val maxTokens: Int,
    val items: List<ContextItem>
)

data class MemoryReferenceDetail(
    val id: String,
    val title: String,
    val content: String,
    val source: String,
    val relevance: Float,
    val createdAt: String,
    val quotedIn: String
)

data class ExportOption(
    val format: ExportFormat,
    val description: String,
    val iconType: ExportIcon
)

enum class ExportFormat(val label: String) { Json("结构化 JSON"), Markdown("Markdown"), Document("可阅读文档") }
enum class ExportIcon { Code, Text, Book }

data class ExportConfig(
    val format: ExportFormat,
    val dateRangeStart: String?,
    val dateRangeEnd: String?,
    val includeMedia: Boolean,
    val includeToolRecords: Boolean,
    val privacyMask: Boolean
)

data class ConversationSettings(
    val channel: String,
    val modelRoute: String,
    val autoPlayVoice: Boolean,
    val mergeConsecutiveMessages: Boolean,
    val memoryWriteStrategy: MemoryWriteStrategy,
    val notificationsEnabled: Boolean
)

enum class MemoryWriteStrategy(val label: String) { Auto("自动写入"), Manual("手动确认"), Disabled("不写入") }

data class PromptTraceStage(
    val id: String,
    val name: String,
    val description: String,
    val tokenCount: Int,
    val timestamp: String,
    val content: String
)

data class PromptTraceData(
    val stages: List<PromptTraceStage>,
    val totalTokens: Int,
    val recordedAt: String
)

data class MediaItem(
    val id: String,
    val type: MediaType,
    val url: String,
    val thumbnailUrl: String?,
    val title: String,
    val timestamp: String,
    val size: String? = null
)

enum class MediaType(val label: String) { Image("图片"), File("文件"), Voice("语音"), Link("链接") }

data class MessageSearchResult(
    val message: MessageDto,
    val conversationTitle: String,
    val characterName: String,
    val matchedSnippet: String
)

data class MessageSearchFilter(
    val keyword: String = "",
    val characterId: String? = null,
    val channel: String? = null,
    val messageType: String? = null,
    val dateFrom: String? = null,
    val dateTo: String? = null
)

data class ChannelOption(
    val id: String,
    val name: String,
    val description: String,
    val available: Boolean,
    val isLastUsed: Boolean = false
)

data class MergeHintState(
    val active: Boolean,
    val firstInputAt: String?,
    val remainingSeconds: Int
)

data class MessageDetailData(
    val message: MessageDto,
    val channel: String,
    val characterName: String,
    val referencedBy: List<String>,
    val relatedMemories: List<String>,
    val toolExecutions: List<ToolExecutionDetail>
)
