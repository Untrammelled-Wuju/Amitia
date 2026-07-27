package com.amitia.feature.settings.security

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
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
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.feature.settings.SecuritySettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun SecurityScreen(
    onBack: () -> Unit,
    onNavigateAppLock: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val security = state.security

    SecurityScreenContent(
        settings = security,
        onBack = onBack,
        onChange = { viewModel.updateSecurity(it) },
        onNavigateAppLock = onNavigateAppLock
    )
}

@Composable
private fun SecurityScreenContent(
    settings: SecuritySettings,
    onBack: () -> Unit,
    onChange: (SecuritySettings) -> Unit,
    onNavigateAppLock: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "安全", onBack = onBack) }
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
            AmitiaSection(title = "应用安全") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "应用锁",
                            subtitle = "进入应用时需要验证",
                            checked = settings.appLockEnabled,
                            onCheckedChange = { onChange(settings.copy(appLockEnabled = it)) },
                            leadingIcon = AmitiaIcons.Lock
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "生物识别",
                            subtitle = "使用指纹或面容解锁",
                            checked = settings.biometricEnabled,
                            onCheckedChange = { onChange(settings.copy(biometricEnabled = it)) },
                            leadingIcon = AmitiaIcons.Fingerprint
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "应用锁设置",
                            subtitle = "PIN、锁定时机、通知隐藏",
                            leadingIcon = AmitiaIcons.ScreenLock,
                            onClick = onNavigateAppLock
                        )
                    }
                }
            }
            AmitiaSection(title = "操作验证") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "敏感操作验证",
                            subtitle = "删除、导出等操作需二次确认",
                            checked = settings.sensitiveOperationVerify,
                            onCheckedChange = { onChange(settings.copy(sensitiveOperationVerify = it)) },
                            leadingIcon = AmitiaIcons.VerifiedUser
                        )
                    }
                }
            }
            AmitiaSection(title = "加密与密钥") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "密钥存储",
                            subtitle = settings.keyStoreStatus,
                            leadingIcon = AmitiaIcons.Key,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "数据加密",
                            subtitle = "本地数据加密状态",
                            leadingIcon = AmitiaIcons.Security,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "高级安全") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "Computer Use 安全",
                            subtitle = "限制计算机使用能力的操作范围",
                            checked = settings.computerUseSecurity,
                            onCheckedChange = { onChange(settings.copy(computerUseSecurity = it)) },
                            leadingIcon = AmitiaIcons.Devices
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "扩展权限严格模式",
                            subtitle = "扩展需要明确授权才能执行",
                            checked = settings.extensionPermissionStrict,
                            onCheckedChange = { onChange(settings.copy(extensionPermissionStrict = it)) },
                            leadingIcon = AmitiaIcons.Extension
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "安全页 - 亮色", showBackground = true)
@Composable
private fun SecurityScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SecurityScreenContent(
            settings = SecuritySettings(),
            onBack = {},
            onChange = {},
            onNavigateAppLock = {}
        )
    }
}

@Preview(name = "安全页 - 暗色", showBackground = true)
@Composable
private fun SecurityScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SecurityScreenContent(
            settings = SecuritySettings(appLockEnabled = true, biometricEnabled = true),
            onBack = {},
            onChange = {},
            onNavigateAppLock = {}
        )
    }
}
