package com.amitia.feature.today

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun ActivityFeedScreen(
    onBack: () -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val activityState by viewModel.activityState.collectAsStateWithLifecycle()
    val filter by viewModel.filter.collectAsStateWithLifecycle()

    val filters = ActivityFilter.entries.map { AmitiaChipItem(it.label, it == filter) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "活动动态", onBack = onBack)
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaChipSelector(
                items = filters,
                onToggle = { index ->
                    ActivityFilter.entries.getOrNull(index)?.let(viewModel::setActivityFilter)
                },
                multiSelect = false
            )
        }
        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
        when (val s = activityState) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.padding(AmitiaSpacing.Base)) { LoadingSkeleton(lineCount = 5) }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = s.error.title,
                    description = s.error.message,
                    onRetry = viewModel::loadActivities
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Timeline,
                    title = "暂无动态",
                    description = "角色和渠道的活动会在这里展示"
                )
            }
            else -> {
                val all = (activityState as ScreenState.Content<List<TodayActivity>>).data
                val filtered = if (filter == ActivityFilter.All) all
                else all.filter { matchesFilter(it, filter) }
                val userActivities = filtered.filterNot { it.category == ActivityCategory.System }
                val systemLogs = filtered.filter { it.category == ActivityCategory.System }
                if (filtered.isEmpty()) {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.FilterList,
                        title = "没有匹配的动态"
                    )
                } else {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(bottom = 100.dp),
                        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        if (userActivities.isNotEmpty()) {
                            item(key = "user_header") {
                                SectionDivider("动态")
                            }
                            items(userActivities, key = { it.id }) { activity ->
                                ActivityRow(activity = activity)
                            }
                        }
                        if (systemLogs.isNotEmpty()) {
                            item(key = "system_header") {
                                SectionDivider("系统日志")
                            }
                            items(systemLogs, key = { "sys_${it.id}" }) { activity ->
                                ActivityRow(activity = activity, isSystem = true)
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SectionDivider(label: String) {
    Text(
        text = label,
        style = MaterialTheme.typography.labelMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
    )
}

@Composable
private fun ActivityRow(activity: TodayActivity, isSystem: Boolean = false) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Box(
            modifier = Modifier
                .size(AmitiaIconSize.Large)
                .clip(CircleShape),
            contentAlignment = Alignment.Center
        ) {
            Surface(
                modifier = Modifier.fillMaxSize(),
                shape = CircleShape,
                color = if (isSystem) MaterialTheme.colorScheme.surfaceVariant
                else MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.6f)
            ) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = activityIcon(activity.iconType),
                        contentDescription = null,
                        tint = if (isSystem) MaterialTheme.colorScheme.onSurfaceVariant
                        else MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = activity.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )
                Text(
                    text = activity.timestamp,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
            if (activity.description != null) {
                Text(
                    text = activity.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
            if (activity.characterName != null) {
                Text(
                    text = activity.characterName,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
            }
        }
    }
}

private fun matchesFilter(activity: TodayActivity, filter: ActivityFilter): Boolean {
    return when (filter) {
        ActivityFilter.Character -> activity.category == ActivityCategory.Character
        ActivityFilter.Channel -> activity.category == ActivityCategory.Channel
        ActivityFilter.Memory -> activity.category == ActivityCategory.Memory
        ActivityFilter.Schedule -> activity.category == ActivityCategory.Schedule
        ActivityFilter.All -> true
    }
}

private fun activityIcon(type: ActivityIconType): ImageVector = when (type) {
    ActivityIconType.Chat -> AmitiaIcons.Chat
    ActivityIconType.Hub -> AmitiaIcons.Hub
    ActivityIconType.Memory -> AmitiaIcons.Memory
    ActivityIconType.Event -> AmitiaIcons.Event
    ActivityIconType.Settings -> AmitiaIcons.Settings
}

@Preview(name = "Activity Feed - Light", showBackground = true)
@Composable
private fun ActivityFeedLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Text("Activity Feed", modifier = Modifier.padding(16.dp))
        }
    }
}

@Preview(name = "Activity Feed - Dark", showBackground = true)
@Composable
private fun ActivityFeedDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Text("Activity Feed", modifier = Modifier.padding(16.dp))
        }
    }
}
