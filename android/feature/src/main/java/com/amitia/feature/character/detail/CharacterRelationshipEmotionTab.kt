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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Favorite
import androidx.compose.material.icons.outlined.History
import androidx.compose.material.icons.outlined.Mood
import androidx.compose.material.icons.outlined.People
import androidx.compose.material.icons.outlined.TrendingDown
import androidx.compose.material.icons.outlined.TrendingFlat
import androidx.compose.material.icons.outlined.TrendingUp
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.TimelineItem
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.CharacterEmotionState
import com.amitia.feature.character.model.CharacterRelationship
import com.amitia.feature.character.model.EmotionTrend
import com.amitia.feature.character.model.RelationshipEvent

@Composable
fun CharacterRelationshipTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.relationshipState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadRelationships() }
        )
        is ScreenState.Content -> {
            val (relationships, events) = (state as ScreenState.Content).data
            RelationshipContent(
                relationships = relationships,
                events = events,
                modifier = Modifier.padding(contentPadding)
            )
        }
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun RelationshipContent(
    relationships: List<CharacterRelationship>,
    events: List<RelationshipEvent>,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "current_rel") {
            AmitiaSection(title = "当前关系定义") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    relationships.forEach { rel ->
                        RelationshipCard(rel)
                    }
                }
            }
        }
        item(key = "timeline_title") {
            Text(
                text = "关系时间线",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 4.dp)
            )
        }
        items(events, key = { it.id }) { event ->
            TimelineItem(
                title = event.title,
                description = event.description,
                timestamp = event.timestamp,
                icon = Icons.Outlined.History,
                isLast = event == events.last()
            )
        }
        item(key = "offline_note") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
            ) {
                Row(
                    modifier = Modifier.padding(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Icon(
                        imageVector = Icons.Outlined.Favorite,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.tertiary,
                        modifier = Modifier.size(18.dp)
                    )
                    Text(
                        text = "离线重逢时关系会产生时间效应，长时间未互动可能影响亲密度",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onTertiaryContainer
                    )
                }
            }
        }
    }
}

@Composable
private fun RelationshipCard(rel: CharacterRelationship) {
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
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.People,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = rel.targetName,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.size(8.dp))
                    Surface(
                        shape = CircleShape,
                        color = MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Text(
                            text = rel.relationLabel,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp)
                        )
                    }
                }
                Text(
                    text = "阶段：${rel.stage}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (rel.lastEvent != null) {
                    Text(
                        text = "最近：${rel.lastEvent}（${rel.lastEventTime}）",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(
                    text = "${rel.intimacy}",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "亲密度",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
fun CharacterEmotionTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.emotionState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadEmotion() }
        )
        is ScreenState.Content -> EmotionContent(
            emotion = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun EmotionContent(
    emotion: CharacterEmotionState,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "current_mood") {
            CurrentMoodCard(emotion)
        }
        item(key = "factors") {
            AmitiaSection(title = "影响因素") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    emotion.factors.forEach { factor ->
                        FactorRow(factor.label, factor.contribution)
                    }
                }
            }
        }
        item(key = "triggers") {
            AmitiaSection(title = "最近触发事件") {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    emotion.recentTriggers.forEach { trigger ->
                        TriggerRow(trigger.event, trigger.emotionChange, trigger.timestamp)
                    }
                }
            }
        }
        item(key = "system_config") {
            EmotionSystemCard(emotion)
        }
    }
}

@Composable
private fun CurrentMoodCard(emotion: CharacterEmotionState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .clip(CircleShape)
                        .background(emotion.moodColor.copy(alpha = 0.2f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Outlined.Mood,
                        contentDescription = null,
                        tint = emotion.moodColor,
                        modifier = Modifier.size(24.dp)
                    )
                }
                Spacer(modifier = Modifier.size(16.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "当前情绪",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = emotion.currentMood,
                        style = MaterialTheme.typography.headlineSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                }
                TrendIcon(emotion.trend)
            }
            Spacer(modifier = Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(
                    text = "强度",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                LinearProgressIndicator(
                    progress = { emotion.intensity / 100f },
                    modifier = Modifier.weight(1f),
                    color = emotion.moodColor,
                    trackColor = MaterialTheme.colorScheme.surfaceVariant
                )
                Text(
                    text = "${emotion.intensity}",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
    }
}

@Composable
private fun TrendIcon(trend: EmotionTrend) {
    val (icon, color, label) = when (trend) {
        EmotionTrend.Rising -> Triple(Icons.Outlined.TrendingUp, Color(0xFF7FB28E), "上升")
        EmotionTrend.Stable -> Triple(Icons.Outlined.TrendingFlat, MaterialTheme.colorScheme.onSurfaceVariant, "平稳")
        EmotionTrend.Falling -> Triple(Icons.Outlined.TrendingDown, Color(0xFFD5A05C), "下降")
    }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        Icon(imageVector = icon, contentDescription = null, tint = color, modifier = Modifier.size(20.dp))
        Text(text = label, style = MaterialTheme.typography.labelMedium, color = color)
    }
}

@Composable
private fun FactorRow(label: String, contribution: Int) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.weight(1f)
        )
        LinearProgressIndicator(
            progress = { contribution / 100f },
            modifier = Modifier.width(120.dp),
            color = MaterialTheme.colorScheme.primary,
            trackColor = MaterialTheme.colorScheme.surfaceVariant
        )
        Text(
            text = "$contribution%",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun TriggerRow(event: String, change: String, time: String) {
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
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = event,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = time,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
            Surface(
                shape = RoundedCornerShape(8.dp),
                color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.5f)
            ) {
                Text(
                    text = change,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onTertiaryContainer,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                )
            }
        }
    }
}

@Composable
private fun EmotionSystemCard(emotion: CharacterEmotionState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "情绪系统",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            com.amitia.core.designsystem.component.AmitiaSwitchRow(
                title = "启用情绪系统",
                subtitle = "角色会根据互动产生情绪变化",
                checked = emotion.systemEnabled,
                onCheckedChange = {}
            )
            if (emotion.systemEnabled) {
                Spacer(modifier = Modifier.height(8.dp))
                AmitiaSliderRow(
                    label = "系统强度",
                    value = emotion.systemIntensity / 100f
                )
            }
        }
    }
}

@Composable
private fun AmitiaSliderRow(label: String, value: Float) {
    com.amitia.core.designsystem.component.AmitiaSlider(
        value = value,
        onValueChange = {},
        label = label,
        valueFormatter = { "${(it * 100).toInt()}%" }
    )
}

@Preview(name = "Relationship - Light", showBackground = true)
@Composable
private fun CharacterRelationshipLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            RelationshipContent(
                relationships = listOf(
                    CharacterRelationship("1", "用户", "主人", "信任期", 85, "一起完成项目", "2天前")
                ),
                events = listOf(
                    RelationshipEvent("1", "初次相遇", "认识用户", "2025-03-15", "关系建立")
                )
            )
        }
    }
}

@Preview(name = "Emotion - Dark", showBackground = true)
@Composable
private fun CharacterEmotionDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            EmotionContent(
                emotion = CharacterEmotionState(
                    currentMood = "愉悦",
                    moodColor = Color(0xFF7FB28E),
                    intensity = 72,
                    trend = EmotionTrend.Stable,
                    factors = listOf(),
                    recentTriggers = listOf(),
                    systemEnabled = true,
                    systemIntensity = 70
                )
            )
        }
    }
}
