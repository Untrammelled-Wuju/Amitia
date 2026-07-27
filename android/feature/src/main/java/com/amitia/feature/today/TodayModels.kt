package com.amitia.feature.today

import com.amitia.core.designsystem.component.AmitiaStatusType
import com.amitia.core.model.CharacterDto

data class TodaySummary(
    val greeting: String,
    val periodLabel: String,
    val character: CharacterDto?,
    val characterMood: String,
    val characterActivity: String,
    val proactivePreview: String?,
    val nextSchedule: ScheduleItem?,
    val recentMemory: MemoryBrief?,
    val online: Boolean,
    val issues: List<RuntimeIssue> = emptyList()
)

data class ScheduleItem(
    val id: String,
    val title: String,
    val time: String,
    val done: Boolean = false,
    val description: String? = null
)

data class MemoryBrief(
    val id: String,
    val title: String,
    val preview: String,
    val importance: Int = 0
)

data class TodayActivity(
    val id: String,
    val title: String,
    val description: String?,
    val timestamp: String,
    val category: ActivityCategory,
    val characterName: String?,
    val iconType: ActivityIconType
)

enum class ActivityCategory { Character, Channel, Memory, Schedule, System }

enum class ActivityIconType { Chat, Hub, Memory, Event, Settings }

data class NotificationItem(
    val id: String,
    val title: String,
    val content: String,
    val timestamp: String,
    val category: NotificationCategory,
    val read: Boolean = false
)

enum class NotificationCategory(val label: String) {
    CharacterMessage("角色消息"),
    Schedule("日程提醒"),
    Channel("渠道异常"),
    Update("扩展更新"),
    System("系统与安全")
}

data class SearchResultGroup(
    val id: String,
    val label: String,
    val items: List<SearchResultItem>
)

data class SearchResultItem(
    val id: String,
    val title: String,
    val subtitle: String,
    val type: SearchItemType
)

enum class SearchItemType { Conversation, Memory, Character, File, Message }

data class RuntimeIssue(
    val id: String,
    val title: String,
    val description: String,
    val level: IssueLevel,
    val fixable: Boolean = true
)

enum class IssueLevel(val status: AmitiaStatusType) {
    Critical(AmitiaStatusType.Failed),
    Warning(AmitiaStatusType.Degraded),
    Info(AmitiaStatusType.Pending)
}

data class QuickAction(
    val id: String,
    val label: String,
    val iconType: QuickActionIcon
)

enum class QuickActionIcon { Chat, Voice, Memory, Schedule, SwitchCharacter, Import }
