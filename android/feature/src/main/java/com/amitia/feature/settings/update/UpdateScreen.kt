package com.amitia.feature.settings.update

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SuccessBanner
import com.amitia.feature.settings.UpdateInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun UpdateScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val update = state.update

    UpdateScreenContent(
        update = update,
        onBack = onBack,
        onCheckUpdate = viewModel::checkUpdate
    )
}

@Composable
private fun UpdateScreenContent(
    update: UpdateInfo,
    onBack: () -> Unit,
    onCheckUpdate: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "更新", onBack = onBack) }
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
            AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Text(
                        text = "当前版本",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "v${update.currentVersion}",
                        style = MaterialTheme.typography.headlineSmall,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    if (update.updateAvailable) {
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                        Text(
                            text = "发现新版本 v${update.latestVersion}",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }
            }
            if (update.isDownloading) {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier.padding(AmitiaSpacing.Base),
                        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        Text(
                            text = "正在下载更新...",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                        LinearProgressIndicator(
                            progress = { update.downloadProgress },
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(8.dp)
                                .clip(RoundedCornerShape(4.dp)),
                            color = MaterialTheme.colorScheme.primary,
                            trackColor = MaterialTheme.colorScheme.surfaceVariant
                        )
                        Text(
                            text = "${(update.downloadProgress * 100).toInt()}%",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
            if (update.downloadComplete) {
                SuccessBanner(message = "下载完成，可以安装更新")
                PrimaryButton(
                    text = "安装更新",
                    onClick = {},
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.SystemUpdate
                )
            }
            if (update.updateAvailable && !update.isDownloading && !update.downloadComplete) {
                AmitiaSection(title = "更新说明") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            Text(
                                text = update.updateNotes.ifBlank {
                                    "新版本包含性能优化和问题修复。\n\n· 优化启动速度\n· 修复已知问题\n· 提升稳定性"
                                },
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
                LoadingButton(
                    text = "下载更新",
                    onClick = onCheckUpdate,
                    loading = false,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Download
                )
            }
            if (!update.updateAvailable && !update.isDownloading) {
                AmitiaSection(title = "检查更新") {
                    SecondaryButton(
                        text = "检查更新",
                        onClick = onCheckUpdate,
                        modifier = Modifier.fillMaxWidth(),
                        leadingIcon = AmitiaIcons.Refresh
                    )
                }
            }
            AmitiaSection(title = "更新设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        com.amitia.core.designsystem.component.SettingsRow(
                            title = "自动检查更新",
                            subtitle = "每天自动检查新版本",
                            leadingIcon = AmitiaIcons.Update,
                            onClick = {}
                        )
                        com.amitia.core.designsystem.component.AmitiaInsetDivider(
                            leadingInset = 56.dp + AmitiaSpacing.Base
                        )
                        com.amitia.core.designsystem.component.SettingsRow(
                            title = "更新历史",
                            subtitle = "查看版本更新记录",
                            leadingIcon = AmitiaIcons.History,
                            onClick = {}
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "更新页 - 亮色", showBackground = true)
@Composable
private fun UpdateScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        UpdateScreenContent(
            update = UpdateInfo(),
            onBack = {},
            onCheckUpdate = {}
        )
    }
}

@Preview(name = "更新页 - 暗色", showBackground = true)
@Composable
private fun UpdateScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        UpdateScreenContent(
            update = UpdateInfo(updateAvailable = true, latestVersion = "0.2.0"),
            onBack = {},
            onCheckUpdate = {}
        )
    }
}
