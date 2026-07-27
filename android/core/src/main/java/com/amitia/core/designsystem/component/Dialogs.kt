package com.amitia.core.designsystem.component

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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.AlertDialogDefaults
import androidx.compose.material3.BasicAlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetDefaults
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import com.amitia.core.designsystem.AmitiaBottomSheetShape
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaElevation
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaInputShape
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.DangerLevel
import com.amitia.core.designsystem.GlassLevel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AmitiaBottomSheet(
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    title: String? = null,
    showDragHandle: Boolean = true,
    content: @Composable () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        modifier = modifier,
        sheetState = sheetState,
        sheetShape = AmitiaBottomSheetShape,
        containerColor = MaterialTheme.colorScheme.surface,
        dragHandle = if (showDragHandle) {
            { ModalBottomSheetDefaults.DragHandle() }
        } else null
    ) {
        if (title != null) {
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
            )
        }
        content()
        Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
    }
}

@Composable
fun AmitiaDialog(
    onDismiss: () -> Unit,
    title: String,
    modifier: Modifier = Modifier,
    message: String? = null,
    icon: ImageVector? = null,
    confirmText: String = "确定",
    onConfirm: (() -> Unit)? = null,
    dismissText: String? = "取消",
    content: @Composable (() -> Unit)? = null
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        modifier = modifier,
        shape = AmitiaCardShape,
        containerColor = MaterialTheme.colorScheme.surface,
        titleContentColor = MaterialTheme.colorScheme.onSurface,
        title = {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (icon != null) {
                    Icon(
                        imageVector = icon,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(AmitiaIconSize.Large)
                    )
                }
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                if (message != null) {
                    Text(
                        text = message,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                content?.invoke()
            }
        },
        confirmButton = {
            if (onConfirm != null) {
                TertiaryButton(text = confirmText, onClick = { onConfirm(); onDismiss() })
            }
        },
        dismissButton = {
            if (dismissText != null) {
                TertiaryButton(text = dismissText, onClick = onDismiss)
            }
        }
    )
}

@Composable
fun AmitiaConfirmDialog(
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
    title: String,
    modifier: Modifier = Modifier,
    message: String? = null,
    confirmText: String = "确认",
    dismissText: String = "取消",
    destructive: Boolean = false
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        modifier = modifier,
        shape = AmitiaCardShape,
        containerColor = MaterialTheme.colorScheme.surface,
        title = {
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
        },
        text = {
            if (message != null) {
                Text(
                    text = message,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        },
        confirmButton = {
            if (destructive) {
                DangerButton(text = confirmText, onClick = { onConfirm(); onDismiss() })
            } else {
                PrimaryButton(text = confirmText, onClick = { onConfirm(); onDismiss() })
            }
        },
        dismissButton = {
            SecondaryButton(text = dismissText, onClick = onDismiss)
        }
    )
}

@Composable
fun AmitiaDangerDialog(
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
    title: String,
    modifier: Modifier = Modifier,
    message: String,
    impactDescription: String? = null,
    confirmText: String = "确认删除",
    dismissText: String = "取消",
    dangerLevel: DangerLevel = DangerLevel.Two
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        modifier = modifier,
        shape = AmitiaCardShape,
        containerColor = MaterialTheme.colorScheme.surface,
        title = {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Warning,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(AmitiaIconSize.Large)
                )
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Text(
                    text = message,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (impactDescription != null) {
                    Surface(
                        shape = AmitiaCardShape,
                        color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.3f)
                    ) {
                        Row(
                            modifier = Modifier.padding(AmitiaSpacing.Sm),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Info,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.error,
                                modifier = Modifier.size(AmitiaIconSize.Medium)
                            )
                            Text(
                                text = impactDescription,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onErrorContainer
                            )
                        }
                    }
                }
            }
        },
        confirmButton = {
            DangerButton(text = confirmText, onClick = { onConfirm(); onDismiss() })
        },
        dismissButton = {
            SecondaryButton(text = dismissText, onClick = onDismiss)
        }
    )
}

data class AmitiaMenuItem(
    val label: String,
    val icon: ImageVector? = null,
    val onClick: () -> Unit,
    val destructive: Boolean = false
)

@Composable
fun AmitiaActionMenu(
    expanded: Boolean,
    onDismiss: () -> Unit,
    items: List<AmitiaMenuItem>,
    modifier: Modifier = Modifier
) {
    DropdownMenu(
        expanded = expanded,
        onDismissRequest = onDismiss,
        modifier = modifier
            .clip(AmitiaCardShape)
            .widthIn(min = 180.dp)
    ) {
        items.forEachIndexed { index, item ->
            DropdownMenuItem(
                text = {
                    Text(
                        text = item.label,
                        style = MaterialTheme.typography.bodyMedium,
                        color = if (item.destructive) MaterialTheme.colorScheme.error
                        else MaterialTheme.colorScheme.onSurface
                    )
                },
                leadingIcon = item.icon?.let {
                    {
                        Icon(
                            imageVector = it,
                            contentDescription = null,
                            tint = if (item.destructive) MaterialTheme.colorScheme.error
                            else MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                },
                onClick = {
                    item.onClick()
                    onDismiss()
                }
            )
            if (index < items.lastIndex) {
                AmitiaInsetDivider(
                    leadingInset = AmitiaSpacing.Base,
                    trailingInset = AmitiaSpacing.None
                )
            }
        }
    }
}

@Composable
fun AmitiaTooltip(
    text: String,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null
) {
    Surface(
        modifier = modifier.clip(AmitiaCardShape),
        color = MaterialTheme.colorScheme.surfaceVariant,
        shadowElevation = AmitiaElevation.Level2
    ) {
        Row(
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            if (icon != null) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
            }
            Text(
                text = text,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
fun AmitiaSnackbarHost(
    hostState: SnackbarHostState,
    modifier: Modifier = Modifier
) {
    SnackbarHost(
        hostState = hostState,
        modifier = modifier
    )
}

@Preview(name = "Dialog - Light", showBackground = true)
@Composable
private fun AmitiaDialogLightPreview() {
    var showConfirm by remember { mutableStateOf(true) }
    var showDanger by remember { mutableStateOf(true) }
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.fillMaxSize()) {
            if (showConfirm) {
                AmitiaConfirmDialog(
                    onDismiss = { showConfirm = false },
                    onConfirm = { showConfirm = false },
                    title = "保存修改",
                    message = "是否保存当前修改？"
                )
            }
        }
    }
}

@Preview(name = "Danger Dialog - Dark", showBackground = true)
@Composable
private fun AmitiaDangerDialogDarkPreview() {
    var show by remember { mutableStateOf(true) }
    AmitiaTheme(darkTheme = true) {
        Box(modifier = Modifier.fillMaxSize()) {
            if (show) {
                AmitiaDangerDialog(
                    onDismiss = { show = false },
                    onConfirm = { show = false },
                    title = "删除角色",
                    message = "此操作将永久删除该角色及其所有对话记录。",
                    impactDescription = "删除后无法恢复，所有关联的记忆和设置也将被清除。",
                    confirmText = "永久删除"
                )
            }
        }
    }
}
