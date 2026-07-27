package com.amitia.feature.settings.runmode

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.feature.settings.RunModeInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun RunModeScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val runMode = state.runMode
    var showSwitchDialog by remember { mutableStateOf<String?>(null) }
    var remoteAddress by remember { mutableStateOf(runMode.remoteAddress) }

    RunModeScreenContent(
        runMode = runMode,
        remoteAddress = remoteAddress,
        onRemoteAddressChange = { remoteAddress = it },
        onBack = onBack,
        onSwitchToLocal = { showSwitchDialog = "local" },
        onSwitchToRemote = { showSwitchDialog = "remote" }
    )

    showSwitchDialog?.let { mode ->
        AmitiaConfirmDialog(
            onDismiss = { showSwitchDialog = null },
            onConfirm = { showSwitchDialog = null },
            title = if (mode == "local") "切换到本地模式" else "切换到远程模式",
            message = if (mode == "local") {
                "切换到本地模式将在本机启动运行时服务。未同步的数据可能丢失，当前运行的服务将被重启。"
            } else {
                "切换到远程模式将连接到远程服务器。请确保网络连接正常，未同步的数据可能丢失。"
            },
            confirmText = "确认切换",
            destructive = true
        )
    }
}

@Composable
private fun RunModeScreenContent(
    runMode: RunModeInfo,
    remoteAddress: String,
    onRemoteAddressChange: (String) -> Unit,
    onBack: () -> Unit,
    onSwitchToLocal: () -> Unit,
    onSwitchToRemote: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "运行模式", onBack = onBack) }
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
            AmitiaSection(title = "当前模式") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = runMode.currentMode,
                                style = MaterialTheme.typography.titleMedium,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                            Spacer(modifier = Modifier.weight(1f))
                            AmitiaStatusDot(
                                color = if (runMode.isLocal) AmitiaStateColors.Running
                                else AmitiaStateColors.Pending
                            )
                            Text(
                                text = runMode.connectionStatus,
                                style = MaterialTheme.typography.labelMedium,
                                color = if (runMode.isLocal) AmitiaStateColors.Running
                                else AmitiaStateColors.Pending
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "模式选择") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSelectionRow(
                            title = "本地模式",
                            subtitle = "在本机启动运行时服务",
                            selected = runMode.isLocal,
                            onSelect = onSwitchToLocal,
                            leadingIcon = AmitiaIcons.Build
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "远程模式",
                            subtitle = "连接到远程服务器",
                            selected = !runMode.isLocal,
                            onSelect = onSwitchToRemote,
                            leadingIcon = AmitiaIcons.CloudDone
                        )
                    }
                }
            }
            if (!runMode.isLocal) {
                AmitiaSection(title = "远程服务地址") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            AmitiaTextField(
                                value = remoteAddress,
                                onValueChange = onRemoteAddressChange,
                                label = "服务地址",
                                placeholder = "https://example.com:8080",
                                leadingIcon = AmitiaIcons.Link
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                            PrimaryButton(
                                text = "连接测试",
                                onClick = {},
                                modifier = Modifier.fillMaxWidth(),
                                leadingIcon = AmitiaIcons.Speed
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "切换影响") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Text(
                            text = "切换运行模式将影响以下内容：",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                        Text(
                            text = "· 当前运行的服务将被停止并重启\n· 未同步的对话和记忆可能丢失\n· 网络连接将重新建立\n· 正在进行的功能调用可能中断",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "运行模式页 - 亮色", showBackground = true)
@Composable
private fun RunModeScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        RunModeScreenContent(
            runMode = RunModeInfo(),
            remoteAddress = "",
            onRemoteAddressChange = {},
            onBack = {},
            onSwitchToLocal = {},
            onSwitchToRemote = {}
        )
    }
}

@Preview(name = "运行模式页 - 暗色", showBackground = true)
@Composable
private fun RunModeScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        RunModeScreenContent(
            runMode = RunModeInfo(isLocal = false, remoteAddress = "https://example.com", connectionStatus = "已连接"),
            remoteAddress = "https://example.com",
            onRemoteAddressChange = {},
            onBack = {},
            onSwitchToLocal = {},
            onSwitchToRemote = {}
        )
    }
}
