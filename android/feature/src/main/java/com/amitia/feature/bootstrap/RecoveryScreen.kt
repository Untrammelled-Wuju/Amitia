package com.amitia.feature.bootstrap

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
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun RecoveryScreen(
    state: RecoveryState,
    onSafeBoot: () -> Unit,
    onNormalBoot: () -> Unit,
    onViewLogs: () -> Unit,
    onRestoreBackup: () -> Unit,
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
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            RecoveryHeader()
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            CrashSummaryCard(state = state)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            PrimaryButton(
                text = "安全启动",
                onClick = onSafeBoot,
                modifier = Modifier.fillMaxWidth(),
                enabled = !state.restoring && state.safeModeAvailable,
                leadingIcon = AmitiaIcons.Shield
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            SecondaryButton(
                text = "正常启动",
                onClick = onNormalBoot,
                modifier = Modifier.fillMaxWidth(),
                enabled = !state.restoring,
                leadingIcon = AmitiaIcons.RestartAlt
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                TertiaryButton(
                    text = "查看日志",
                    onClick = onViewLogs,
                    leadingIcon = AmitiaIcons.BugReport
                )
                TertiaryButton(
                    text = if (state.restoring) "恢复中" else "恢复备份",
                    onClick = onRestoreBackup,
                    leadingIcon = AmitiaIcons.Backup
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            Text(
                text = "不会自动清除你的数据",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun RecoveryHeader() {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(64.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.WarningAmber,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.error,
                modifier = Modifier.size(32.dp)
            )
        }
        Text(
            text = "启动恢复",
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun CrashSummaryCard(state: RecoveryState) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(CircleShape)
                        .background(AmitiaStateColors.Failed)
                )
                Text(
                    text = "上次异常退出",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
            Text(
                text = if (state.crashReason.isNotBlank()) state.crashReason else "检测到上次会话未能正常结束",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            if (state.crashTime.isNotBlank()) {
                Text(
                    text = "时间：${state.crashTime}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
        }
    }
}

@Preview(name = "Recovery - Light", showBackground = true)
@Composable
private fun RecoveryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RecoveryScreen(
            state = RecoveryState(
                crashReason = "运行时进程意外终止 (SIGKILL)",
                crashTime = "今天 14:32"
            ),
            onSafeBoot = {},
            onNormalBoot = {},
            onViewLogs = {},
            onRestoreBackup = {}
        )
    }
}

@Preview(name = "Recovery - Dark", showBackground = true)
@Composable
private fun RecoveryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RecoveryScreen(
            state = RecoveryState(
                crashReason = "数据库写入异常导致崩溃",
                crashTime = "昨天 22:15",
                restoring = true
            ),
            onSafeBoot = {},
            onNormalBoot = {},
            onViewLogs = {},
            onRestoreBackup = {}
        )
    }
}
