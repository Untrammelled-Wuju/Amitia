package com.amitia.core.designsystem.component

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.EmptyReason

@Composable
fun LoadingSkeleton(
    modifier: Modifier = Modifier,
    lineCount: Int = 3,
    lineHeight: Int = 16
) {
    val infiniteTransition = rememberInfiniteTransition(label = "skeleton")
    val alpha by infiniteTransition.animateFloat(
        initialValue = 0.3f,
        targetValue = 0.6f,
        animationSpec = infiniteRepeatable(
            animation = tween(1000),
            repeatMode = RepeatMode.Reverse
        ),
        label = "skeletonAlpha"
    )
    Column(
        modifier = modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        repeat(lineCount) { index ->
            val widthFraction = if (index == lineCount - 1) 0.6f else 1f
            Box(
                modifier = Modifier
                    .fillMaxWidth(widthFraction)
                    .height(lineHeight.dp)
                    .clip(AmitiaPillShape)
                    .background(MaterialTheme.colorScheme.onSurface.copy(alpha = alpha * 0.1f))
            )
        }
    }
}

@Composable
fun InlineLoading(
    modifier: Modifier = Modifier,
    message: String? = null,
    size: Int = 24
) {
    Row(
        modifier = modifier.padding(AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        CircularProgressIndicator(
            modifier = Modifier.size(size.dp),
            strokeWidth = 2.dp,
            color = MaterialTheme.colorScheme.primary
        )
        if (message != null) {
            Text(
                text = message,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
fun AmitiaEmptyState(
    icon: ImageVector,
    title: String,
    modifier: Modifier = Modifier,
    description: String? = null,
    reason: EmptyReason = EmptyReason.NoData,
    primaryAction: @Composable (() -> Unit)? = null,
    secondaryAction: @Composable (() -> Unit)? = null
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(AmitiaSpacing.Xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(80.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                modifier = Modifier.size(AmitiaIconSize.Huge)
            )
        }
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface,
            textAlign = TextAlign.Center
        )
        if (description != null) {
            Text(
                text = description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
        }
        if (primaryAction != null || secondaryAction != null) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (primaryAction != null) primaryAction()
                if (secondaryAction != null) secondaryAction()
            }
        }
    }
}

@Composable
fun AmitiaErrorState(
    icon: ImageVector,
    title: String,
    modifier: Modifier = Modifier,
    description: String? = null,
    onRetry: (() -> Unit)? = null,
    onDiagnostics: (() -> Unit)? = null
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(AmitiaSpacing.Xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(80.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.error,
                modifier = Modifier.size(AmitiaIconSize.Huge)
            )
        }
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface,
            textAlign = TextAlign.Center
        )
        if (description != null) {
            Text(
                text = description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
                maxLines = 4,
                overflow = TextOverflow.Ellipsis
            )
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        Row(
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (onRetry != null) {
                PrimaryButton(
                    text = "重试",
                    onClick = onRetry,
                    leadingIcon = AmitiaIcons.Refresh
                )
            }
            if (onDiagnostics != null) {
                TertiaryButton(
                    text = "诊断",
                    onClick = onDiagnostics,
                    leadingIcon = AmitiaIcons.BugReport
                )
            }
        }
    }
}

@Composable
fun OfflineBanner(
    modifier: Modifier = Modifier,
    onRetry: (() -> Unit)? = null
) {
    AmitiaInfoBanner(
        icon = AmitiaIcons.WifiOff,
        message = "网络连接不可用",
        actionLabel = if (onRetry != null) "重试" else null,
        onAction = onRetry,
        containerColor = MaterialTheme.colorScheme.surfaceVariant,
        contentColor = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = modifier
    )
}

@Composable
fun RuntimeUnavailableBanner(
    modifier: Modifier = Modifier,
    message: String = "运行时服务不可用",
    onConfigure: (() -> Unit)? = null
) {
    AmitiaInfoBanner(
        icon = AmitiaIcons.Error,
        message = message,
        actionLabel = if (onConfigure != null) "配置" else null,
        onAction = onConfigure,
        containerColor = MaterialTheme.colorScheme.errorContainer,
        contentColor = MaterialTheme.colorScheme.onErrorContainer,
        modifier = modifier
    )
}

@Composable
fun PermissionRequiredState(
    permissionName: String,
    modifier: Modifier = Modifier,
    description: String = "此功能需要相关权限才能正常使用",
    icon: ImageVector = AmitiaIcons.Lock,
    onGrant: (() -> Unit)? = null
) {
    AmitiaEmptyState(
        icon = icon,
        title = "需要 $permissionName 权限",
        description = description,
        modifier = modifier,
        reason = EmptyReason.PermissionDenied,
        primaryAction = if (onGrant != null) {
            { PrimaryButton(text = "授予权限", onClick = onGrant, leadingIcon = AmitiaIcons.Security) }
        } else null
    )
}

@Composable
fun PartialFailureCard(
    message: String,
    modifier: Modifier = Modifier,
    onDismiss: (() -> Unit)? = null,
    onRetry: (() -> Unit)? = null
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.WarningAmber,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.tertiary,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            Text(
                text = message,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.weight(1f),
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            if (onRetry != null) {
                TertiaryButton(text = "重试", onClick = onRetry)
            }
            if (onDismiss != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Close,
                    contentDescription = "关闭",
                    onClick = onDismiss
                )
            }
        }
    }
}

@Composable
fun SuccessBanner(
    message: String,
    modifier: Modifier = Modifier,
    onDismiss: (() -> Unit)? = null
) {
    AmitiaInfoBanner(
        icon = AmitiaIcons.CheckCircle,
        message = message,
        actionLabel = null,
        onAction = null,
        onDismiss = onDismiss,
        containerColor = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.4f),
        contentColor = MaterialTheme.colorScheme.onTertiaryContainer,
        iconTint = AmitiaStateColors.Running,
        modifier = modifier
    )
}

@Composable
fun WarningBanner(
    message: String,
    modifier: Modifier = Modifier,
    onDismiss: (() -> Unit)? = null,
    onAction: (() -> Unit)? = null,
    actionLabel: String? = null
) {
    AmitiaInfoBanner(
        icon = AmitiaIcons.WarningAmber,
        message = message,
        actionLabel = actionLabel,
        onAction = onAction,
        onDismiss = onDismiss,
        containerColor = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f),
        contentColor = MaterialTheme.colorScheme.onTertiaryContainer,
        iconTint = AmitiaStateColors.Degraded,
        modifier = modifier
    )
}

@Composable
private fun AmitiaInfoBanner(
    icon: ImageVector,
    message: String,
    modifier: Modifier = Modifier,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    onDismiss: (() -> Unit)? = null,
    containerColor: Color = MaterialTheme.colorScheme.surfaceVariant,
    contentColor: Color = MaterialTheme.colorScheme.onSurfaceVariant,
    iconTint: Color = contentColor
) {
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        shape = AmitiaCardShape,
        color = containerColor
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = iconTint,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            Text(
                text = message,
                style = MaterialTheme.typography.bodySmall,
                color = contentColor,
                modifier = Modifier.weight(1f),
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            if (actionLabel != null && onAction != null) {
                val interactionSource = remember { MutableInteractionSource() }
                Text(
                    text = actionLabel,
                    style = MaterialTheme.typography.labelLarge,
                    color = contentColor,
                    modifier = Modifier
                        .clip(AmitiaPillShape)
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onAction
                        )
                        .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
                )
            }
            if (onDismiss != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Close,
                    contentDescription = "关闭",
                    onClick = onDismiss
                )
            }
        }
    }
}

@Preview(name = "State - Light", showBackground = true)
@Composable
private fun AmitiaStateLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            LoadingSkeleton(lineCount = 3)
            InlineLoading(message = "加载中...")
            OfflineBanner(onRetry = {})
            RuntimeUnavailableBanner(onConfigure = {})
            SuccessBanner(message = "操作成功完成")
            WarningBanner(message = "部分功能可能受限", onDismiss = {})
            PartialFailureCard(
                message = "记忆服务连接超时，部分功能不可用",
                onRetry = {},
                onDismiss = {}
            )
        }
    }
}

@Preview(name = "Empty/Error State - Light", showBackground = true)
@Composable
private fun AmitiaEmptyErrorLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxl)
        ) {
            AmitiaEmptyState(
                icon = AmitiaIcons.ChatBubbleOutline,
                title = "还没有对话",
                description = "选择一个角色开始对话吧",
                primaryAction = { PrimaryButton(text = "开始对话", onClick = {}) }
            )
            AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = "连接失败",
                description = "无法连接到服务器，请检查网络后重试",
                onRetry = {},
                onDiagnostics = {}
            )
            PermissionRequiredState(
                permissionName = "通知",
                onGrant = {}
            )
        }
    }
}

@Preview(name = "State - Dark", showBackground = true)
@Composable
private fun AmitiaStateDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            OfflineBanner(onRetry = {})
            SuccessBanner(message = "操作成功完成")
            AmitiaEmptyState(
                icon = AmitiaIcons.Memory,
                title = "暂无记忆",
                description = "随着对话的进行，记忆会自动保存到这里"
            )
        }
    }
}
