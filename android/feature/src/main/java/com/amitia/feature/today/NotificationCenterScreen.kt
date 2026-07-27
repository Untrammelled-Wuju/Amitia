package com.amitia.feature.today

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.TertiaryButton
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun NotificationCenterScreen(
    onBack: () -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val state by viewModel.notificationState.collectAsStateWithLifecycle()
    val filter by viewModel.notificationFilter.collectAsStateWithLifecycle()

    val filters = NotificationFilter.entries.map { AmitiaChipItem(it.label, it == filter) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "通知中心",
            onBack = onBack,
            actions = {
                TertiaryButton(
                    text = "全部已读",
                    onClick = viewModel::markAllNotificationsRead
                )
            }
        )
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaChipSelector(
                items = filters,
                onToggle = { index ->
                    NotificationFilter.entries.getOrNull(index)?.let(viewModel::setNotificationFilter)
                },
                multiSelect = false
            )
        }
        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
        when (val s = state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.padding(AmitiaSpacing.Base)) { LoadingSkeleton(lineCount = 4) }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = s.error.title,
                    description = s.error.message,
                    onRetry = viewModel::loadNotifications
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Notifications,
                    title = "没有通知",
                    description = "新的消息和提醒会出现在这里"
                )
            }
            else -> {
                val all = (state as ScreenState.Content<List<NotificationItem>>).data
                val filtered = if (filter == NotificationFilter.All) all
                else all.filter { it.category == mapFilter(filter) }
                if (filtered.isEmpty()) {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.NotificationsOff,
                        title = "该分类下暂无通知"
                    )
                } else {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(bottom = 100.dp)
                    ) {
                        items(filtered, key = { it.id }) { item ->
                            NotificationRow(item = item) { viewModel.markNotificationRead(item.id) }
                            HorizontalDivider(
                                color = MaterialTheme.colorScheme.outlineVariant,
                                modifier = Modifier.padding(start = AmitiaSpacing.Base)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun NotificationRow(item: NotificationItem, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            )
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Md),
        verticalAlignment = Alignment.Top,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Box(
            modifier = Modifier
                .size(40.dp)
                .clip(CircleShape)
                .background(notificationCategoryColor(item.category).copy(alpha = 0.15f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = notificationIcon(item.category),
                contentDescription = null,
                tint = notificationCategoryColor(item.category),
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (!item.read) {
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primary)
                    )
                    androidx.compose.foundation.layout.Spacer(modifier = Modifier.size(AmitiaSpacing.Xs))
                }
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    fontWeight = if (item.read) FontWeight.Normal else FontWeight.Medium,
                    modifier = Modifier.weight(1f)
                )
                Text(
                    text = item.timestamp,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
            Text(
                text = item.content,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

private fun mapFilter(filter: NotificationFilter): NotificationCategory = when (filter) {
    NotificationFilter.CharacterMessage -> NotificationCategory.CharacterMessage
    NotificationFilter.Schedule -> NotificationCategory.Schedule
    NotificationFilter.Channel -> NotificationCategory.Channel
    NotificationFilter.Update -> NotificationCategory.Update
    NotificationFilter.System -> NotificationCategory.System
    NotificationFilter.All -> NotificationCategory.CharacterMessage
}

@Composable
private fun notificationCategoryColor(category: NotificationCategory) = when (category) {
    NotificationCategory.CharacterMessage -> MaterialTheme.colorScheme.primary
    NotificationCategory.Schedule -> MaterialTheme.colorScheme.tertiary
    NotificationCategory.Channel -> AmitiaStateColors.Degraded
    NotificationCategory.Update -> AmitiaStateColors.Running
    NotificationCategory.System -> AmitiaStateColors.Failed
}

private fun notificationIcon(category: NotificationCategory): ImageVector = when (category) {
    NotificationCategory.CharacterMessage -> AmitiaIcons.Chat
    NotificationCategory.Schedule -> AmitiaIcons.Event
    NotificationCategory.Channel -> AmitiaIcons.Hub
    NotificationCategory.Update -> AmitiaIcons.Upgrade
    NotificationCategory.System -> AmitiaIcons.Security
}

@Preview(name = "Notification Center - Light", showBackground = true)
@Composable
private fun NotificationCenterLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Notification Center", style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Preview(name = "Notification Center - Dark", showBackground = true)
@Composable
private fun NotificationCenterDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Notification Center", style = MaterialTheme.typography.titleMedium)
        }
    }
}
