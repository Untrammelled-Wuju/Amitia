package com.amitia.feature.channel

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.widget.Toast
import androidx.compose.foundation.background
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
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
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
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun ChannelDiagnosticScreen(
    onBack: () -> Unit,
    viewModel: ChannelDiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ChannelDiagnosticContent(
        state = state,
        onBack = onBack,
        onRetry = viewModel::load
    )
}

@Composable
fun ChannelDiagnosticContent(
    state: ScreenState<ChannelDiagnosticData>,
    onBack: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "渠道诊断", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.BugReport,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Tune,
                title = "暂无诊断数据",
                description = "请选择一个渠道后重新诊断",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> DiagnosticBody(data = state.data)
            is ScreenState.Partial -> DiagnosticBody(data = state.data)
        }
    }
}

@Composable
private fun DiagnosticBody(data: ChannelDiagnosticData) {
    val context = LocalContext.current
    val allPassed = data.items.all { it.passed }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        DiagnosticSummaryCard(channelName = data.channelName, allPassed = allPassed, total = data.items.size, passed = data.items.count { it.passed })
        AmitiaSectionHeader(title = "检查项")
        data.items.forEach { item -> DiagnosticItemRow(item = item) }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
        AmitiaSectionHeader(title = "脱敏诊断信息")
        RawDiagnosticCard(rawText = data.rawText)
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
        LoadingButton(
            text = "一键复制诊断信息",
            onClick = { copyToClipboard(context, data.rawText) },
            loading = false,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.ContentCopy
        )
        SecondaryButton(
            text = "重新诊断",
            onClick = {},
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Refresh
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
    }
}

@Composable
private fun DiagnosticSummaryCard(channelName: String, allPassed: Boolean, total: Int, passed: Int) {
    val summaryColor = if (allPassed) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.errorContainer
    val onSummaryColor = if (allPassed) MaterialTheme.colorScheme.onTertiaryContainer else MaterialTheme.colorScheme.onErrorContainer
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        color = summaryColor
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(CircleShape)
                    .background(onSummaryColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (allPassed) AmitiaIcons.CheckCircle else AmitiaIcons.Warning,
                    contentDescription = null,
                    tint = onSummaryColor,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = channelName,
                    style = MaterialTheme.typography.titleMedium,
                    color = onSummaryColor,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = if (allPassed) "所有检查项通过" else "$passed/$total 项通过",
                    style = MaterialTheme.typography.bodySmall,
                    color = onSummaryColor.copy(alpha = 0.85f)
                )
            }
        }
    }
}

@Composable
private fun DiagnosticItemRow(item: ChannelDiagnosticItem) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(32.dp)
                    .clip(CircleShape)
                    .background(
                        if (item.passed) MaterialTheme.colorScheme.tertiary.copy(alpha = 0.15f)
                        else MaterialTheme.colorScheme.error.copy(alpha = 0.15f)
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (item.passed) AmitiaIcons.Check else AmitiaIcons.Close,
                    contentDescription = null,
                    tint = if (item.passed) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = item.detail,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun RawDiagnosticCard(rawText: String) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.Top
        ) {
            Text(
                text = rawText,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f)
            )
        }
    }
}

private fun copyToClipboard(context: Context, text: String) {
    val clipboard = context.getSystemService(ClipboardManager::class.java)
    clipboard?.setPrimaryClip(ClipData.newPlainText("AmitiaDiagnostic", text))
    Toast.makeText(context, "诊断信息已复制", Toast.LENGTH_SHORT).show()
}

@Preview(name = "ChannelDiagnostic - Light", showBackground = true)
@Composable
private fun ChannelDiagnosticLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelDiagnosticContent(
            state = ScreenState.Content(ChannelMockData.diagnostics),
            onBack = {}, onRetry = {}
        )
    }
}

@Preview(name = "ChannelDiagnostic - Dark", showBackground = true)
@Composable
private fun ChannelDiagnosticDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelDiagnosticContent(
            state = ScreenState.Error(com.amitia.core.designsystem.UiError(title = "诊断失败", message = "无法连接到渠道服务")),
            onBack = {}, onRetry = {}
        )
    }
}
