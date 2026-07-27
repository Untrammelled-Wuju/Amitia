package com.amitia.feature.bootstrap

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun StartupPreparationScreen(
    state: StartupState,
    onRetry: () -> Unit,
    onSwitchRemote: () -> Unit,
    onDiagnostics: () -> Unit,
    onExit: () -> Unit,
    onToggleDetail: () -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        if (state.failed) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                StartupFailureContent(
                    issue = state.issue,
                    onRetry = onRetry,
                    onSwitchRemote = onSwitchRemote,
                    onDiagnostics = onDiagnostics,
                    onExit = onExit
                )
            }
            return@Surface
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            StartupLogo()
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            Text(
                text = currentPhaseLabel(state.currentPhase),
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            LinearProgressIndicator(
                progress = { state.progress.coerceIn(0f, 1f) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(4.dp),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
            if (state.issue != StartupIssue.None) {
                StartupIssueBanner(issue = state.issue)
                Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
            }
            TertiaryButton(
                text = if (state.detailExpanded) "收起详情" else "查看详情",
                onClick = onToggleDetail,
                leadingIcon = if (state.detailExpanded) AmitiaIcons.ExpandLess else AmitiaIcons.ExpandMore
            )
            AnimatedVisibility(
                visible = state.detailExpanded,
                enter = expandVertically() + fadeIn(),
                exit = shrinkVertically() + fadeOut()
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = AmitiaSpacing.Md),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    state.items.forEach { item ->
                        StartupDetailRow(item = item)
                    }
                }
            }
        }
    }
}

@Composable
private fun StartupLogo() {
    Box(
        modifier = Modifier
            .size(88.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.primaryContainer),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = "A",
            style = MaterialTheme.typography.displayMedium,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun StartupDetailRow(item: StartupProgressItem) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(AmitiaCardShape)
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(20.dp)
                .clip(CircleShape)
                .background(if (item.done) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            if (item.done) {
                Icon(
                    imageVector = AmitiaIcons.Check,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        Text(
            text = item.label,
            style = MaterialTheme.typography.bodyMedium,
            color = if (item.done) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun StartupIssueBanner(issue: StartupIssue) {
    val (message, color) = when (issue) {
        StartupIssue.RuntimeSlow -> "本地运行时启动较慢，请耐心等待" to AmitiaStateColors.Degraded
        StartupIssue.ServiceFailed -> "服务启动失败，请重试或切换模式" to AmitiaStateColors.Failed
        StartupIssue.MigratingData -> "正在进行数据迁移，请勿关闭应用" to AmitiaStateColors.Pending
        StartupIssue.StorageLow -> "存储空间不足，请清理后重试" to AmitiaStateColors.Degraded
        StartupIssue.RemoteUnreachable -> "远程服务不可达，请检查网络" to AmitiaStateColors.Failed
        StartupIssue.None -> "" to MaterialTheme.colorScheme.primary
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = color.copy(alpha = 0.12f)
    ) {
        Text(
            text = message,
            style = MaterialTheme.typography.bodySmall,
            color = color,
            modifier = Modifier.padding(AmitiaSpacing.Base),
            textAlign = TextAlign.Center
        )
    }
}

@Composable
private fun StartupFailureContent(
    issue: StartupIssue,
    onRetry: () -> Unit,
    onSwitchRemote: () -> Unit,
    onDiagnostics: () -> Unit,
    onExit: () -> Unit
) {
    AmitiaErrorState(
        icon = AmitiaIcons.Error,
        title = "启动失败",
        description = "运行时启动未能完成，你可以重试、切换到远程模式，或进入诊断查看详情。",
        onRetry = onRetry,
        onDiagnostics = onDiagnostics
    )
    Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Xxl),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        SecondaryButton(
            text = "切换远程",
            onClick = onSwitchRemote,
            modifier = Modifier.weight(1f),
            leadingIcon = AmitiaIcons.CloudOff
        )
        DangerButton(
            text = "退出",
            onClick = onExit,
            modifier = Modifier.weight(1f),
            leadingIcon = AmitiaIcons.PowerSettingsNew
        )
    }
}

private fun currentPhaseLabel(phase: StartupPhase): String = when (phase) {
    StartupPhase.PreparingEnvironment -> "正在准备本地环境"
    StartupPhase.StartingServices -> "正在启动服务"
    StartupPhase.ConnectingServices -> "正在连接服务"
    StartupPhase.RestoringSession -> "正在恢复会话"
}

@Preview(name = "Startup Preparation - Light", showBackground = true)
@Composable
private fun StartupPreparationLightPreview() {
    AmitiaTheme(darkTheme = false) {
        StartupPreparationScreen(
            state = StartupState(
                currentPhase = StartupPhase.StartingServices,
                progress = 0.5f,
                items = listOf(
                    StartupProgressItem(StartupPhase.PreparingEnvironment, "准备本地环境", true),
                    StartupProgressItem(StartupPhase.StartingServices, "启动服务", false),
                    StartupProgressItem(StartupPhase.ConnectingServices, "连接服务", false),
                    StartupProgressItem(StartupPhase.RestoringSession, "恢复会话", false)
                ),
                detailExpanded = true
            ),
            onRetry = {},
            onSwitchRemote = {},
            onDiagnostics = {},
            onExit = {},
            onToggleDetail = {}
        )
    }
}

@Preview(name = "Startup Preparation - Dark", showBackground = true)
@Composable
private fun StartupPreparationDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        StartupPreparationScreen(
            state = StartupState(
                currentPhase = StartupPhase.ConnectingServices,
                progress = 0.75f,
                issue = StartupIssue.RuntimeSlow,
                failed = false
            ),
            onRetry = {},
            onSwitchRemote = {},
            onDiagnostics = {},
            onExit = {},
            onToggleDetail = {}
        )
    }
}
