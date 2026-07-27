package com.amitia.feature.onboarding.steps

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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun RuntimeInstallStepPage(
    state: OnboardingFlowUiState,
    onStart: () -> Unit,
    onPause: () -> Unit,
    onRetry: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            StepTitle(text = "本地运行安装")
            StepDescription(text = "正在准备本地运行时组件，请保持网络连接。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            if (state.runtimeItems.isEmpty()) {
                PrimaryButton(
                    text = "开始安装",
                    onClick = onStart,
                    modifier = Modifier.fillMaxWidth()
                )
                return@Column
            }
            state.runtimeItems.forEach { item ->
                RuntimeInstallRow(item = item)
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            val allDone = state.runtimeItems.isNotEmpty() && state.runtimeItems.all { it.status == InstallStatus.Done }
            val anyFailed = state.runtimeItems.any { it.status == InstallStatus.Failed }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (state.runtimeInstalling) {
                    SecondaryButton(
                        text = "暂停",
                        onClick = onPause,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Pause
                    )
                } else if (anyFailed) {
                    SecondaryButton(
                        text = "重试",
                        onClick = onRetry,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Refresh
                    )
                }
                if (allDone) {
                    PrimaryButton(
                        text = "下一步",
                        onClick = onNext,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.ArrowForward
                    )
                } else if (!state.runtimeInstalling && !anyFailed) {
                    PrimaryButton(
                        text = "开始安装",
                        onClick = onStart,
                        modifier = Modifier.weight(1f)
                    )
                }
            }
            if (state.runtimeInstalling) {
                val progress = state.runtimeItems.count { it.status == InstallStatus.Done }.toFloat() /
                    state.runtimeItems.size.coerceAtLeast(1)
                LinearProgressIndicator(
                    progress = { progress },
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(4.dp),
                    color = MaterialTheme.colorScheme.primary,
                    trackColor = MaterialTheme.colorScheme.surfaceVariant
                )
            }
        }
    }
}

@Composable
private fun RuntimeInstallRow(item: RuntimeInstallItem) {
    val (statusText, statusColor) = when (item.status) {
        InstallStatus.Pending -> "等待中" to MaterialTheme.colorScheme.onSurfaceVariant
        InstallStatus.Downloading -> "下载中" to AmitiaStateColors.Pending
        InstallStatus.Verifying -> "校验中" to AmitiaStateColors.Pending
        InstallStatus.Installing -> "安装中" to AmitiaStateColors.Installing
        InstallStatus.Starting -> "启动中" to AmitiaStateColors.Installing
        InstallStatus.Done -> "已完成" to AmitiaStateColors.Running
        InstallStatus.Failed -> "失败" to AmitiaStateColors.Failed
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(32.dp)
                    .clip(CircleShape)
                    .background(statusColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                if (item.status == InstallStatus.Done) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = statusColor,
                        modifier = Modifier.size(18.dp)
                    )
                } else if (item.status == InstallStatus.Failed) {
                    Icon(
                        imageVector = AmitiaIcons.Close,
                        contentDescription = null,
                        tint = statusColor,
                        modifier = Modifier.size(18.dp)
                    )
                } else if (item.status != InstallStatus.Pending) {
                    Icon(
                        imageVector = AmitiaIcons.Download,
                        contentDescription = null,
                        tint = statusColor,
                        modifier = Modifier.size(18.dp)
                    )
                }
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = statusText,
                    style = MaterialTheme.typography.labelSmall,
                    color = statusColor
                )
            }
        }
    }
}

@Preview(name = "RuntimeInstall - Light", showBackground = true)
@Composable
private fun RuntimeInstallStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RuntimeInstallStepPage(
            state = OnboardingFlowUiState(
                runtimeInstalling = true,
                runtimeItems = listOf(
                    RuntimeInstallItem("内嵌 Linux 环境", InstallStatus.Done),
                    RuntimeInstallItem("Amitia Go Backend", InstallStatus.Downloading),
                    RuntimeInstallItem("Qdrant", InstallStatus.Pending),
                    RuntimeInstallItem("SurrealDB", InstallStatus.Pending),
                    RuntimeInstallItem("SQLite 数据目录", InstallStatus.Pending)
                )
            ),
            onStart = {},
            onPause = {},
            onRetry = {},
            onNext = {}
        )
    }
}

@Preview(name = "RuntimeInstall - Dark", showBackground = true)
@Composable
private fun RuntimeInstallStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RuntimeInstallStepPage(
            state = OnboardingFlowUiState(
                runtimeInstalling = false,
                runtimeItems = listOf(
                    RuntimeInstallItem("内嵌 Linux 环境", InstallStatus.Done),
                    RuntimeInstallItem("Amitia Go Backend", InstallStatus.Done),
                    RuntimeInstallItem("Qdrant", InstallStatus.Failed)
                )
            ),
            onStart = {},
            onPause = {},
            onRetry = {},
            onNext = {}
        )
    }
}
