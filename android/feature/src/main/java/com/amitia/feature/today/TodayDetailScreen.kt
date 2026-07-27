package com.amitia.feature.today

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.MemoryCard
import com.amitia.core.designsystem.component.TimelineItem

@Composable
fun TodayDetailScreen(
    onBack: () -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val todayState by viewModel.todayState.collectAsStateWithLifecycle()
    val activityState by viewModel.activityState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "今日详情", onBack = onBack)
        when (val s = todayState) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base)) {
                    LoadingSkeleton(lineCount = 6)
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.CloudOff,
                    title = s.error.title,
                    description = s.error.message,
                    onRetry = viewModel::loadToday
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Today,
                    title = "今天还没有活动",
                    description = "随着对话的进行，活动会出现在这里"
                )
            }
            is ScreenState.Content, is ScreenState.Partial -> {
                val summary = (s as ScreenState.Content<TodaySummary>).data
                TodayDetailContent(
                    summary = summary,
                    activityState = activityState,
                    onRetryActivity = viewModel::loadActivities
                )
            }
        }
    }
}

@Composable
private fun TodayDetailContent(
    summary: TodaySummary,
    activityState: ScreenState<List<TodayActivity>>,
    onRetryActivity: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(bottom = 100.dp),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        item(key = "title") {
            Text(
                text = summary.greeting,
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
            )
        }
        item(key = "timeline_header") {
            AmitiaSectionHeader(
                title = "角色时间线",
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
            )
        }
        when (val as_ = activityState) {
            is ScreenState.Loading -> {
                item(key = "timeline_loading") { LoadingSkeleton(lineCount = 4) }
            }
            is ScreenState.Error -> {
                item(key = "timeline_error") {
                    AmitiaErrorState(
                        icon = AmitiaIcons.Error,
                        title = as_.error.title,
                        description = as_.error.message,
                        onRetry = onRetryActivity
                    )
                }
            }
            is ScreenState.Empty -> {
                item(key = "timeline_empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.Timeline,
                        title = "暂无时间线"
                    )
                }
            }
            else -> {
                val activities = (activityState as ScreenState.Content<List<TodayActivity>>).data
                androidx.compose.foundation.lazy.items(activities, key = { it.id }) { activity ->
                    val index = activities.indexOf(activity)
                    TimelineItem(
                        title = activity.title,
                        description = activity.description,
                        timestamp = activity.timestamp,
                        icon = activityIcon(activity.iconType),
                        isLast = index == activities.lastIndex
                    )
                }
            }
        }
        if (summary.recentMemory != null) {
            item(key = "memory_header") {
                AmitiaSectionHeader(
                    title = "今日记忆",
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
                )
            }
            item(key = "memory_card") {
                Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)) {
                    MemoryCard(
                        title = summary.recentMemory.title,
                        preview = summary.recentMemory.preview,
                        importance = summary.recentMemory.importance,
                        timestamp = "今天"
                    )
                }
            }
        }
        if (summary.nextSchedule != null) {
            item(key = "schedule_header") {
                AmitiaSectionHeader(
                    title = "日程与事件",
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
                )
            }
            item(key = "schedule_item") {
                TimelineItem(
                    title = summary.nextSchedule.title,
                    description = "${summary.nextSchedule.time} · ${summary.nextSchedule.description ?: ""}",
                    timestamp = if (summary.nextSchedule.done) "已完成" else "待开始",
                    icon = AmitiaIcons.Event,
                    isLast = true,
                    iconColor = if (summary.nextSchedule.done) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.tertiary
                )
            }
        }
    }
}

private fun activityIcon(type: ActivityIconType) = when (type) {
    ActivityIconType.Chat -> AmitiaIcons.Chat
    ActivityIconType.Hub -> AmitiaIcons.Hub
    ActivityIconType.Memory -> AmitiaIcons.Memory
    ActivityIconType.Event -> AmitiaIcons.Event
    ActivityIconType.Settings -> AmitiaIcons.Settings
}

@Preview(name = "Today Detail - Light", showBackground = true)
@Composable
private fun TodayDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Today Detail Screen", style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Preview(name = "Today Detail - Dark", showBackground = true)
@Composable
private fun TodayDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Today Detail Screen", style = MaterialTheme.typography.titleMedium)
        }
    }
}
