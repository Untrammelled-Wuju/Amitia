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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LinearProgressIndicator
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
import androidx.compose.ui.text.style.TextAlign
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
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun ExtensionImportScreen(
    onBack: () -> Unit,
    onInstalled: () -> Unit
) {
    var progress by remember { mutableStateOf(ImportProgress(ImportStep.SelectFile, 0f, "请选择扩展包")) }
    ExtensionImportContent(
        progress = progress,
        onBack = onBack,
        onSelectFile = { progress = ImportProgress(ImportStep.Verify, 0.2f, "校验中...") },
        onVerify = { progress = ImportProgress(ImportStep.ShowPermissions, 0.4f, "校验通过", sampleImportPermissions()) },
        onApprove = { progress = ImportProgress(ImportStep.Install, 0.7f, "安装中...") },
        onInstall = { progress = ImportProgress(ImportStep.Done, 1f, "安装完成") },
        onDone = onInstalled,
        onCancel = { progress = ImportProgress(ImportStep.SelectFile, 0f, "请选择扩展包") }
    )
}

@Composable
fun ExtensionImportContent(
    progress: ImportProgress,
    onBack: () -> Unit,
    onSelectFile: () -> Unit,
    onVerify: () -> Unit,
    onApprove: () -> Unit,
    onInstall: () -> Unit,
    onDone: () -> Unit,
    onCancel: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "导入扩展包", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "format_hint") {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = "支持 .amitiax 扩展包格式，禁止静默安装",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
            item(key = "steps") {
                StepIndicator(currentStep = progress.step)
            }
            item(key = "progress") {
                LinearProgressIndicator(
                    progress = { progress.progress },
                    modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(4.dp))
                )
                Text(
                    text = progress.message,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = AmitiaSpacing.Xs)
                )
            }
            when (progress.step) {
                ImportStep.SelectFile -> item(key = "select") {
                    SelectFileCard(onSelectFile = onSelectFile)
                }
                ImportStep.Verify -> item(key = "verify") {
                    VerifyCard(onVerify = onVerify)
                }
                ImportStep.ShowPermissions -> {
                    item(key = "perm_header") { AmitiaSectionHeader(title = "权限摘要") }
                    item(key = "perm_warn") {
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.medium,
                            color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                        ) {
                            Text(
                                text = "请仔细审查权限范围与风险，批准后将授予相应权限",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(AmitiaSpacing.Base)
                            )
                        }
                    }
                    items(progress.permissions, key = { it.name }) { perm ->
                        PermissionReviewRow(permission = perm)
                    }
                    item(key = "perm_actions") {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            DangerButton(
                                text = "取消",
                                onClick = onCancel,
                                modifier = Modifier.weight(1f)
                            )
                            PrimaryButton(
                                text = "批准并安装",
                                onClick = onApprove,
                                modifier = Modifier.weight(1f)
                            )
                        }
                    }
                }
                ImportStep.Install -> item(key = "installing") {
                    Box(
                        modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl),
                        contentAlignment = Alignment.Center
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Icon(
                                imageVector = AmitiaIcons.Download,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.primary,
                                modifier = Modifier.size(AmitiaIconSize.Huge)
                            )
                            Text(
                                text = "正在安装...",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface,
                                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
                            )
                        }
                    }
                }
                ImportStep.Done -> item(key = "done") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.4f)
                    ) {
                        Column(
                            modifier = Modifier.padding(AmitiaSpacing.Base),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.CheckCircle,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.size(AmitiaIconSize.Huge)
                            )
                            Text(
                                text = "安装成功",
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onSurface,
                                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
                            )
                            PrimaryButton(
                                text = "完成",
                                onClick = onDone,
                                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun StepIndicator(currentStep: ImportStep) {
    val steps = ImportStep.entries
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        steps.forEach { step ->
            val isActive = step == currentStep
            val isDone = steps.indexOf(step) < steps.indexOf(currentStep)
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Box(
                    modifier = Modifier
                        .size(28.dp)
                        .clip(CircleShape)
                        .background(
                            when {
                                isDone -> MaterialTheme.colorScheme.tertiary
                                isActive -> MaterialTheme.colorScheme.primary
                                else -> MaterialTheme.colorScheme.surfaceVariant
                            }
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    if (isDone) {
                        Icon(
                            imageVector = AmitiaIcons.Check,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                    } else {
                        Text(
                            text = "${steps.indexOf(step) + 1}",
                            style = MaterialTheme.typography.labelSmall,
                            color = if (isActive) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
                            fontWeight = FontWeight.Bold
                        )
                    }
                }
                Text(
                    text = step.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = if (isActive) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                    textAlign = TextAlign.Center
                )
            }
        }
    }
}

@Composable
private fun SelectFileCard(onSelectFile: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                imageVector = AmitiaIcons.UploadFile,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(AmitiaIconSize.Huge)
            )
            Text(
                text = "点击选择 .amitiax 扩展包文件",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
            PrimaryButton(
                text = "选择文件",
                onClick = onSelectFile,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
        }
    }
}

@Composable
private fun VerifyCard(onVerify: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                imageVector = AmitiaIcons.VerifiedUser,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(AmitiaIconSize.Huge)
            )
            Text(
                text = "正在校验签名与结构",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
            SecondaryButton(
                text = "下一步",
                onClick = onVerify,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
        }
    }
}

@Composable
private fun PermissionReviewRow(permission: PluginPermission) {
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
                    .background(
                        when (permission.riskLevel) {
                            PermissionRiskLevel.Critical, PermissionRiskLevel.High -> MaterialTheme.colorScheme.errorContainer
                            PermissionRiskLevel.Medium -> MaterialTheme.colorScheme.tertiaryContainer
                            PermissionRiskLevel.Low -> MaterialTheme.colorScheme.primaryContainer
                        }
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Security,
                    contentDescription = null,
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
                    text = "${permission.category.label} · ${permission.description}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2, overflow = TextOverflow.Ellipsis
                )
            }
            Text(
                text = permission.riskLevel.label,
                style = MaterialTheme.typography.labelMedium,
                color = when (permission.riskLevel) {
                    PermissionRiskLevel.Critical, PermissionRiskLevel.High -> MaterialTheme.colorScheme.error
                    PermissionRiskLevel.Medium -> MaterialTheme.colorScheme.tertiary
                    PermissionRiskLevel.Low -> MaterialTheme.colorScheme.primary
                }
            )
        }
    }
}

private fun sampleImportPermissions() = listOf(
    PluginPermission("网络访问", "访问网络资源", true, PermissionRiskLevel.Low, PermissionCategory.Network),
    PluginPermission("文件读写", "读写本地文件", true, PermissionRiskLevel.Medium, PermissionCategory.File),
    PluginPermission("后台任务", "定时执行任务", false, PermissionRiskLevel.High, PermissionCategory.BackgroundTask)
)

@Preview(name = "Extension Import - Light", showBackground = true)
@Composable
private fun ExtensionImportLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ExtensionImportContent(
            progress = ImportProgress(ImportStep.ShowPermissions, 0.4f, "校验通过", sampleImportPermissions()),
            onBack = {}, onSelectFile = {}, onVerify = {}, onApprove = {}, onInstall = {}, onDone = {}, onCancel = {}
        )
    }
}

@Preview(name = "Extension Import - Dark", showBackground = true)
@Composable
private fun ExtensionImportDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ExtensionImportContent(
            progress = ImportProgress(ImportStep.SelectFile, 0f, "请选择扩展包"),
            onBack = {}, onSelectFile = {}, onVerify = {}, onApprove = {}, onInstall = {}, onDone = {}, onCancel = {}
        )
    }
}
