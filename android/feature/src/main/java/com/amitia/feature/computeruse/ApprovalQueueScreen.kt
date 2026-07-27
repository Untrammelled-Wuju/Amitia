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
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun ApprovalQueueScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val approvals by viewModel.pendingApprovals.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    ApprovalQueueContent(
        approvals = approvals,
        loading = loading,
        onBack = onBack,
        onApproveOnce = viewModel::approveOnce,
        onAlwaysAllow = viewModel::alwaysAllow,
        onDeny = viewModel::denyApproval
    )
}

@Composable
fun ApprovalQueueContent(
    approvals: List<PendingApproval>,
    loading: Boolean,
    onBack: () -> Unit,
    onApproveOnce: (String) -> Unit,
    onAlwaysAllow: (String) -> Unit,
    onDeny: (String) -> Unit
) {
    var pendingDeny by remember { mutableStateOf<PendingApproval?>(null) }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "审批队列", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            if (loading) {
                item(key = "loading") {
                    Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                        Text(
                            text = "加载审批队列...",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            } else if (approvals.isEmpty()) {
                item(key = "empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.CheckCircle,
                        title = "没有待审批操作",
                        description = "当 AI 请求执行需要确认的操作时将出现在这里",
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            } else {
                item(key = "hint") {
                    WarningBanner(message = "共 ${approvals.size} 个操作等待你的审批，请逐一确认")
                }
                item(key = "header") { AmitiaSectionHeader(title = "待执行操作") }
                items(approvals, key = { it.id }) { approval ->
                    ApprovalCard(
                        approval = approval,
                        onApproveOnce = { onApproveOnce(approval.id) },
                        onAlwaysAllow = { onAlwaysAllow(approval.id) },
                        onDeny = { pendingDeny = approval }
                    )
                }
            }
        }
    }
    pendingDeny?.let { approval ->
        AmitiaConfirmDialog(
            onDismiss = { pendingDeny = null },
            onConfirm = {
                onDeny(approval.id)
                pendingDeny = null
            },
            title = "拒绝操作",
            message = "确认拒绝「${approval.operation}」？拒绝后该操作不会执行。",
            confirmText = "确认拒绝",
            destructive = true
        )
    }
}

@Composable
private fun ApprovalCard(
    approval: PendingApproval,
    onApproveOnce: () -> Unit,
    onAlwaysAllow: () -> Unit,
    onDeny: () -> Unit
) {
    val riskColor = when (approval.risk) {
        ApprovalRisk.Low -> MaterialTheme.colorScheme.primary
        ApprovalRisk.Medium -> MaterialTheme.colorScheme.tertiary
        ApprovalRisk.High -> MaterialTheme.colorScheme.error
        ApprovalRisk.Critical -> MaterialTheme.colorScheme.error
    }
    val riskBg = when (approval.risk) {
        ApprovalRisk.Critical -> MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.25f)
        ApprovalRisk.High -> MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.15f)
        else -> MaterialTheme.colorScheme.surface
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = riskBg
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier.size(36.dp).clip(CircleShape).background(riskColor.copy(alpha = 0.15f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = when (approval.risk) {
                            ApprovalRisk.Critical -> AmitiaIcons.Warning
                            ApprovalRisk.High -> AmitiaIcons.WarningAmber
                            ApprovalRisk.Medium -> AmitiaIcons.Info
                            ApprovalRisk.Low -> AmitiaIcons.InfoOutlined
                        },
                        contentDescription = null,
                        tint = riskColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = approval.operation,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 2, overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "来自 ${approval.sourceRole} · ${approval.sourceTask}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Surface(shape = MaterialTheme.shapes.small, color = riskColor.copy(alpha = 0.2f)) {
                    Text(
                        text = approval.risk.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = riskColor,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            Text(
                text = approval.description,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 3, overflow = TextOverflow.Ellipsis
            )
            Text(
                text = "请求时间 ${approval.timestamp}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                PrimaryButton(
                    text = "允许一次",
                    onClick = onApproveOnce,
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Check
                )
                SecondaryButton(
                    text = "始终允许",
                    onClick = onAlwaysAllow,
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.ToggleOn
                )
            }
            DangerButton(
                text = "拒绝",
                onClick = onDeny,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Block
            )
        }
    }
}

@Preview(name = "Approval Queue - Light", showBackground = true)
@Composable
private fun ApprovalQueueLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ApprovalQueueContent(
            approvals = listOf(
                PendingApproval("a1", "删除文件 config.ini", ApprovalRisk.High, "艾米", "清理临时文件", "请求删除 Downloads 目录下的配置文件", "14:32"),
                PendingApproval("a2", "打开支付宝", ApprovalRisk.Critical, "艾米", "代为支付账单", "请求打开支付宝应用", "14:35")
            ),
            loading = false, onBack = {},
            onApproveOnce = {}, onAlwaysAllow = {}, onDeny = {}
        )
    }
}

@Preview(name = "Approval Queue - Dark", showBackground = true)
@Composable
private fun ApprovalQueueDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ApprovalQueueContent(
            approvals = emptyList(), loading = false, onBack = {},
            onApproveOnce = {}, onAlwaysAllow = {}, onDeny = {}
        )
    }
}
