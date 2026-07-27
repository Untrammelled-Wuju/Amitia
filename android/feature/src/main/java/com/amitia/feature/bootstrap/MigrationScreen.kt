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
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun MigrationScreen(
    state: MigrationState,
    onRollback: () -> Unit,
    onRetry: () -> Unit,
    onCompleted: () -> Unit,
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
            MigrationHeader()
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            VersionInfoRow(from = state.fromVersion, to = state.toVersion)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))

            if (state.failed) {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = "迁移失败",
                    description = "数据迁移未能完成，你可以回滚到上一版本或重试。强制退出可能导致数据损坏。",
                    onRetry = onRetry
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    if (state.rollbackAvailable) {
                        SecondaryButton(
                            text = "回滚",
                            onClick = onRollback,
                            modifier = Modifier.weight(1f),
                            leadingIcon = AmitiaIcons.Undo
                        )
                    }
                    DangerButton(
                        text = "强制退出",
                        onClick = onCompleted,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Warning
                    )
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                Text(
                    text = "强制退出有数据损坏风险，建议优先回滚",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error,
                    textAlign = TextAlign.Center
                )
                return@Column
            }

            if (state.completed) {
                CompletedContent(onCompleted = onCompleted)
                return@Column
            }

            LinearProgressIndicator(
                progress = { state.progress.coerceIn(0f, 1f) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(4.dp),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            Text(
                text = "${(state.progress * 100).toInt()}%",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            state.steps.forEach { step ->
                MigrationStepRow(step = step)
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Md))
            Text(
                text = "迁移期间请勿关闭应用或强制退出",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                textAlign = TextAlign.Center
            )
        }
    }
}

@Composable
private fun MigrationHeader() {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.SystemUpdate,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(28.dp)
            )
        }
        Text(
            text = "数据迁移",
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun VersionInfoRow(from: String, to: String) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(AmitiaSpacing.Base),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = "当前版本",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = if (from.isNotBlank()) "v$from" else "—",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
            Icon(
                imageVector = AmitiaIcons.ArrowForward,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = "目标版本",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = if (to.isNotBlank()) "v$to" else "—",
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.primary
                )
            }
        }
    }
}

@Composable
private fun MigrationStepRow(step: MigrationStep) {
    val color = when {
        step.failed -> AmitiaStateColors.Failed
        step.done -> AmitiaStateColors.Running
        step.inProgress -> AmitiaStateColors.Pending
        else -> MaterialTheme.colorScheme.surfaceVariant
    }
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
                .background(color),
            contentAlignment = Alignment.Center
        ) {
            if (step.done) {
                Icon(
                    imageVector = AmitiaIcons.Check,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimary,
                    modifier = Modifier.size(14.dp)
                )
            } else if (step.failed) {
                Icon(
                    imageVector = AmitiaIcons.Close,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onError,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        Text(
            text = step.name,
            style = MaterialTheme.typography.bodyMedium,
            color = if (step.done || step.inProgress) MaterialTheme.colorScheme.onSurface
            else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f)
        )
        if (step.inProgress) {
            Text(
                text = "进行中",
                style = MaterialTheme.typography.labelSmall,
                color = color
            )
        }
    }
}

@Composable
private fun CompletedContent(onCompleted: () -> Unit) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Box(
            modifier = Modifier
                .size(64.dp)
                .clip(CircleShape)
                .background(AmitiaStateColors.Running.copy(alpha = 0.15f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.CheckCircle,
                contentDescription = null,
                tint = AmitiaStateColors.Running,
                modifier = Modifier.size(32.dp)
            )
        }
        Text(
            text = "迁移完成",
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onBackground
        )
        PrimaryButton(
            text = "继续",
            onClick = onCompleted,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.ArrowForward
        )
    }
}

@Preview(name = "Migration - Light", showBackground = true)
@Composable
private fun MigrationLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MigrationScreen(
            state = MigrationState(
                fromVersion = "1.2.0",
                toVersion = "1.3.0",
                progress = 0.4f,
                steps = listOf(
                    MigrationStep("备份当前数据", done = true),
                    MigrationStep("迁移数据库结构", done = true),
                    MigrationStep("迁移记忆索引", done = false, inProgress = true),
                    MigrationStep("迁移角色配置", done = false),
                    MigrationStep("校验数据完整性", done = false)
                )
            ),
            onRollback = {},
            onRetry = {},
            onCompleted = {}
        )
    }
}

@Preview(name = "Migration - Dark", showBackground = true)
@Composable
private fun MigrationDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MigrationScreen(
            state = MigrationState(
                fromVersion = "1.2.0",
                toVersion = "1.3.0",
                failed = true,
                error = "记忆索引迁移失败"
            ),
            onRollback = {},
            onRetry = {},
            onCompleted = {}
        )
    }
}
