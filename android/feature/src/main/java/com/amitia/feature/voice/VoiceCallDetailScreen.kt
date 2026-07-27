package com.amitia.feature.voice

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
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun VoiceCallDetailScreen(
    callId: String,
    onBack: () -> Unit,
    viewModel: VoiceViewModel = hiltViewModel()
) {
    val state by viewModel.detailState.collectAsStateWithLifecycle()
    LaunchedEffect(callId) { viewModel.loadDetail(callId) }
    VoiceCallDetailContent(state = state, callId = callId, onBack = onBack)
}

@Composable
fun VoiceCallDetailContent(
    state: ScreenState<VoiceCallDetailUiState>,
    callId: String,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "通话详情",
            onBack = onBack,
            actions = {
                com.amitia.core.designsystem.component.AmitiaIconButton(
                    icon = AmitiaIcons.Download,
                    contentDescription = "导出",
                    onClick = {}
                )
            }
        )
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载通话详情...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = state.error.title,
                    description = state.error.message,
                    onRetry = onBack,
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                com.amitia.core.designsystem.component.AmitiaEmptyState(
                    icon = AmitiaIcons.PhoneOff,
                    title = "未找到通话",
                    description = "该通话记录可能已被删除",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> DetailBody(data = state.data)
            is ScreenState.Partial -> DetailBody(data = state.data)
        }
    }
}

@Composable
private fun DetailBody(data: VoiceCallDetailUiState) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        DetailHeader(data = data)
        AmitiaSection(title = "通话摘要") {
            DetailCard {
                Text(
                    text = data.summary,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
        AmitiaSection(title = "字幕记录") {
            DetailCard {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    data.captions.forEach { caption ->
                        Row(verticalAlignment = Alignment.Top) {
                            Text(
                                text = "${caption.speaker}:",
                                style = MaterialTheme.typography.labelMedium,
                                color = if (caption.isUser) MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.padding(end = AmitiaSpacing.Xs)
                            )
                            Text(
                                text = caption.text,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                    }
                }
            }
        }
        AmitiaSection(title = "关键记忆") {
            DetailCard {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                    data.keyMemories.forEach { memory ->
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(
                                imageVector = AmitiaIcons.Memory,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.size(AmitiaIconSize.Small)
                            )
                            Spacer(modifier = Modifier.size(AmitiaSpacing.Xs))
                            Text(
                                text = memory,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                    }
                }
            }
        }
        AmitiaSection(title = "音频诊断") {
            DetailCard {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        imageVector = AmitiaIcons.GraphicEq,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                    Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                    Text(
                        text = data.audioDiagnostics,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
        if (data.hasRecording) {
            SecondaryButton(
                text = "回放录音",
                onClick = {},
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.PlayArrow
            )
        }
        DangerButton(
            text = "删除通话记录",
            onClick = {},
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Delete
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
    }
}

@Composable
private fun DetailHeader(data: VoiceCallDetailUiState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            androidx.compose.foundation.layout.Box(
                modifier = Modifier.size(48.dp),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Phone,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(AmitiaIconSize.Large)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = data.characterName,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${data.time} · ${data.duration}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun DetailCard(content: @Composable () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            content()
        }
    }
}

@Preview(name = "Voice Detail - Light", showBackground = true)
@Composable
private fun VoiceCallDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceCallDetailContent(
            state = ScreenState.Content(
                VoiceCallDetailUiState(
                    callId = "1",
                    characterName = "艾米",
                    time = "今天 14:30",
                    duration = "05:23",
                    summary = "本次通话围绕今日工作安排展开。",
                    captions = listOf(
                        VoiceCaptionItem("1", "艾米", "你好", "14:30:01", isUser = false)
                    ),
                    keyMemories = listOf("用户下午有会议"),
                    audioDiagnostics = "平均延迟 180ms",
                    hasRecording = true
                )
            ),
            callId = "1",
            onBack = {}
        )
    }
}

@Preview(name = "Voice Detail - Dark", showBackground = true)
@Composable
private fun VoiceCallDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceCallDetailContent(
            state = ScreenState.Loading,
            callId = "1",
            onBack = {}
        )
    }
}
