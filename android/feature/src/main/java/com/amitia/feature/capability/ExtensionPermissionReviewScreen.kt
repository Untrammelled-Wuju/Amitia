package com.amitia.feature.capability

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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ExtensionPermissionReviewScreen(
    onBack: () -> Unit,
    onApprove: () -> Unit,
    onReject: () -> Unit
) {
    ExtensionPermissionReviewContent(
        permissions = sampleReviewPermissions(),
        onBack = onBack,
        onApprove = onApprove,
        onReject = onReject
    )
}

@Composable
fun ExtensionPermissionReviewContent(
    permissions: List<PluginPermission>,
    onBack: () -> Unit,
    onApprove: () -> Unit,
    onReject: () -> Unit
) {
    val grouped = permissions.groupBy { it.category }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "权限审查", onBack = onBack)
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "risk_summary") {
                RiskSummaryCard(permissions = permissions)
            }
            item(key = "scope_hint") {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                ) {
                    Text(
                        text = "请逐一审查每个权限的范围与风险，批准后将授予相应权限",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
            PermissionCategory.entries.forEach { category ->
                val group = grouped[category] ?: emptyList()
                if (group.isNotEmpty()) {
                    item(key = "cat_${category.name}_header") {
                        AmitiaSectionHeader(
                            title = category.label,
                            trailing = {
                                Text(
                                    text = "${group.size} 项",
                                    style = MaterialTheme.typography.labelMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        )
                    }
                    items(group, key = { "${category.name}_${it.name}" }) { perm ->
                        PermissionDetailRow(permission = perm)
                    }
                }
            }
        }
        Surface(
            modifier = Modifier.fillMaxWidth(),
            color = MaterialTheme.colorScheme.surface,
            tonalElevation = 2.dp
        ) {
            Row(
                modifier = Modifier.padding(AmitiaSpacing.Base),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                DangerButton(
                    text = "拒绝",
                    onClick = onReject,
                    leadingIcon = AmitiaIcons.Close,
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "批准全部",
                    onClick = onApprove,
                    leadingIcon = AmitiaIcons.Check,
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun RiskSummaryCard(permissions: List<PluginPermission>) {
    val critical = permissions.count { it.riskLevel == PermissionRiskLevel.Critical }
    val high = permissions.count { it.riskLevel == PermissionRiskLevel.High }
    val medium = permissions.count { it.riskLevel == PermissionRiskLevel.Medium }
    val low = permissions.count { it.riskLevel == PermissionRiskLevel.Low }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Text(
                text = "风险概览",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                RiskStat("严重", critical, MaterialTheme.colorScheme.error)
                RiskStat("高", high, MaterialTheme.colorScheme.error)
                RiskStat("中", medium, MaterialTheme.colorScheme.tertiary)
                RiskStat("低", low, MaterialTheme.colorScheme.primary)
            }
        }
    }
}

@Composable
private fun RiskStat(label: String, count: Int, color: androidx.compose.ui.graphics.Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = count.toString(),
            style = MaterialTheme.typography.titleMedium,
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

@Composable
private fun PermissionDetailRow(permission: PluginPermission) {
    val riskColor = when (permission.riskLevel) {
        PermissionRiskLevel.Critical -> MaterialTheme.colorScheme.error
        PermissionRiskLevel.High -> MaterialTheme.colorScheme.error
        PermissionRiskLevel.Medium -> MaterialTheme.colorScheme.tertiary
        PermissionRiskLevel.Low -> MaterialTheme.colorScheme.primary
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(riskColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = permissionIcon(permission.category),
                    contentDescription = null,
                    tint = riskColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = permission.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "范围：${permission.description}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "风险：${permission.riskLevel.label}",
                    style = MaterialTheme.typography.labelSmall,
                    color = riskColor
                )
            }
        }
    }
}

private fun permissionIcon(category: PermissionCategory): ImageVector = when (category) {
    PermissionCategory.DataAccess -> AmitiaIcons.Storage
    PermissionCategory.Network -> AmitiaIcons.Hub
    PermissionCategory.File -> AmitiaIcons.Folder
    PermissionCategory.BackgroundTask -> AmitiaIcons.Schedule
    PermissionCategory.UiContribution -> AmitiaIcons.Widgets
    PermissionCategory.SystemControl -> AmitiaIcons.Settings
}

private fun sampleReviewPermissions() = listOf(
    PluginPermission("读取记忆数据", "访问长期记忆存储", false, PermissionRiskLevel.High, PermissionCategory.DataAccess),
    PluginPermission("写入记忆", "修改或新增记忆条目", false, PermissionRiskLevel.Critical, PermissionCategory.DataAccess),
    PluginPermission("网络请求", "向外部服务发起请求", true, PermissionRiskLevel.Medium, PermissionCategory.Network),
    PluginPermission("文件读取", "读取本地文件", true, PermissionRiskLevel.Low, PermissionCategory.File),
    PluginPermission("文件写入", "写入本地文件", false, PermissionRiskLevel.Medium, PermissionCategory.File),
    PluginPermission("定时任务", "后台定时执行", false, PermissionRiskLevel.High, PermissionCategory.BackgroundTask),
    PluginPermission("注入界面", "向对话注入 UI 元素", false, PermissionRiskLevel.Medium, PermissionCategory.UiContribution),
    PluginPermission("系统设置", "修改系统配置", false, PermissionRiskLevel.Critical, PermissionCategory.SystemControl)
)

@Preview(name = "Permission Review - Light", showBackground = true)
@Composable
private fun ExtensionPermissionReviewLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ExtensionPermissionReviewContent(
            permissions = sampleReviewPermissions(),
            onBack = {}, onApprove = {}, onReject = {}
        )
    }
}

@Preview(name = "Permission Review - Dark", showBackground = true)
@Composable
private fun ExtensionPermissionReviewDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ExtensionPermissionReviewContent(
            permissions = sampleReviewPermissions(),
            onBack = {}, onApprove = {}, onReject = {}
        )
    }
}
