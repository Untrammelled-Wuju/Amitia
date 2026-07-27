package com.amitia.feature.settings.backup

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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.DangerLevel
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.AmitiaDangerDialog
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.feature.settings.BackupState
import com.amitia.feature.settings.BackupItem
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun BackupRestoreScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val backup = state.backup
    var showRestoreDialog by remember { mutableStateOf<BackupItem?>(null) }

    BackupRestoreScreenContent(
        backup = backup,
        onBack = onBack,
        onCreateBackup = viewModel::startBackup,
        onRestore = { showRestoreDialog = it },
        onToggleAuto = { viewModel.updateAutoBackup(it) }
    )

    showRestoreDialog?.let { item ->
        AmitiaDangerDialog(
            onDismiss = { showRestoreDialog = null },
            onConfirm = {
                viewModel.startRestore(item.id)
                showRestoreDialog = null
            },
            title = "恢复备份",
            message = "将从备份 ${item.name}（${item.date}）恢复数据。",
            impactDescription = "恢复将覆盖当前所有数据，包括角色、对话、记忆和设置。恢复前请确保已备份当前数据。",
            confirmText = "确认恢复",
            dangerLevel = DangerLevel.Two
        )
    }
}

@Composable
private fun BackupRestoreScreenContent(
    backup: BackupState,
    onBack: () -> Unit,
    onCreateBackup: () -> Unit,
    onRestore: (BackupItem) -> Unit,
    onToggleAuto: (Boolean) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "备份与恢复", onBack = onBack) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSection(title = "创建备份") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        if (backup.isBackingUp) {
                            Text(
                                text = "正在备份...",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.primary
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                            LinearProgressIndicator(
                                progress = { backup.backupProgress },
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .height(8.dp)
                                    .clip(RoundedCornerShape(4.dp)),
                                color = MaterialTheme.colorScheme.primary,
                                trackColor = MaterialTheme.colorScheme.surfaceVariant
                            )
                            Text(
                                text = "${(backup.backupProgress * 100).toInt()}%",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else {
                            LoadingButton(
                                text = "立即备份",
                                onClick = onCreateBackup,
                                loading = false,
                                modifier = Modifier.fillMaxWidth(),
                                leadingIcon = AmitiaIcons.Backup
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "自动备份") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "自动备份",
                            subtitle = "每天凌晨自动创建备份",
                            checked = backup.autoBackup,
                            onCheckedChange = onToggleAuto,
                            leadingIcon = AmitiaIcons.Schedule
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "备份加密",
                            subtitle = "使用 AES-256 加密备份文件",
                            checked = backup.encryptionEnabled,
                            onCheckedChange = {},
                            leadingIcon = AmitiaIcons.Lock
                        )
                    }
                }
            }
            if (backup.isRestoring) {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Text(
                            text = "正在恢复...",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                        LinearProgressIndicator(
                            progress = { backup.restoreProgress },
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(8.dp)
                                .clip(RoundedCornerShape(4.dp)),
                            color = MaterialTheme.colorScheme.primary,
                            trackColor = MaterialTheme.colorScheme.surfaceVariant
                        )
                        Text(
                            text = "${(backup.restoreProgress * 100).toInt()}%",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
            AmitiaSection(title = "备份列表") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        backup.backups.forEachIndexed { index, item ->
                            BackupItemRow(
                                item = item,
                                onRestore = { onRestore(item) }
                            )
                            if (index < backup.backups.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun BackupItemRow(
    item: BackupItem,
    onRestore: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Text(
                    text = item.name,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (item.encrypted) {
                    AmitiaStatusDot(color = AmitiaStateColors.Running)
                    Text(
                        text = "已加密",
                        style = MaterialTheme.typography.labelSmall,
                        color = AmitiaStateColors.Running
                    )
                }
            }
            Text(
                text = "${item.date} · ${item.size}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        SecondaryButton(
            text = "恢复",
            onClick = onRestore
        )
    }
}

@Preview(name = "备份与恢复页 - 亮色", showBackground = true)
@Composable
private fun BackupRestoreScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        BackupRestoreScreenContent(
            backup = BackupState(),
            onBack = {},
            onCreateBackup = {},
            onRestore = {},
            onToggleAuto = {}
        )
    }
}

@Preview(name = "备份与恢复页 - 暗色", showBackground = true)
@Composable
private fun BackupRestoreScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        BackupRestoreScreenContent(
            backup = BackupState(isBackingUp = true, backupProgress = 0.6f),
            onBack = {},
            onCreateBackup = {},
            onRestore = {},
            onToggleAuto = {}
        )
    }
}
