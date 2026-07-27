package com.amitia.feature.computeruse

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.AmitiaDangerDialog
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun RunningSessionScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val session by viewModel.currentSession.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    RunningSessionContent(
        session = session,
        loading = loading,
        onBack = onBack,
        onPause = viewModel::pauseSession,
        onResume = viewModel::resumeSession,
        onStop = viewModel::stopSession,
        onTakeover = viewModel::takeoverSession
    )
}

@Composable
fun RunningSessionContent(
    session: ComputerUseSession?,
    loading: Boolean,
    onBack: () -> Unit,
    onPause: () -> Unit,
    onResume: () -> Unit,
    onStop: () -> Unit,
    onTakeover: () -> Unit
) {
    var pendingStop by remember { mutableStateOf(false) }
    var pendingTakeover by remember { mutableStateOf(false) }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "运行会话", onBack = onBack)
        if (loading) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(
                    text = "加载会话...",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        } else if (session == null) {
            AmitiaEmptyState(
                icon = AmitiaIcons.PlayArrow,
                title = "暂无运行会话",
                description = "启动 Computer Use 后会话将显示在这里",
                modifier = Modifier.fillMaxSize()
            )
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "status") { SessionStatusCard(session = session) }
                if (session.status == SessionStatus.Paused) {
                    item(key = "paused_warn") {
                        WarningBanner(message = "会话已暂停，AI 操作处于挂起状态")
                    }
                }
                item(key = "screen") { ScreenSummaryCard(session = session) }
                item(key = "steps_header") { AmitiaSectionHeader(title = "操作步骤") }
                items(session.steps, key = { it.id }) { step ->
                    SessionStepRow(step = step)
                }
                item(key = "controls_header") { AmitiaSectionHeader(title = "会话控制") }
                item(key = "controls") {
                    SessionControls(
                        status = session.status,
                        onPause = onPause,
                        onResume = onResume,
                        onStop = { pendingStop = true },
                        onTakeover = { pendingTakeover = true }
                    )
                }
            }
        }
    }
    if (pendingStop) {
        AmitiaDangerDialog(
            onDismiss = { pendingStop = false },
            onConfirm = {
                onStop()
                pendingStop = false
            },
            title = "停止会话",
            message = "即将停止当前 Computer Use 会话",
            impactDescription = "停止后正在执行的步骤将被中断，未完成的操作不会继续",
            confirmText = "停止会话",
            dangerLevel = com.amitia.core.designsystem.DangerLevel.Two
        )
    }
    if (pendingTakeover) {
        AmitiaConfirmDialog(
            onDismiss = { pendingTakeover = false },
            onConfirm = {
                onTakeover()
                pendingTakeover = false
            },
            title = "接管会话",
            message = "接管后 AI 将停止操作，由你直接控制设备",
            confirmText = "确认接管",
            destructive = true
        )
    }
}

@Composable
private fun SessionStatusCard(session: ComputerUseSession) {
    val accentColor = when (session.status) {
        SessionStatus.Running -> MaterialTheme.colorScheme.tertiary
        SessionStatus.Paused -> MaterialTheme.colorScheme.secondary
        SessionStatus.Stopped -> MaterialTheme.colorScheme.error
        SessionStatus.Pending -> MaterialTheme.colorScheme.primary
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Box(
                    modifier = Modifier.size(40.dp).clip(CircleShape).background(accentColor.copy(alpha = 0.15f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.PlayArrow,
                        contentDescription = null,
                        tint = accentColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "当前目标",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = session.target,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 2, overflow = TextOverflow.Ellipsis
                    )
                }
                Surface(shape = MaterialTheme.shapes.small, color = accentColor.copy(alpha = 0.2f)) {
                    Text(
                        text = session.status.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = accentColor,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            Text(
                text = "当前步骤：${session.currentStep}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = "开始于 ${session.startedAt}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
        }
    }
}

@Composable
private fun ScreenSummaryCard(session: ComputerUseSession) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Visibility,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "当前屏幕摘要",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = session.screenSummary,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 4, overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun SessionStepRow(step: SessionStep) {
    val (iconColor, icon) = when (step.status) {
        StepStatus.Done -> MaterialTheme.colorScheme.tertiary to AmitiaIcons.CheckCircle
        StepStatus.Running -> MaterialTheme.colorScheme.primary to AmitiaIcons.PlayArrow
        StepStatus.Failed -> MaterialTheme.colorScheme.error to AmitiaIcons.Error
        StepStatus.Pending -> MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f) to AmitiaIcons.RadioButtonUnchecked
    }
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = AmitiaSpacing.Xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier.size(AmitiaIconSize.Medium).clip(CircleShape).background(iconColor.copy(alpha = 0.15f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(imageVector = icon, contentDescription = null, tint = iconColor, modifier = Modifier.size(AmitiaIconSize.Small))
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = step.description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 2, overflow = TextOverflow.Ellipsis
            )
            Text(
                text = step.status.label + if (step.timestamp.isNotEmpty()) " · ${step.timestamp}" else "",
                style = MaterialTheme.typography.labelSmall,
                color = iconColor
            )
        }
    }
}

@Composable
private fun SessionControls(
    status: SessionStatus,
    onPause: () -> Unit,
    onResume: () -> Unit,
    onStop: () -> Unit,
    onTakeover: () -> Unit
) {
    val canControl = status == SessionStatus.Running || status == SessionStatus.Paused
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
        if (status == SessionStatus.Paused) {
            PrimaryButton(
                text = "继续执行",
                onClick = onResume,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.PlayArrow
            )
        } else {
            SecondaryButton(
                text = "暂停",
                onClick = onPause,
                modifier = Modifier.fillMaxWidth(),
                enabled = status == SessionStatus.Running,
                leadingIcon = AmitiaIcons.Pause
            )
        }
        DangerButton(
            text = "停止会话",
            onClick = onStop,
            modifier = Modifier.fillMaxWidth(),
            enabled = canControl,
            leadingIcon = AmitiaIcons.Stop
        )
        SecondaryButton(
            text = "接管控制",
            onClick = onTakeover,
            modifier = Modifier.fillMaxWidth(),
            enabled = canControl,
            leadingIcon = AmitiaIcons.TouchApp
        )
    }
}

@Preview(name = "Running Session - Light", showBackground = true)
@Composable
private fun RunningSessionLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RunningSessionContent(
            session = ComputerUseSession(
                "s1", "整理桌面文件", SessionStatus.Running, "正在分类文件",
                "桌面显示 12 个待整理文件夹",
                listOf(
                    SessionStep("1", "扫描桌面", StepStatus.Done, "14:30"),
                    SessionStep("2", "分类文件", StepStatus.Running, "14:31"),
                    SessionStep("3", "移动到对应目录", StepStatus.Pending, "")
                ),
                "14:30"
            ),
            loading = false, onBack = {}, onPause = {}, onResume = {}, onStop = {}, onTakeover = {}
        )
    }
}

@Preview(name = "Running Session - Dark", showBackground = true)
@Composable
private fun RunningSessionDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RunningSessionContent(
            session = null, loading = true, onBack = {},
            onPause = {}, onResume = {}, onStop = {}, onTakeover = {}
        )
    }
}
