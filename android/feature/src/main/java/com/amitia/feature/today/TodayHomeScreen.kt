package com.amitia.feature.today

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.WarningBanner
import com.amitia.core.model.CharacterDto

@Composable
fun TodayHomeScreen(
    onOpenChat: () -> Unit,
    onOpenCharacter: (String) -> Unit,
    onOpenNotifications: () -> Unit,
    onOpenSearch: () -> Unit,
    onOpenSchedule: () -> Unit,
    onOpenMemory: () -> Unit,
    onOpenTodayDetail: () -> Unit,
    onOpenActivity: () -> Unit,
    onOpenIssues: () -> Unit,
    onOpenQuickActions: () -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val state by viewModel.todayState.collectAsStateWithLifecycle()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(bottom = 100.dp),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        item(key = "top_bar") {
            TodayTopBar(
                summary = (state as? ScreenState.Content)?.data,
                onOpenNotifications = onOpenNotifications,
                onOpenSearch = onOpenSearch
            )
        }
        when (val s = state) {
            is ScreenState.Loading -> {
                item(key = "skeleton") {
                    Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)) {
                        LoadingSkeleton(lineCount = 5, lineHeight = 20)
                    }
                }
            }
            is ScreenState.Error -> {
                item(key = "error") {
                    AmitiaErrorState(
                        icon = AmitiaIcons.CloudOff,
                        title = s.error.title,
                        description = s.error.message,
                        onRetry = viewModel::loadToday
                    )
                }
            }
            is ScreenState.Empty -> {
                item(key = "empty") {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Xl)) {
                        Text(
                            text = "暂无今日数据",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
            }
            is ScreenState.Content, is ScreenState.Partial -> {
                val summary = (s as ScreenState.Content<TodaySummary>).data
                item(key = "hero") {
                    Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)) {
                        CharacterHeroSection(summary = summary) {
                            summary.character?.let { onOpenCharacter(it.id) }
                        }
                    }
                }
                item(key = "quick_entry") {
                    Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)) {
                        QuickChatEntry(onClick = onOpenChat)
                    }
                }
                item(key = "info_header") {
                    AmitiaSectionHeader(
                        title = "今日",
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base),
                        trailing = { OnlineStatusChip(online = summary.online) }
                    )
                }
                summary.nextSchedule?.let { schedule ->
                    item(key = "schedule") {
                        TodayInfoRow(
                            icon = AmitiaIcons.Event,
                            title = schedule.title,
                            subtitle = "${schedule.time} · ${schedule.description ?: "无备注"}",
                            onClick = onOpenSchedule
                        )
                    }
                }
                summary.recentMemory?.let { memory ->
                    item(key = "memory") {
                        TodayInfoRow(
                            icon = AmitiaIcons.Memory,
                            title = memory.title,
                            subtitle = memory.preview,
                            onClick = onOpenMemory
                        )
                    }
                }
                if (summary.characterActivity.isNotBlank()) {
                    item(key = "activity") {
                        TodayInfoRow(
                            icon = AmitiaIcons.AutoAwesome,
                            title = "角色动态",
                            subtitle = summary.characterActivity,
                            onClick = onOpenTodayDetail
                        )
                    }
                }
                if (summary.issues.isNotEmpty()) {
                    item(key = "issues_banner") {
                        WarningBanner(
                            message = "${summary.issues.size} 项运行异常需要关注",
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base),
                            actionLabel = "查看",
                            onAction = onOpenIssues
                        )
                    }
                }
                item(key = "activity_entry") {
                    AmitiaSectionHeader(
                        title = "活动动态",
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base),
                        trailing = {
                            AmitiaIconButton(
                                icon = AmitiaIcons.ArrowForward,
                                contentDescription = "全部",
                                onClick = onOpenActivity
                            )
                        }
                    )
                }
                item(key = "quick_actions_entry") {
                    Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)) {
                        QuickActionsTrigger(onClick = onOpenQuickActions)
                    }
                }
            }
        }
    }
}

@Composable
private fun TodayTopBar(
    summary: TodaySummary?,
    onOpenNotifications: () -> Unit,
    onOpenSearch: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = summary?.greeting ?: "你好",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = summary?.periodLabel ?: "",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        AmitiaIconButton(
            icon = AmitiaIcons.Search,
            contentDescription = "搜索",
            onClick = onOpenSearch
        )
        AmitiaIconButton(
            icon = AmitiaIcons.Notifications,
            contentDescription = "通知",
            onClick = onOpenNotifications
        )
    }
}

@Composable
private fun QuickActionsTrigger(onClick: () -> Unit) {
    val interactionSource = androidx.compose.runtime.remember { androidx.compose.foundation.interaction.MutableInteractionSource() }
    androidx.compose.material3.Surface(
        modifier = Modifier
            .fillMaxWidth()
            .androidx_clickable(interactionSource, onClick),
        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            androidx.compose.foundation.layout.Box(
                modifier = Modifier
                    .androidx_clip_circle()
                    .androidx_bg_primary()
                    .size(40.dp),
                contentAlignment = Alignment.Center
            ) {
                androidx.compose.material3.Icon(
                    imageVector = AmitiaIcons.Add,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Text(
                text = "快捷操作",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.weight(1f)
            )
            androidx.compose.material3.Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

private fun Modifier.androidx_clickable(
    source: androidx.compose.foundation.interaction.MutableInteractionSource,
    onClick: () -> Unit
): Modifier = this.then(
    androidx.compose.foundation.clickable(
        interactionSource = source,
        indication = null,
        role = androidx.compose.ui.semantics.Role.Button,
        onClick = onClick
    )
)

private fun Modifier.androidx_clip_circle() = this.then(androidx.compose.ui.draw.clip(androidx.compose.foundation.shape.CircleShape))
private fun Modifier.androidx_bg_primary() = this.then(androidx.compose.foundation.background(MaterialTheme.colorScheme.primary))

@Preview(name = "Today Home - Light", showBackground = true)
@Composable
private fun TodayHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        TodayHomeScreenPreviewBody()
    }
}

@Preview(name = "Today Home - Dark", showBackground = true)
@Composable
private fun TodayHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        TodayHomeScreenPreviewBody()
    }
}

@Composable
private fun TodayHomeScreenPreviewBody() {
    val summary = TodaySummary(
        greeting = "下午好",
        periodLabel = "14:32 · 周三",
        character = CharacterDto(id = "1", name = "艾米", isCurrent = true, description = "温柔知性助手"),
        characterMood = "心情宁静，正在整理笔记",
        characterActivity = "在翻阅你最近收藏的文章",
        proactivePreview = "想和你确认明天的会议安排",
        nextSchedule = ScheduleItem("s1", "产品评审会议", "16:00"),
        recentMemory = MemoryBrief("m1", "用户偏好", "喜欢简洁的回复风格", 4),
        online = true
    )
    CharacterHeroSection(summary = summary, onClick = {})
    Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    QuickChatEntry(onClick = {})
}
