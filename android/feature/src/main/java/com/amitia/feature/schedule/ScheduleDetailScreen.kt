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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.amitiaStatusColor

@Composable
fun ScheduleDetailScreen(
    scheduleId: String,
    onBack: () -> Unit,
    onEdit: (String) -> Unit,
    viewModel: ScheduleDetailViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LaunchedEffect(scheduleId) { viewModel.load(scheduleId) }
    ScheduleDetailContent(
        state = state,
        onBack = onBack,
        onEdit = { onEdit(scheduleId) },
        onDelete = { viewModel.delete(onBack) },
        onRetry = { viewModel.load(scheduleId) }
    )
}

@Composable
fun ScheduleDetailContent(
    state: ScreenState<ScheduleDetailData>,
    onBack: () -> Unit,
    onEdit: () -> Unit,
    onDelete: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "日程详情", onBack = onBack)
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
                title = "日程不存在",
                description = "该日程可能已被删除",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> DetailBody(
                item = state.data.item,
                onEdit = onEdit,
                onDelete = onDelete
            )
            is ScreenState.Partial -> DetailBody(
                item = state.data.item,
                onEdit = onEdit,
                onDelete = onDelete
            )
        }
    }
}

@Composable
private fun DetailBody(item: ScheduleItem, onEdit: () -> Unit, onDelete: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        DetailHeader(item = item)
        AmitiaSectionHeader(title = "基本信息")
        InfoRow(icon = AmitiaIcons.Schedule, label = "时间", value = "${item.startTime} - ${item.endTime}")
        InfoRow(icon = AmitiaIcons.Person, label = "角色", value = item.role)
        InfoRow(icon = AmitiaIcons.Info, label = "状态", value = item.status.label, valueColor = amitiaStatusColor(item.status.statusType))
        InfoRow(icon = AmitiaIcons.History, label = "来源", value = item.source.label)
        if (item.channel != null) {
            InfoRow(icon = AmitiaIcons.Hub, label = "渠道", value = item.channel)
        }
        if (item.triggerAction != null) {
            AmitiaSectionHeader(title = "触发动作")
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
            ) {
                Row(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Bolt,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary
                    )
                    Text(
                        text = item.triggerAction,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }
        }
        if (item.reminder != null) {
            AmitiaSectionHeader(title = "提醒")
            InfoRow(icon = AmitiaIcons.Notifications, label = "提醒", value = item.reminder)
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        PrimaryButton(
            text = "编辑日程",
            onClick = onEdit,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Edit
        )
        SecondaryButton(
            text = "复制为新日程",
            onClick = onEdit,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.ContentCopy
        )
        DangerButton(
            text = "删除日程",
            onClick = onDelete,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Delete
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

@Composable
private fun DetailHeader(item: ScheduleItem) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Text(
                text = item.title,
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                val statusColor = amitiaStatusColor(item.status.statusType)
                Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
                Text(
                    text = item.status.label,
                    style = MaterialTheme.typography.labelMedium,
                    color = statusColor
                )
                if (item.isRoleSchedule) {
                    Surface(shape = RoundedCornerShape(6.dp), color = MaterialTheme.colorScheme.tertiaryContainer) {
                        Text(
                            text = "角色日程",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer,
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Xs, vertical = 1.dp)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoRow(
    icon: ImageVector,
    label: String,
    value: String,
    valueColor: androidx.compose.ui.graphics.Color = MaterialTheme.colorScheme.onSurface
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(MaterialTheme.colorScheme.surface)
            .padding(AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Box(
            modifier = Modifier.size(36.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(imageVector = icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(18.dp))
        }
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = valueColor,
            fontWeight = FontWeight.Medium
        )
    }
}

@Preview(name = "ScheduleDetail - Light", showBackground = true)
@Composable
private fun ScheduleDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ScheduleDetailContent(
            state = ScreenState.Content(ScheduleDetailData(ScheduleMockData.todaySchedules[1])),
            onBack = {}, onEdit = {}, onDelete = {}, onRetry = {}
        )
    }
}

@Preview(name = "ScheduleDetail - Dark", showBackground = true)
@Composable
private fun ScheduleDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ScheduleDetailContent(
            state = ScreenState.Loading,
            onBack = {}, onEdit = {}, onDelete = {}, onRetry = {}
        )
    }
}
