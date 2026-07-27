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
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTopBar

@Composable
fun PermissionModeScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val overview by viewModel.overview.collectAsStateWithLifecycle()
    PermissionModeContent(
        currentMode = overview.currentMode,
        onBack = onBack,
        onSelectMode = { mode ->
            if (mode.weight > overview.currentMode.weight) {
                viewModel.setPermissionMode(mode)
            } else {
                viewModel.setPermissionMode(mode)
            }
        }
    )
}

@Composable
fun PermissionModeContent(
    currentMode: PermissionMode,
    onBack: () -> Unit,
    onSelectMode: (PermissionMode) -> Unit
) {
    var pendingMode by remember { mutableStateOf<PermissionMode?>(null) }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "权限模式", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "hint") {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = "切换到更高权限模式时需要二次确认",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
            items(PermissionMode.entries, key = { it.name }) { mode ->
                PermissionModeCard(
                    mode = mode,
                    selected = currentMode == mode,
                    onSelect = {
                        if (mode.weight > currentMode.weight) {
                            pendingMode = mode
                        } else {
                            onSelectMode(mode)
                        }
                    }
                )
            }
        }
    }
    pendingMode?.let { mode ->
        AmitiaDangerDialog(
            onDismiss = { pendingMode = null },
            onConfirm = {
                onSelectMode(mode)
                pendingMode = null
            },
            title = "切换到 ${mode.label}",
            message = "即将切换到更高权限模式",
            impactDescription = mode.risk,
            confirmText = "确认切换",
            dangerLevel = com.amitia.core.designsystem.DangerLevel.Three
        )
    }
}

@Composable
private fun PermissionModeCard(
    mode: PermissionMode,
    selected: Boolean,
    onSelect: () -> Unit
) {
    val accentColor = when (mode) {
        PermissionMode.FullControl -> MaterialTheme.colorScheme.error
        PermissionMode.AutoApproval -> MaterialTheme.colorScheme.tertiary
        PermissionMode.ManualApproval -> MaterialTheme.colorScheme.primary
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = if (selected) accentColor.copy(alpha = 0.1f) else MaterialTheme.colorScheme.surface
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
                        imageVector = when (mode) {
                            PermissionMode.FullControl -> AmitiaIcons.Bolt
                            PermissionMode.AutoApproval -> AmitiaIcons.AutoAwesome
                            PermissionMode.ManualApproval -> AmitiaIcons.Security
                        },
                        contentDescription = null,
                        tint = accentColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = mode.label,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = mode.risk,
                        style = MaterialTheme.typography.labelMedium,
                        color = accentColor
                    )
                }
                if (selected) {
                    Surface(shape = MaterialTheme.shapes.small, color = accentColor) {
                        Text(
                            text = "当前",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                        )
                    }
                }
            }
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                ModeDetailRow(label = "允许范围", value = mode.description)
                ModeDetailRow(label = "典型行为", value = when (mode) {
                    PermissionMode.FullControl -> "自主完成所有操作，如打开应用、编辑文件、发送消息"
                    PermissionMode.AutoApproval -> "符合安全规则的操作自动执行，其余转入审批队列"
                    PermissionMode.ManualApproval -> "每个操作均需用户逐次批准后才能执行"
                })
            }
            AmitiaSelectionRow(
                title = if (selected) "已选择" else "选择此模式",
                selected = selected,
                onSelect = onSelect,
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Composable
private fun ModeDetailRow(label: String, value: String) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 3, overflow = TextOverflow.Ellipsis
        )
    }
}

private val PermissionMode.weight: Int get() = when (this) {
    PermissionMode.FullControl -> 3
    PermissionMode.AutoApproval -> 2
    PermissionMode.ManualApproval -> 1
}

@Preview(name = "Permission Mode - Light", showBackground = true)
@Composable
private fun PermissionModeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PermissionModeContent(
            currentMode = PermissionMode.ManualApproval,
            onBack = {}, onSelectMode = {}
        )
    }
}

@Preview(name = "Permission Mode - Dark", showBackground = true)
@Composable
private fun PermissionModeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PermissionModeContent(
            currentMode = PermissionMode.AutoApproval,
            onBack = {}, onSelectMode = {}
        )
    }
}
