package com.amitia.feature.schedule

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.amitiaStatusColor

@Composable
fun ScheduleHomeScreen(
    onBack: () -> Unit,
    onNewSchedule: () -> Unit,
    onOpenCalendar: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit,
    viewModel: ScheduleHomeViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ScheduleHomeContent(
        state = state,
        onBack = onBack,
        onNewSchedule = onNewSchedule,
        onOpenCalendar = onOpenCalendar,
        onOpenDetail = onOpenDetail,
        onOpenProactiveWindow = onOpenProactiveWindow,
        onRetry = viewModel::load
    )
}

@Composable
fun ScheduleHomeContent(
    state: ScreenState<ScheduleHomeData>,
    onBack: () -> Unit,
    onNewSchedule: () -> Unit,
    onOpenCalendar: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "日程",
            onBack = onBack,
            actions = {
                AmitiaIconButton(
                    icon = AmitiaIcons.CalendarMonth,
                    contentDescription = "日历",
                    onClick = onOpenCalendar
                )
                AmitiaIconButton(
                    icon = AmitiaIcons.Refresh,
                    contentDescription = "刷新",
                    onClick = onRetry
                )
            }
        )
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.EventOutlined,
                title = "今日暂无日程",
                description = "点击下方按钮创建你的第一个日程",
                modifier = Modifier.fillMaxSize(),
                primaryAction = { PrimaryButton(text = "新建日程", onClick = onNewSchedule, leadingIcon = AmitiaIcons.Add) }
            )
            is ScreenState.Content -> ScheduleHomeBody(
                data = state.data,
                onNewSchedule = onNewSchedule,
                onOpenDetail = onOpenDetail,
                onOpenProactiveWindow = onOpenProactiveWindow
            )
            is ScreenState.Partial -> ScheduleHomeBody(
                data = state.data,
                onNewSchedule = onNewSchedule,
                onOpenDetail = onOpenDetail,
                onOpenProactiveWindow = onOpenProactiveWindow
            )
        }
    }
}

@Composable
private fun ScheduleHomeBody(
    data: ScheduleHomeData,
    onNewSchedule: () -> Unit,
    onOpenDetail: (String) -> Unit,
    onOpenProactiveWindow: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { WeekOverviewCard(overview = data.weekOverview) }
        item {
            AmitiaSectionHeader(title = "今日日程")
        }
        if (data.today.isEmpty()) {
            item {
                EmptyInlineHint(text = "今日暂无日程安排")
            }
        } else {
            items(data.today.size) { index ->
                ScheduleRow(item = data.today[index], onClick = { onOpenDetail(data.today[index].id) })
            }
        }
        item { AmitiaSectionHeader(title = "即将到来") }
        if (data.upcoming.isEmpty()) {
            item { EmptyInlineHint(text = "暂无即将到来的日程") }
        } else {
            items(data.upcoming.size) { index ->
                ScheduleRow(item = data.upcoming[index], onClick = { onOpenDetail(data.upcoming[index].id) })
            }
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xs)) }
        item { ProactiveWindowCard(window = data.proactiveWindow, onClick = onOpenProactiveWindow) }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Lg)) }
        item {
            PrimaryButton(
                text = "新建日程",
                onClick = onNewSchedule,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Add
            )
        }
    }
}

@Composable
private fun WeekOverviewCard(overview: WeekOverview) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = AmitiaIcons.CalendarToday,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(24.dp)
                )
                Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                Text(
                    text = "本周概览",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                OverviewMetric("总日程", overview.totalSchedules.toString(), AmitiaIcons.Event)
                OverviewMetric("已完成", overview.completedCount.toString(), AmitiaIcons.CheckCircle)
                OverviewMetric("即将", overview.upcomingCount.toString(), AmitiaIcons.Schedule)
                OverviewMetric("角色", overview.roleScheduleCount.toString(), AmitiaIcons.Person)
            }
        }
    }
}

@Composable
private fun OverviewMetric(label: String, value: String, icon: androidx.compose.ui.graphics.vector.ImageVector) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(20.dp)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun ScheduleRow(item: ScheduleItem, onClick: () -> Unit) {
    val statusColor = amitiaStatusColor(item.status.statusType)
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = item.startTime,
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = item.endTime,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(statusColor)
            )
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false)
                    )
                    if (item.isRoleSchedule) {
                        Spacer(modifier = Modifier.size(AmitiaSpacing.Xs))
                        Surface(
                            shape = RoundedCornerShape(6.dp),
                            color = MaterialTheme.colorScheme.primaryContainer
                        ) {
                            Text(
                                text = "角色",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimaryContainer,
                                modifier = Modifier.padding(horizontal = AmitiaSpacing.Xs, vertical = 1.dp)
                            )
                        }
                    }
                }
                Text(
                    text = "${item.role} · ${item.source.label}" + (item.channel?.let { " · $it" } ?: ""),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Text(
                text = item.status.label,
                style = MaterialTheme.typography.labelSmall,
                color = statusColor
            )
        }
    }
}

@Composable
private fun ProactiveWindowCard(window: ProactiveMessageWindow, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.AutoAwesome,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "主动消息时间窗",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "${window.startTime} - ${window.endTime} · 每日 ${window.frequencyPerDay} 次",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun EmptyInlineHint(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.bodySmall,
        color = AmitiaColors.OnSurfaceMuted,
        modifier = Modifier.padding(vertical = AmitiaSpacing.Sm)
    )
}

@Preview(name = "ScheduleHome - Light", showBackground = true)
@Composable
private fun ScheduleHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ScheduleHomeContent(
            state = ScreenState.Content(
                ScheduleHomeData(
                    today = ScheduleMockData.todaySchedules,
                    upcoming = ScheduleMockData.upcomingSchedules,
                    weekOverview = ScheduleMockData.weekOverview
                )
            ),
            onBack = {}, onNewSchedule = {}, onOpenCalendar = {}, onOpenDetail = {}, onOpenProactiveWindow = {}, onRetry = {}
        )
    }
}

@Preview(name = "ScheduleHome - Dark", showBackground = true)
@Composable
private fun ScheduleHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ScheduleHomeContent(
            state = ScreenState.Loading,
            onBack = {}, onNewSchedule = {}, onOpenCalendar = {}, onOpenDetail = {}, onOpenProactiveWindow = {}, onRetry = {}
        )
    }
}
