package com.amitia.feature.home

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.material.icons.outlined.ChevronRight
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import coil.compose.AsyncImage
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ProactiveMessageDto
import com.amitia.runtime.api.RuntimeState

@Composable
fun HomeScreen(
    onOpenRuntime: () -> Unit,
    onOpenChat: (characterId: String) -> Unit,
    onOpenCharacter: (characterId: String) -> Unit,
    viewModel: HomeViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize()) {
        TopAppBar(
            title = {
                Text(
                    text = "Amitia",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Medium
                )
            },
            actions = {
                IconButton(onClick = viewModel::refresh) {
                    Icon(Icons.Outlined.Refresh, contentDescription = "刷新")
                }
            },
            colors = TopAppBarDefaults.topAppBarColors(
                containerColor = MaterialTheme.colorScheme.background,
                titleContentColor = MaterialTheme.colorScheme.onBackground,
                actionIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
            )
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            item {
                CurrentCharacterCard(
                    state = state,
                    onOpenCharacter = onOpenCharacter,
                    onOpenChat = onOpenChat
                )
            }
            item {
                RuntimeStatusCard(
                    state = state,
                    onOpenRuntime = onOpenRuntime
                )
            }
            if (state.proactiveMessages.isNotEmpty()) {
                item {
                    AmitiaSectionHeader(title = "主动消息")
                }
                items(state.proactiveMessages) { message ->
                    ProactiveMessageRow(message = message, onClick = {
                        message.characterId?.let(onOpenChat)
                    })
                }
            }
            item {
                AmitiaSectionHeader(title = "最近对话")
            }
            items(state.recentConversations) { conversation ->
                ConversationRow(conversation = conversation, onClick = {
                    conversation.characterId?.let(onOpenChat)
                })
            }
            if (state.recentConversations.isEmpty() && state.currentCharacter != null) {
                item {
                    Text(
                        text = "暂无对话，点击角色卡片开始聊天。",
                        style = MaterialTheme.typography.bodySmall,
                        color = AmitiaColors.OnSurfaceMuted,
                        modifier = Modifier.padding(vertical = 24.dp)
                    )
                }
            }
        }
    }
}

@Composable
private fun CurrentCharacterCard(
    state: HomeUiState,
    onOpenCharacter: (String) -> Unit,
    onOpenChat: (String) -> Unit
) {
    val character = state.currentCharacter
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = 0.dp
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            if (character == null) {
                Text(
                    text = "尚未设置当前角色",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = "前往「角色」标签创建或切换。",
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            } else {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(
                        modifier = Modifier
                            .size(56.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        if (!character.avatar.isNullOrBlank()) {
                            AsyncImage(
                                model = character.avatar,
                                contentDescription = character.name,
                                modifier = Modifier.fillMaxSize()
                            )
                        } else {
                            Text(
                                text = character.name.take(1),
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onPrimaryContainer
                            )
                        }
                    }
                    Spacer(modifier = Modifier.size(16.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = character.name,
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = character.description ?: character.personality ?: "未填写身份与性格",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    IconButton(onClick = { onOpenCharacter(character.id) }) {
                        Icon(
                            Icons.Outlined.ChevronRight,
                            contentDescription = "查看详情",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
                Spacer(modifier = Modifier.height(16.dp))
                Surface(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { onOpenChat(character.id) },
                    shape = RoundedCornerShape(12.dp),
                    color = MaterialTheme.colorScheme.primaryContainer
                ) {
                    Row(
                        modifier = Modifier.padding(vertical = 12.dp, horizontal = 16.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "开始对话",
                            style = MaterialTheme.typography.labelLarge,
                            color = MaterialTheme.colorScheme.onPrimaryContainer
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun RuntimeStatusCard(
    state: HomeUiState,
    onOpenRuntime: () -> Unit
) {
    val rs = state.runtimeState
    val dotColor = when (rs) {
        is RuntimeState.Running -> AmitiaColors.StateRunning
        is RuntimeState.Degraded -> AmitiaColors.StateDegraded
        is RuntimeState.Failed -> AmitiaColors.StateFailed
        is RuntimeState.Installing, is RuntimeState.Starting, is RuntimeState.Updating,
        is RuntimeState.Stopping -> AmitiaColors.StateInstalling
        else -> AmitiaColors.StateIdle
    }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onOpenRuntime),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            AmitiaStatusDot(color = dotColor)
            Spacer(modifier = Modifier.size(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "运行时",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = rs.readableMessage,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Icon(
                imageVector = Icons.Outlined.Settings,
                contentDescription = "管理",
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun ConversationRow(conversation: ConversationDto, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = conversation.title ?: "未命名会话",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = conversation.lastMessageAt ?: "暂无消息",
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
            if (conversation.unreadCount > 0) {
                Surface(
                    shape = CircleShape,
                    color = MaterialTheme.colorScheme.primary
                ) {
                    Text(
                        text = "${conversation.unreadCount}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                    )
                }
            }
        }
    }
}

@Composable
private fun ProactiveMessageRow(message: ProactiveMessageDto, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = message.content,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = message.scheduledAt ?: message.createdAt ?: "",
                style = MaterialTheme.typography.labelSmall,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
    }
}
