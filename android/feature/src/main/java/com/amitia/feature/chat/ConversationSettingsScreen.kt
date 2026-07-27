package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.SettingsRow

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConversationSettingsScreen(
    conversationId: String,
    onBack: () -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val settings by viewModel.settings.collectAsStateWithLifecycle()
    var showMemoryMenu by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "对话设置", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            item(key = "channel_section") {
                AmitiaSection(title = "渠道") {
                    AmitiaContentSurface {
                        SettingsRow(
                            title = "当前渠道",
                            subtitle = settings.channel,
                            leadingIcon = AmitiaIcons.Chat
                        )
                    }
                }
            }
            item(key = "model_section") {
                AmitiaSection(title = "模型路由") {
                    AmitiaContentSurface {
                        SettingsRow(
                            title = "模型路由",
                            subtitle = settings.modelRoute,
                            leadingIcon = AmitiaIcons.SmartToy,
                            onClick = {}
                        )
                    }
                }
            }
            item(key = "voice_section") {
                AmitiaSection(title = "语音设置") {
                    AmitiaContentSurface {
                        Column {
                            SettingsToggleRow(
                                title = "自动播放语音",
                                subtitle = "收到角色回复后自动播放语音",
                                icon = AmitiaIcons.VolumeUp,
                                checked = settings.autoPlayVoice,
                                onToggle = {
                                    viewModel.updateSetting { it.copy(autoPlayVoice = !it.autoPlayVoice) }
                                }
                            )
                        }
                    }
                }
            }
            item(key = "message_section") {
                AmitiaSection(title = "消息设置") {
                    AmitiaContentSurface {
                        Column {
                            SettingsToggleRow(
                                title = "合并连续消息",
                                subtitle = "短时间内连续发送的消息将合并为一条",
                                icon = AmitiaIcons.Layers,
                                checked = settings.mergeConsecutiveMessages,
                                onToggle = {
                                    viewModel.updateSetting { it.copy(mergeConsecutiveMessages = !it.mergeConsecutiveMessages) }
                                }
                            )
                        }
                    }
                }
            }
            item(key = "memory_section") {
                AmitiaSection(title = "记忆写入") {
                    AmitiaContentSurface {
                        Box {
                            SettingsRow(
                                title = "写入策略",
                                subtitle = settings.memoryWriteStrategy.label,
                                leadingIcon = AmitiaIcons.Memory,
                                onClick = { showMemoryMenu = true }
                            )
                            DropdownMenu(
                                expanded = showMemoryMenu,
                                onDismissRequest = { showMemoryMenu = false }
                            ) {
                                MemoryWriteStrategy.entries.forEach { strategy ->
                                    DropdownMenuItem(
                                        text = { Text(text = strategy.label) },
                                        onClick = {
                                            viewModel.updateSetting { it.copy(memoryWriteStrategy = strategy) }
                                            showMemoryMenu = false
                                        }
                                    )
                                }
                            }
                        }
                    }
                }
            }
            item(key = "notification_section") {
                AmitiaSection(title = "通知设置") {
                    AmitiaContentSurface {
                        SettingsToggleRow(
                            title = "消息通知",
                            subtitle = "收到新消息时发送通知",
                            icon = AmitiaIcons.Notifications,
                            checked = settings.notificationsEnabled,
                            onToggle = {
                                viewModel.updateSetting { it.copy(notificationsEnabled = !it.notificationsEnabled) }
                            }
                        )
                    }
                }
            }
            item(key = "danger_section") {
                AmitiaSection(title = "危险操作") {
                    DangerButton(
                        text = "清空对话记录",
                        onClick = {},
                        modifier = Modifier.fillMaxWidth(),
                        leadingIcon = AmitiaIcons.DeleteForever
                    )
                }
            }
        }
    }
}

@Composable
private fun SettingsToggleRow(
    title: String,
    subtitle: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    checked: Boolean,
    onToggle: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            modifier = Modifier.size(AmitiaIconSize.Large).clip(CircleShape)
                .background(MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Spacer(modifier = Modifier.size(AmitiaSpacing.Base))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
        Switch(
            checked = checked,
            onCheckedChange = { onToggle() },
            colors = SwitchDefaults.colors(
                checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                checkedTrackColor = MaterialTheme.colorScheme.primary
            )
        )
    }
}

@Preview(name = "Conversation Settings - Light", showBackground = true)
@Composable
private fun ConversationSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                SettingsToggleRow(
                    title = "自动播放语音",
                    subtitle = "收到角色回复后自动播放语音",
                    icon = AmitiaIcons.VolumeUp,
                    checked = true,
                    onToggle = {}
                )
            }
        }
    }
}

@Preview(name = "Conversation Settings - Dark", showBackground = true)
@Composable
private fun ConversationSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            SettingsToggleRow(
                title = "合并连续消息",
                subtitle = "短时间内连续发送的消息将合并为一条",
                icon = AmitiaIcons.Layers,
                checked = false,
                onToggle = {}
            )
        }
    }
}
