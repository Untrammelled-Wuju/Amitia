package com.amitia.feature.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.CloudSync
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Logout
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.RecordVoiceOver
import androidx.compose.material.icons.outlined.Restore
import androidx.compose.material.icons.outlined.Storage
import androidx.compose.material.icons.outlined.TextSnippet
import androidx.compose.material.icons.outlined.Tune
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Divider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors

@Composable
fun SettingsScreen(
    onOpenRuntime: () -> Unit,
    onLogout: () -> Unit,
    viewModel: SettingsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var showRemoteDialog by remember { mutableStateOf(false) }
    var showLogoutDialog by remember { mutableStateOf(false) }
    var showCacheDialog by remember { mutableStateOf(false) }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "设置",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            SettingsGroup(title = "运行时") {
                SettingsToggleRow(
                    title = "本地模式",
                    subtitle = "在本机启动 Runtime",
                    selected = state.runtimeMode == com.amitia.core.network.endpoint.RuntimeEndpointProvider.RuntimeMode.LOCAL,
                    onClick = viewModel::switchToLocalMode
                )
                SettingsActionRow(
                    icon = Icons.Outlined.CloudSync,
                    title = "远程模式",
                    subtitle = if (state.remoteUrl.isBlank()) "未配置" else state.remoteUrl,
                    onClick = { showRemoteDialog = true }
                )
                SettingsActionRow(
                    icon = Icons.Outlined.Devices,
                    title = "Runtime 管理",
                    subtitle = "服务状态、诊断、备份",
                    onClick = onOpenRuntime
                )
            }
            SettingsGroup(title = "外观") {
                ThemeRow(state = state, onSet = viewModel::setThemeMode)
            }
            SettingsGroup(title = "通知与语音") {
                SettingsSwitchRow(
                    icon = Icons.Outlined.Notifications,
                    title = "通知",
                    subtitle = "新消息与运行时事件",
                    checked = state.notificationsEnabled,
                    onChange = viewModel::setNotificationsEnabled
                )
                SettingsSwitchRow(
                    icon = Icons.Outlined.RecordVoiceOver,
                    title = "自动播放 TTS",
                    subtitle = "收到 voice_audio 事件后自动播放",
                    checked = state.ttsAutoPlay,
                    onChange = viewModel::setTtsAutoPlay
                )
                OutlinedTextField(
                    value = state.preferredVoice.orEmpty(),
                    onValueChange = viewModel::setPreferredVoice,
                    label = { Text(text = "TTS 语音 ID（可选）") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )
            }
            SettingsGroup(title = "数据") {
                SettingsActionRow(
                    icon = Icons.Outlined.Storage,
                    title = "缓存目录",
                    subtitle = state.cacheDirHint.ifBlank { "使用默认目录" },
                    onClick = {}
                )
                SettingsActionRow(
                    icon = Icons.Outlined.Delete,
                    title = "清理缓存",
                    subtitle = "不影响对话与记忆数据",
                    onClick = { showCacheDialog = true }
                )
                SettingsActionRow(
                    icon = Icons.Outlined.Restore,
                    title = "数据备份",
                    subtitle = state.backupStatus.ifBlank { "请求后端生成备份" },
                    onClick = viewModel::triggerBackup
                )
                SettingsActionRow(
                    icon = Icons.Outlined.Restore,
                    title = "数据恢复",
                    subtitle = "从最近备份恢复",
                    onClick = viewModel::triggerRestore
                )
            }
            SettingsGroup(title = "日志与关于") {
                SettingsActionRow(
                    icon = Icons.Outlined.TextSnippet,
                    title = "日志级别",
                    subtitle = state.logLevel,
                    onClick = {}
                )
                SettingsActionRow(
                    icon = Icons.Outlined.Info,
                    title = "版本",
                    subtitle = state.appVersion,
                    onClick = {}
                )
            }
            SettingsGroup(title = "账户") {
                SettingsActionRow(
                    icon = Icons.Outlined.Logout,
                    title = "退出登录",
                    subtitle = "清除本地会话与连接",
                    onClick = { showLogoutDialog = true }
                )
            }
            Spacer(modifier = Modifier.height(24.dp))
        }
    }
    if (showRemoteDialog) {
        RemoteConfigDialog(
            currentUrl = state.remoteUrl,
            onDismiss = { showRemoteDialog = false },
            onConfirm = { url, token ->
                viewModel.switchToRemoteMode(url, token)
                showRemoteDialog = false
            }
        )
    }
    if (showLogoutDialog) {
        AlertDialog(
            onDismissRequest = { showLogoutDialog = false },
            title = { Text(text = "退出登录") },
            text = { Text(text = "将清除本地会话，需要重新登录或使用访客模式。") },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.logout()
                    showLogoutDialog = false
                    onLogout()
                }) {
                    Text(text = "退出", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { showLogoutDialog = false }) {
                    Text(text = "取消")
                }
            }
        )
    }
    if (showCacheDialog) {
        AlertDialog(
            onDismissRequest = { showCacheDialog = false },
            title = { Text(text = "清理缓存") },
            text = { Text(text = "清理本地缓存目录中的临时文件，不会影响对话历史、记忆与角色。") },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.clearCache()
                    showCacheDialog = false
                }) {
                    Text(text = "清理")
                }
            },
            dismissButton = {
                TextButton(onClick = { showCacheDialog = false }) {
                    Text(text = "取消")
                }
            }
        )
    }
}

@Composable
private fun SettingsGroup(title: String, content: @Composable () -> Unit) {
    Text(
        text = title,
        style = MaterialTheme.typography.labelLarge,
        color = AmitiaColors.OnSurfaceMuted,
        modifier = Modifier.padding(top = 8.dp, bottom = 4.dp)
    )
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = androidx.compose.foundation.shape.RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(vertical = 4.dp)) {
            content()
        }
    }
}

@Composable
private fun SettingsActionRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.padding(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun SettingsSwitchRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    checked: Boolean,
    onChange: (Boolean) -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.padding(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Switch(checked = checked, onCheckedChange = onChange)
    }
}

@Composable
private fun SettingsToggleRow(
    title: String,
    subtitle: String,
    selected: Boolean,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        RadioButton(selected = selected, onClick = onClick)
    }
}

@Composable
private fun ThemeRow(
    state: SettingsUiState,
    onSet: (SettingsDataStore.ThemeMode) -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = Icons.Outlined.Tune,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.padding(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "主题",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "深色优先，亮色为辅助",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        SettingsDataStore.ThemeMode.entries.forEach { mode ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                RadioButton(selected = state.themeMode == mode, onClick = { onSet(mode) })
                Text(
                    text = when (mode) {
                        SettingsDataStore.ThemeMode.SYSTEM -> "跟随系统"
                        SettingsDataStore.ThemeMode.DARK -> "深色"
                        SettingsDataStore.ThemeMode.LIGHT -> "浅色"
                    },
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun RemoteConfigDialog(
    currentUrl: String,
    onDismiss: () -> Unit,
    onConfirm: (url: String, token: String) -> Unit
) {
    var url by remember(currentUrl) { mutableStateOf(currentUrl) }
    var token by remember { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(text = "远程模式") },
        text = {
            Column(modifier = Modifier.fillMaxWidth()) {
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text(text = "后端地址") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = token,
                    onValueChange = { token = it },
                    label = { Text(text = "访问令牌（可选）") },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation()
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = { onConfirm(url.trim(), token.trim()) },
                enabled = url.startsWith("http")
            ) {
                Text(text = "切换")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(text = "取消")
            }
        }
    )
}
