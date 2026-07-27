package com.amitia.feature.voicecenter

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
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.LoadingButton

@Composable
fun AudioDiagnosticScreen(
    onBack: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val state by viewModel.audioDiagnosticsState.collectAsStateWithLifecycle()
    val testing by viewModel.testing.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadAudioDiagnostics() }
    AudioDiagnosticContent(
        state = state,
        testing = testing,
        onBack = onBack,
        onRetry = viewModel::loadAudioDiagnostics,
        onRerun = { viewModel.loadAudioDiagnostics() }
    )
}

@Composable
fun AudioDiagnosticContent(
    state: ScreenState<List<AudioDiagnosticItemUiModel>>,
    testing: Boolean,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onRerun: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "音频诊断", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在检测音频设备...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.BugReport,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.GraphicEq,
                title = "暂无诊断结果",
                description = "请先配置音频设备后再进行诊断",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> {
                val grouped = state.data.groupBy { it.category }
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    item(key = "summary") { AudioDiagnosticSummary(items = state.data) }
                    item(key = "rerun") {
                        LoadingButton(
                            text = "重新检测",
                            onClick = onRerun,
                            loading = testing,
                            leadingIcon = AmitiaIcons.Refresh,
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                    grouped.forEach { (category, items) ->
                        item(key = "header_${category.name}") {
                            AmitiaSection(title = category.label) {}
                        }
                        items(items, key = { it.id }) { item ->
                            AudioDiagnosticCard(item = item)
                        }
                    }
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun AudioDiagnosticSummary(items: List<AudioDiagnosticItemUiModel>) {
    val passCount = items.count { it.status == AudioDiagnosticStatus.Pass }
    val warnCount = items.count { it.status == AudioDiagnosticStatus.Warning }
    val failCount = items.count { it.status == AudioDiagnosticStatus.Failed }
    val checkingCount = items.count { it.status == AudioDiagnosticStatus.Checking }
    val allHealthy = failCount == 0 && warnCount == 0

    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = if (allHealthy) AmitiaIcons.HealthAndSafety else AmitiaIcons.WarningAmber,
                    contentDescription = null,
                    tint = if (allHealthy) AmitiaStateColors.Running
                    else if (failCount > 0) AmitiaStateColors.Failed
                    else AmitiaStateColors.Degraded,
                    modifier = Modifier.size(24.dp)
                )
                Text(
                    text = "诊断概览",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = if (allHealthy) "音频系统正常" else "$failCount 异常, $warnCount 警告",
                    style = MaterialTheme.typography.bodySmall,
                    color = if (allHealthy) AmitiaStateColors.Running
                    else if (failCount > 0) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.tertiary
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                SummaryChip("正常", passCount, AmitiaStateColors.Running, Modifier.weight(1f))
                SummaryChip("警告", warnCount, AmitiaStateColors.Degraded, Modifier.weight(1f))
                SummaryChip("异常", failCount, AmitiaStateColors.Failed, Modifier.weight(1f))
                if (checkingCount > 0) {
                    SummaryChip("检测中", checkingCount, AmitiaStateColors.Pending, Modifier.weight(1f))
                }
            }
        }
    }
}

@Composable
private fun SummaryChip(label: String, count: Int, color: androidx.compose.ui.graphics.Color, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        color = color.copy(alpha = 0.12f)
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "$count",
                style = MaterialTheme.typography.titleLarge,
                color = color,
                fontWeight = FontWeight.Bold
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun AudioDiagnosticCard(item: AudioDiagnosticItemUiModel) {
    val statusColor = when (item.status) {
        AudioDiagnosticStatus.Pass -> AmitiaStateColors.Running
        AudioDiagnosticStatus.Warning -> AmitiaStateColors.Degraded
        AudioDiagnosticStatus.Failed -> AmitiaStateColors.Failed
        AudioDiagnosticStatus.Checking -> AmitiaStateColors.Pending
    }
    val statusIcon = when (item.status) {
        AudioDiagnosticStatus.Pass -> AmitiaIcons.CheckCircle
        AudioDiagnosticStatus.Warning -> AmitiaIcons.WarningAmber
        AudioDiagnosticStatus.Failed -> AmitiaIcons.Error
        AudioDiagnosticStatus.Checking -> AmitiaIcons.Sync
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape).background(statusColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = statusIcon,
                    contentDescription = null,
                    tint = statusColor,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier.weight(1f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Surface(
                        shape = RoundedCornerShape(8.dp),
                        color = statusColor.copy(alpha = 0.15f)
                    ) {
                        Text(
                            text = item.status.label,
                            style = MaterialTheme.typography.labelSmall,
                            color = statusColor,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                Text(
                    text = item.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                item.detail?.let { detail ->
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Text(
                        text = detail,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                    )
                }
                item.latencyMs?.let { latency ->
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Speed,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                            modifier = Modifier.size(14.dp)
                        )
                        Text(
                            text = "延迟: ${latency}ms",
                            style = MaterialTheme.typography.labelSmall,
                            color = if (latency > 300) AmitiaStateColors.Degraded
                            else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                        )
                    }
                }
            }
        }
    }
}

@Preview(name = "AudioDiagnostic - Light", showBackground = true)
@Composable
private fun AudioDiagnosticLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AudioDiagnosticContent(
            state = ScreenState.Content(
                listOf(
                    AudioDiagnosticItemUiModel(
                        id = "mic",
                        title = "麦克风输入检测",
                        category = AudioDiagnosticCategory.Microphone,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测麦克风是否正常工作",
                        detail = "采样率: 48000Hz, 通道: 单声道"
                    ),
                    AudioDiagnosticItemUiModel(
                        id = "tts_stt",
                        title = "TTS/STT 连接检测",
                        category = AudioDiagnosticCategory.TtsStt,
                        status = AudioDiagnosticStatus.Warning,
                        description = "检测 TTS 和 STT 服务连接状态",
                        detail = "TTS 已连接, STT 延迟较高",
                        latencyMs = 320
                    ),
                    AudioDiagnosticItemUiModel(
                        id = "latency",
                        title = "端到端延迟检测",
                        category = AudioDiagnosticCategory.Latency,
                        status = AudioDiagnosticStatus.Pass,
                        description = "检测语音端到端延迟",
                        latencyMs = 180
                    )
                )
            ),
            testing = false,
            onBack = {},
            onRetry = {},
            onRerun = {}
        )
    }
}

@Preview(name = "AudioDiagnostic - Dark", showBackground = true)
@Composable
private fun AudioDiagnosticDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AudioDiagnosticContent(
            state = ScreenState.Loading,
            testing = false,
            onBack = {},
            onRetry = {},
            onRerun = {}
        )
    }
}
