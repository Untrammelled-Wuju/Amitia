package com.amitia.feature.voicecenter

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun VoiceCenterHomeScreen(
    onBack: () -> Unit,
    onTtsSettings: () -> Unit,
    onSttSettings: () -> Unit,
    onVoiceLibrary: () -> Unit,
    onVoiceClone: () -> Unit,
    onRealtimeVoice: () -> Unit,
    onAudioDiagnostics: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val voicesState by viewModel.voicesState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadVoices() }
    VoiceCenterHomeContent(
        state = voicesState,
        onBack = onBack,
        onTtsSettings = onTtsSettings,
        onSttSettings = onSttSettings,
        onVoiceLibrary = onVoiceLibrary,
        onVoiceClone = onVoiceClone,
        onRealtimeVoice = onRealtimeVoice,
        onAudioDiagnostics = onAudioDiagnostics
    )
}

@Composable
fun VoiceCenterHomeContent(
    state: ScreenState<List<VoiceItemUiModel>>,
    onBack: () -> Unit,
    onTtsSettings: () -> Unit,
    onSttSettings: () -> Unit,
    onVoiceLibrary: () -> Unit,
    onVoiceClone: () -> Unit,
    onRealtimeVoice: () -> Unit,
    onAudioDiagnostics: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "语音中心", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "hero") { VoiceHeroCard(state = state) }
            item(key = "tts_stt_section") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    VoiceEntryRow(
                        icon = AmitiaIcons.GraphicEq,
                        iconColor = AmitiaStateColors.Running,
                        title = "TTS 设置",
                        subtitle = "语音合成参数配置",
                        onClick = onTtsSettings
                    )
                    VoiceEntryRow(
                        icon = AmitiaIcons.Mic,
                        iconColor = AmitiaStateColors.Running,
                        title = "STT 设置",
                        subtitle = "语音识别参数配置",
                        onClick = onSttSettings
                    )
                }
            }
            item(key = "voice_section_header") {
                AmitiaSectionHeader(title = "声音管理")
            }
            item(key = "voice_entries") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    VoiceEntryRow(
                        icon = AmitiaIcons.MusicNote,
                        iconColor = AmitiaStateColors.Degraded,
                        title = "声音库",
                        subtitle = "浏览和试听所有声音",
                        onClick = onVoiceLibrary
                    )
                    VoiceEntryRow(
                        icon = AmitiaIcons.PersonAdd,
                        iconColor = AmitiaStateColors.Degraded,
                        title = "声音复刻",
                        subtitle = "克隆自定义声音",
                        onClick = onVoiceClone
                    )
                }
            }
            item(key = "tools_section_header") {
                AmitiaSectionHeader(title = "工具与诊断")
            }
            item(key = "tool_entries") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    VoiceEntryRow(
                        icon = AmitiaIcons.Phone,
                        iconColor = AmitiaStateColors.Running,
                        title = "实时语音",
                        subtitle = "实时语音对话",
                        onClick = onRealtimeVoice
                    )
                    VoiceEntryRow(
                        icon = AmitiaIcons.BugReport,
                        iconColor = AmitiaStateColors.Degraded,
                        title = "音频诊断",
                        subtitle = "检测音频设备和连接",
                        onClick = onAudioDiagnostics
                    )
                }
            }
            item(key = "recent_voices_header") {
                AmitiaSectionHeader(title = "最近使用的声音")
            }
            when (state) {
                is ScreenState.Loading -> {
                    item(key = "loading") {
                        Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                            InlineLoading(message = "加载声音...")
                        }
                    }
                }
                is ScreenState.Error -> {
                    item(key = "error") {
                        AmitiaErrorState(
                            icon = AmitiaIcons.CloudOff,
                            title = "加载失败",
                            description = state.error.message,
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Empty -> {
                    item(key = "empty") {
                        AmitiaEmptyState(
                            icon = AmitiaIcons.MicNone,
                            title = "暂无声音",
                            description = "在声音库中添加或试听声音",
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
                is ScreenState.Content -> {
                    val recent = state.data.take(3)
                    items(recent.size) { index ->
                        val voice = recent[index]
                        RecentVoiceItem(voice = voice)
                    }
                }
                is ScreenState.Partial -> {}
            }
        }
    }
}

@Composable
private fun VoiceHeroCard(state: ScreenState<List<VoiceItemUiModel>>) {
    val voiceCount = (state as? ScreenState.Content)?.data?.size ?: 0
    val favCount = (state as? ScreenState.Content)?.data?.count { it.isFavorite } ?: 0
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Lg),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(48.dp).clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.VolumeUp,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "语音能力总览",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "$voiceCount 个声音可用 · $favCount 个收藏",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun VoiceEntryRow(
    icon: ImageVector,
    iconColor: androidx.compose.ui.graphics.Color,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier.fillMaxWidth().clickable(
            interactionSource = interactionSource,
            indication = null,
            role = Role.Button,
            onClick = onClick
        ),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(40.dp).clip(CircleShape).background(iconColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = iconColor,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

@Composable
private fun RecentVoiceItem(voice: VoiceItemUiModel) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.PlayArrow,
                    contentDescription = "试听",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = voice.name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${voice.provider} · ${voice.language}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            if (voice.isFavorite) {
                Icon(
                    imageVector = AmitiaIcons.Star,
                    contentDescription = null,
                    tint = AmitiaStateColors.Degraded,
                    modifier = Modifier.size(18.dp)
                )
            }
        }
    }
}

@Preview(name = "VoiceCenterHome - Light", showBackground = true)
@Composable
private fun VoiceCenterHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceCenterHomeContent(
            state = ScreenState.Content(
                listOf(
                    VoiceItemUiModel("1", "标准女声", "Azure", "zh-CN", "女", "温柔", isFavorite = true),
                    VoiceItemUiModel("2", "标准男声", "Azure", "zh-CN", "男", "沉稳")
                )
            ),
            onBack = {},
            onTtsSettings = {},
            onSttSettings = {},
            onVoiceLibrary = {},
            onVoiceClone = {},
            onRealtimeVoice = {},
            onAudioDiagnostics = {}
        )
    }
}

@Preview(name = "VoiceCenterHome - Dark", showBackground = true)
@Composable
private fun VoiceCenterHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceCenterHomeContent(
            state = ScreenState.Empty(),
            onBack = {},
            onTtsSettings = {},
            onSttSettings = {},
            onVoiceLibrary = {},
            onVoiceClone = {},
            onRealtimeVoice = {},
            onAudioDiagnostics = {}
        )
    }
}
