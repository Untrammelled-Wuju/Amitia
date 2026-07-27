package com.amitia.feature.character.detail

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.Schedule
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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.ProactiveFrequency
import com.amitia.feature.character.model.ProactiveMessageRecord
import com.amitia.feature.character.model.ProactiveMessageRule
import com.amitia.feature.character.model.QuietHours
import com.amitia.feature.character.model.TimeWindow

@Composable
fun CharacterProactiveTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.proactiveState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadProactive() }
        )
        is ScreenState.Content -> ProactiveContent(
            rule = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun ProactiveContent(
    rule: ProactiveMessageRule,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "toggle") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    AmitiaSwitchRow(
                        title = "启用主动消息",
                        subtitle = "角色会在合适的时间主动发送消息",
                        checked = rule.enabled,
                        onCheckedChange = {},
                        leadingIcon = Icons.Outlined.NotificationsActive
                    )
                }
            }
        }
        item(key = "next_window") {
            NextWindowCard(rule.nextCandidateWindow)
        }
        item(key = "time_windows") {
            AmitiaSection(title = "时间窗口", subtitle = "角色可以发送主动消息的时间段") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    rule.timeWindows.forEach { window ->
                        TimeWindowRow(window)
                    }
                }
            }
        }
        item(key = "frequency") {
            FrequencyCard(rule.frequency)
        }
        item(key = "quiet_hours") {
            QuietHoursCard(rule.quietHours)
        }
        item(key = "triggers") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    AmitiaSwitchRow(
                        title = "生活状态触发",
                        subtitle = "根据角色生活状态变化触发主动消息",
                        checked = rule.lifeStatusTrigger,
                        onCheckedChange = {}
                    )
                }
            }
        }
        item(key = "channel_assignment") {
            AmitiaSection(title = "渠道分配", subtitle = "主动消息发送到哪些渠道") {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    rule.channelAssignment.forEach { channel ->
                        Surface(
                            shape = RoundedCornerShape(20.dp),
                            color = MaterialTheme.colorScheme.primaryContainer
                        ) {
                            Text(
                                text = channel,
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.onPrimaryContainer,
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp)
                            )
                        }
                    }
                }
            }
        }
        item(key = "recent_messages") {
            AmitiaSection(title = "最近主动消息") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    rule.recentMessages.forEach { record ->
                        ProactiveMessageRow(record)
                    }
                }
            }
        }
        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "预览消息",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "保存设置",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun NextWindowCard(nextWindow: String?) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.2f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Schedule,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "下次候选时间",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                )
                Text(
                    text = nextWindow ?: "暂无候选",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium
                )
            }
        }
    }
}

@Composable
private fun TimeWindowRow(window: TimeWindow) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(10.dp)
                    .clip(CircleShape)
                    .background(
                        if (window.enabled) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.outlineVariant
                    )
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = window.label,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "${window.startHour}:00 - ${window.endHour}:00",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Surface(
                shape = RoundedCornerShape(8.dp),
                color = if (window.enabled) MaterialTheme.colorScheme.primaryContainer
                else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = if (window.enabled) "启用" else "已禁用",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (window.enabled) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Composable
private fun FrequencyCard(frequency: ProactiveFrequency) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "频率控制",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                FrequencyItem("最小间隔", "${frequency.minIntervalMinutes} 分钟")
                FrequencyItem("每日上限", "${frequency.maxPerDay} 条")
                FrequencyItem("随机模式", if (frequency.randomMode) "开启" else "关闭")
            }
        }
    }
}

@Composable
private fun FrequencyItem(label: String, value: String) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            style = MaterialTheme.typography.titleMedium,
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
private fun QuietHoursCard(quietHours: QuietHours) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            AmitiaSwitchRow(
                title = "免打扰时段",
                subtitle = "在此时段内不会发送主动消息",
                checked = quietHours.enabled,
                onCheckedChange = {}
            )
            if (quietHours.enabled) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "${quietHours.startHour}:00 - ${quietHours.endHour}:00",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
    }
}

@Composable
private fun ProactiveMessageRow(record: ProactiveMessageRecord) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = record.content,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 2
                )
                Text(
                    text = record.time,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
            Surface(
                shape = RoundedCornerShape(8.dp),
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = record.channel,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Preview(name = "Proactive - Light", showBackground = true)
@Composable
private fun CharacterProactiveLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ProactiveContent(
                rule = ProactiveMessageRule(
                    enabled = true,
                    timeWindows = listOf(TimeWindow("1", "早晨", 8, 10, true)),
                    frequency = ProactiveFrequency(120, 5, true),
                    quietHours = QuietHours(true, 23, 7),
                    lifeStatusTrigger = true,
                    channelAssignment = listOf("Web"),
                    recentMessages = listOf(ProactiveMessageRecord("1", "你好", "Web", "今天")),
                    nextCandidateWindow = "今天 19:00 - 22:00"
                )
            )
        }
    }
}

@Preview(name = "Proactive - Dark", showBackground = true)
@Composable
private fun CharacterProactiveDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ProactiveContent(
                rule = ProactiveMessageRule(
                    enabled = false,
                    timeWindows = listOf(),
                    frequency = ProactiveFrequency(60, 3, false),
                    quietHours = QuietHours(false, 0, 0),
                    lifeStatusTrigger = false,
                    channelAssignment = listOf(),
                    recentMessages = listOf(),
                    nextCandidateWindow = null
                )
            )
        }
    }
}
