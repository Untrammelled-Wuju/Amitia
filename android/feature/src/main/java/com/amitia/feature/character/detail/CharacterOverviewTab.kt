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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.Chat
import androidx.compose.material.icons.outlined.Forum
import androidx.compose.material.icons.outlined.GraphicEq
import androidx.compose.material.icons.outlined.Mic
import androidx.compose.material.icons.outlined.Mood
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.CharacterOverviewData

@Composable
fun CharacterOverviewTab(
    characterId: String,
    viewModel: CharacterDetailViewModel,
    onChat: () -> Unit,
    contentPadding: PaddingValues
) {
    val state by viewModel.overviewState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadOverview(characterId) }
        )
        is ScreenState.Content -> OverviewContent(
            data = (state as ScreenState.Content).data,
            onChat = onChat,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun OverviewContent(
    data: CharacterOverviewData,
    onChat: () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        OverviewHero(data)
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            PrimaryButton(
                text = "对话",
                onClick = onChat,
                leadingIcon = Icons.Outlined.Forum,
                modifier = Modifier.weight(1f)
            )
            SecondaryButton(
                text = "语音",
                onClick = {},
                leadingIcon = Icons.Outlined.GraphicEq,
                modifier = Modifier.weight(1f)
            )
        }
        OverviewInfoCard(
            icon = Icons.Outlined.Mood,
            title = "当前情绪",
            value = data.currentMood
        )
        OverviewInfoCard(
            icon = Icons.Outlined.Schedule,
            title = "生活状态",
            value = data.lifeStatus
        )
        if (data.recentConversation != null) {
            OverviewDetailCard(
                icon = Icons.Outlined.Chat,
                title = "最近对话",
                subtitle = data.recentConversationTime,
                content = data.recentConversation
            )
        }
        if (data.recentMemory != null) {
            OverviewDetailCard(
                icon = Icons.Outlined.AutoAwesome,
                title = "最近记忆",
                content = data.recentMemory
            )
        }
        if (data.nextProactivePlan != null) {
            OverviewDetailCard(
                icon = Icons.Outlined.NotificationsActive,
                title = "下一条主动计划",
                content = data.nextProactivePlan
            )
        }
        Spacer(modifier = Modifier.height(8.dp))
    }
}

@Composable
private fun OverviewHero(data: CharacterOverviewData) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(20.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(72.dp)
                    .clip(CircleShape)
                    .background(data.themeColor.copy(alpha = 0.2f)),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = data.name.take(1),
                    style = MaterialTheme.typography.headlineMedium,
                    color = data.themeColor,
                    fontWeight = FontWeight.Medium
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = data.name,
                    style = MaterialTheme.typography.headlineSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = data.identity,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun OverviewInfoCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    value: String
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = value,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
    }
}

@Composable
private fun OverviewDetailCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String? = null,
    content: String
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp)
                )
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (subtitle != null) {
                    Spacer(modifier = Modifier.weight(1f))
                    Text(
                        text = subtitle,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                }
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = content,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Preview(name = "Overview - Light", showBackground = true)
@Composable
private fun CharacterOverviewLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            OverviewContent(
                data = CharacterOverviewData(
                    name = "艾米",
                    identity = "温柔知性的陪伴助手",
                    avatar = null,
                    currentMood = "愉悦",
                    lifeStatus = "休息中",
                    recentConversation = "今天聊了很多开心的事情",
                    recentConversationTime = "15分钟前",
                    recentMemory = "记住了用户喜欢喝咖啡",
                    nextProactivePlan = "候选时间范围：今晚 20:00-21:30",
                    themeColor = Color(0xFF8FA8A0)
                ),
                onChat = {}
            )
        }
    }
}

@Preview(name = "Overview - Dark", showBackground = true)
@Composable
private fun CharacterOverviewDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            OverviewContent(
                data = CharacterOverviewData(
                    name = "艾米",
                    identity = "温柔知性的陪伴助手",
                    avatar = null,
                    currentMood = "愉悦",
                    lifeStatus = "休息中",
                    recentConversation = null,
                    recentConversationTime = null,
                    recentMemory = null,
                    nextProactivePlan = null,
                    themeColor = Color(0xFF8FA8A0)
                ),
                onChat = {}
            )
        }
    }
}
