package com.amitia.feature.settings.applock

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
import com.amitia.core.designsystem.component.AmitiaPasswordField
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.feature.settings.AppLockSettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AppLockScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val appLock = state.appLock

    AppLockScreenContent(
        settings = appLock,
        onBack = onBack,
        onChange = { viewModel.updateAppLock(it) }
    )
}

@Composable
private fun AppLockScreenContent(
    settings: AppLockSettings,
    onBack: () -> Unit,
    onChange: (AppLockSettings) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "应用锁", onBack = onBack) }
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
            AmitiaSection(title = "解锁方式") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "PIN 码",
                            subtitle = "使用数字 PIN 码解锁",
                            checked = settings.pinEnabled,
                            onCheckedChange = { onChange(settings.copy(pinEnabled = it)) },
                            leadingIcon = AmitiaIcons.Pin
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "生物识别",
                            subtitle = "使用指纹或面容解锁",
                            checked = settings.biometricEnabled,
                            onCheckedChange = { onChange(settings.copy(biometricEnabled = it)) },
                            leadingIcon = AmitiaIcons.Fingerprint
                        )
                    }
                }
            }
            if (settings.pinEnabled) {
                AmitiaSection(title = "设置 PIN 码") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            AmitiaPasswordField(
                                value = "",
                                onValueChange = {},
                                label = "输入新 PIN 码",
                                placeholder = "4-8 位数字"
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                            AmitiaPasswordField(
                                value = "",
                                onValueChange = {},
                                label = "确认 PIN 码",
                                placeholder = "再次输入"
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                            PrimaryButton(
                                text = "保存 PIN 码",
                                onClick = {},
                                modifier = Modifier.fillMaxWidth()
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "锁定时机") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "后台返回时锁定",
                            subtitle = "应用切到后台后立即锁定",
                            checked = settings.lockOnBackground,
                            onCheckedChange = { onChange(settings.copy(lockOnBackground = it)) },
                            leadingIcon = AmitiaIcons.ScreenLock
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "锁定延迟",
                            subtitle = settings.lockDelay,
                            leadingIcon = AmitiaIcons.Schedule,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "通知") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "隐藏通知内容",
                            subtitle = "锁屏时不显示消息内容",
                            checked = settings.hideNotificationContent,
                            onCheckedChange = { onChange(settings.copy(hideNotificationContent = it)) },
                            leadingIcon = AmitiaIcons.VisibilityOff
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "应用锁页 - 亮色", showBackground = true)
@Composable
private fun AppLockScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AppLockScreenContent(
            settings = AppLockSettings(pinEnabled = true),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "应用锁页 - 暗色", showBackground = true)
@Composable
private fun AppLockScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AppLockScreenContent(
            settings = AppLockSettings(),
            onBack = {},
            onChange = {}
        )
    }
}
