package com.amitia.feature.settings.crashrecovery

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
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.background
import androidx.compose.ui.draw.clip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.settings.CrashRecoveryState
import com.amitia.feature.settings.CrashReport
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun CrashRecoveryScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val crashRecovery = state.crashRecovery

    CrashRecoveryScreenContent(
        state = crashRecovery,
        onBack = onBack,
        onAutoSubmitChange = { newValue ->
            val updated = crashRecovery.copy(autoSubmit = newValue)
            viewModel.updateCrashRecovery(updated)
        }
    )
}

@Composable
private fun CrashRecoveryScreenContent(
    state: CrashRecoveryState,
    onBack: () -> Unit,
    onAutoSubmitChange: (Boolean) -> Unit
) {
    var selectedReport by remember { mutableStateOf<CrashReport?>(null) }
    var showClearDialog by remember { mutableStateOf(false) }

    AmitiaPageScaffold(
        topBar = {
            AmitiaTopBar(
                title = "崩溃恢复与报告",
                onBack = onBack,
                actions = {
                    if (state.crashes.isNotEmpty()) {
                        androidx.compose.material3.IconButton(onClick = { showClearDialog = true }) {
                            Icon(
                                imageVector = AmitiaIcons.DeleteOutlined,
                                contentDescription = "清除记录"
                            )
                        }
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))

            AmitiaSection(title = "自动提交") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    AmitiaSwitchRow(
                        title = "自动提交崩溃报告",
                        subtitle = "崩溃后自动上传脱敏报告，帮助改进应用",
                        checked = state.autoSubmit,
                        onCheckedChange = onAutoSubmitChange,
                        leadingIcon = AmitiaIcons.BugReport
                    )
                }
            }

            if (state.crashes.isEmpty()) {
                AmitiaEmptyState(
                    icon = AmitiaIcons.CheckCircle,
                    title = "暂无崩溃记录",
                    description = "应用运行稳定，没有崩溃记录",
                    modifier = Modifier.padding(top = AmitiaSpacing.Xxl)
                )
            } else {
                AmitiaSection(title = "崩溃记录", subtitle = "${state.crashes.size} 条记录") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column {
                            state.crashes.forEachIndexed { index, crash ->
                                CrashReportItem(
                                    crash = crash,
                                    onClick = { selectedReport = crash }
                                )
                                if (index < state.crashes.lastIndex) {
                                    AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                                }
                            }
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }

    selectedReport?.let { report ->
        CrashReportDetailDialog(
            report = report,
            onDismiss = { selectedReport = null }
        )
    }

    if (showClearDialog) {
        AmitiaConfirmDialog(
            onDismiss = { showClearDialog = false },
            onConfirm = {
                showClearDialog = false
            },
            title = "清除崩溃记录",
            message = "将清除所有崩溃记录，此操作不可恢复。",
            confirmText = "清除",
            destructive = true
        )
    }
}

@Composable
private fun CrashReportItem(
    crash: CrashReport,
    onClick: () -> Unit
) {
    val statusColor = if (crash.safeBoot) {
        AmitiaStateColors.Running
    } else {
        AmitiaStateColors.Failed
    }
    val statusText = if (crash.safeBoot) "安全启动成功" else "安全启动失败"

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Box(
            modifier = Modifier
                .size(AmitiaIconSize.Large)
                .clip(CircleShape)
                .background(statusColor.copy(alpha = 0.15f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = if (crash.safeBoot) AmitiaIcons.CheckCircle else AmitiaIcons.Error,
                contentDescription = null,
                tint = statusColor,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = crash.module,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = crash.time,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = statusText,
                style = MaterialTheme.typography.labelSmall,
                color = statusColor
            )
        }
        Surface(
            shape = CircleShape,
            color = MaterialTheme.colorScheme.surfaceVariant
        ) {
            Text(
                text = crash.reportId,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
            )
        }
    }
}

@Composable
private fun CrashReportDetailDialog(
    report: CrashReport,
    onDismiss: () -> Unit
) {
    com.amitia.core.designsystem.component.AmitiaDialog(
        onDismiss = onDismiss,
        title = "崩溃报告详情",
        icon = AmitiaIcons.BugReport,
        confirmText = "关闭",
        onConfirm = onDismiss,
        dismissText = null,
        content = {
            Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                DetailRow(label = "报告编号", value = report.reportId)
                DetailRow(label = "崩溃时间", value = report.time)
                DetailRow(label = "影响模块", value = report.module)
                DetailRow(
                    label = "安全启动",
                    value = if (report.safeBoot) "成功" else "失败"
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                Surface(
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = "报告已脱敏处理，不包含任何个人隐私数据。包含调用栈、模块状态和系统信息。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
                Row(
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    SecondaryButton(
                        text = "查看脱敏报告",
                        onClick = {},
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Description
                    )
                    PrimaryButton(
                        text = "提交报告",
                        onClick = {},
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Upload
                    )
                }
            }
        }
    )
}

@Composable
private fun DetailRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Preview(name = "崩溃恢复页 - 亮色", showBackground = true)
@Composable
private fun CrashRecoveryScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CrashRecoveryScreenContent(
            state = CrashRecoveryState(
                crashes = listOf(
                    CrashReport("2026-07-25 14:30", "Runtime", true, "CR-001"),
                    CrashReport("2026-07-20 09:15", "Channel", false, "CR-002")
                ),
                autoSubmit = false
            ),
            onBack = {},
            onAutoSubmitChange = {}
        )
    }
}

@Preview(name = "崩溃恢复页 - 暗色", showBackground = true)
@Composable
private fun CrashRecoveryScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CrashRecoveryScreenContent(
            state = CrashRecoveryState(
                crashes = emptyList(),
                autoSubmit = true
            ),
            onBack = {},
            onAutoSubmitChange = {}
        )
    }
}
