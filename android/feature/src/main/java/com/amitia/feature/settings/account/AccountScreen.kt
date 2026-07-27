package com.amitia.feature.settings.account

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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.AmitiaDangerDialog
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.DangerLevel
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.amitia.feature.settings.AccountInfo
import com.amitia.feature.settings.DeviceInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AccountScreen(
    onBack: () -> Unit,
    onLogout: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var showLogoutDialog by remember { mutableStateOf(false) }
    var showDeleteDialog by remember { mutableStateOf(false) }

    AccountScreenContent(
        account = state.account,
        onBack = onBack,
        onLogout = { showLogoutDialog = true },
        onDeleteAccount = { showDeleteDialog = true }
    )

    if (showLogoutDialog) {
        AmitiaConfirmDialog(
            onDismiss = { showLogoutDialog = false },
            onConfirm = {
                showLogoutDialog = false
                onLogout()
            },
            title = "退出登录",
            message = "将清除本地会话，需要重新登录或使用访客模式。",
            confirmText = "退出",
            destructive = true
        )
    }
    if (showDeleteDialog) {
        AmitiaDangerDialog(
            onDismiss = { showDeleteDialog = false },
            onConfirm = { showDeleteDialog = false },
            title = "删除账号",
            message = "此操作将永久删除你的账号及所有关联数据。",
            impactDescription = "删除后无法恢复，所有角色、对话、记忆和设置都将被清除。",
            confirmText = "永久删除",
            dangerLevel = DangerLevel.Three
        )
    }
}

@Composable
private fun AccountScreenContent(
    account: AccountInfo,
    onBack: () -> Unit,
    onLogout: () -> Unit,
    onDeleteAccount: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "账号", onBack = onBack) }
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
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(AmitiaSpacing.Base),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                ) {
                    Box(
                        modifier = Modifier.size(64.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        androidx.compose.material3.Icon(
                            imageVector = AmitiaIcons.AccountCircle,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(56.dp)
                        )
                    }
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = account.userName,
                            style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onSurface,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = account.plan,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                        if (account.userEmail.isNotBlank()) {
                            Text(
                                text = account.userEmail,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "登录方式") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "登录方式",
                            subtitle = account.loginMethod,
                            leadingIcon = AmitiaIcons.Security
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "修改密码",
                            subtitle = "更改账户密码",
                            leadingIcon = AmitiaIcons.Lock,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "当前服务") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "服务套餐",
                            subtitle = account.plan,
                            leadingIcon = AmitiaIcons.Star,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "使用情况",
                            subtitle = "查看用量统计",
                            leadingIcon = AmitiaIcons.Assessment,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "设备") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        account.devices.forEachIndexed { index, device ->
                            SettingsRow(
                                title = device.name,
                                subtitle = "${device.platform} · ${device.status}",
                                leadingIcon = AmitiaIcons.Devices
                            )
                            if (index < account.devices.lastIndex) {
                                AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            AmitiaSection(title = "账户操作") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "退出登录",
                            subtitle = "清除本地会话",
                            leadingIcon = AmitiaIcons.Logout,
                            onClick = onLogout
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "删除账号",
                            subtitle = "永久删除账号和数据",
                            leadingIcon = AmitiaIcons.DeleteForever,
                            onClick = onDeleteAccount
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
fun AccountScreenScaffold(
    onBack: () -> Unit,
    onLogout: () -> Unit,
    state: ScreenState<AccountInfo> = ScreenState.Content(AccountInfo())
) {
    when (state) {
        is ScreenState.Loading -> Column(
            modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) { repeat(4) { LoadingSkeleton(lineCount = 3) } }
        is ScreenState.Content -> AccountScreenContent(
            account = state.data,
            onBack = onBack,
            onLogout = onLogout,
            onDeleteAccount = {}
        )
        is ScreenState.Empty -> AmitiaEmptyState(
            icon = AmitiaIcons.Person,
            title = "未登录",
            description = "请先登录以查看账号信息"
        )
        is ScreenState.Error -> AmitiaErrorState(
            icon = AmitiaIcons.Error,
            title = "加载失败",
            description = state.error.message
        )
        is ScreenState.Partial -> AccountScreenContent(
            account = state.data,
            onBack = onBack,
            onLogout = onLogout,
            onDeleteAccount = {}
        )
    }
}

@Preview(name = "账号页 - 亮色", showBackground = true)
@Composable
private fun AccountScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AccountScreenContent(
            account = AccountInfo(
                userName = "测试用户",
                userEmail = "test@amitia.com",
                loginMethod = "邮箱登录",
                plan = "专业版",
                devices = listOf(
                    DeviceInfo("当前设备", "Android", "在线"),
                    DeviceInfo("桌面端", "Windows", "离线")
                )
            ),
            onBack = {},
            onLogout = {},
            onDeleteAccount = {}
        )
    }
}

@Preview(name = "账号页 - 暗色", showBackground = true)
@Composable
private fun AccountScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AccountScreenContent(
            account = AccountInfo(),
            onBack = {},
            onLogout = {},
            onDeleteAccount = {}
        )
    }
}
