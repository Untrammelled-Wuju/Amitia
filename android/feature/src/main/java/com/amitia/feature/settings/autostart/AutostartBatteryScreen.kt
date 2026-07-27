package com.amitia.feature.settings.autostart

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
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
import com.amitia.core.designsystem.component.WarningBanner
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.feature.settings.AutostartBatteryInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AutostartBatteryScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val info = state.autostartBattery

    AutostartBatteryScreenContent(
        info = info,
        onBack = onBack,
        onChange = { viewModel.updateAutostartBattery(it) }
    )
}

@Composable
private fun AutostartBatteryScreenContent(
    info: AutostartBatteryInfo,
    onBack: () -> Unit,
    onChange: (AutostartBatteryInfo) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "开机自启动与电池", onBack = onBack) }
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
            if (!info.autostartEnabled || info.batteryOptimized || info.backgroundRestricted) {
                WarningBanner(
                    message = when {
                        info.backgroundRestricted -> "应用被系统限制后台运行，部分功能可能不可用"
                        info.batteryOptimized -> "电池优化已开启，可能影响后台运行时服务"
                        else -> "开机自启动未开启，设备重启后需要手动启动"
                    },
                    onDismiss = null
                )
            }
            AmitiaSection(title = "开机自启动") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "开机自启动",
                            subtitle = "设备开机后自动启动应用",
                            checked = info.autostartEnabled,
                            onCheckedChange = { onChange(info.copy(autostartEnabled = it)) },
                            leadingIcon = AmitiaIcons.PowerSettingsNew
                        )
                    }
                }
            }
            AmitiaSection(title = "电池优化") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "忽略电池优化",
                            subtitle = "允许应用在后台持续运行",
                            checked = !info.batteryOptimized,
                            onCheckedChange = { onChange(info.copy(batteryOptimized = !it)) },
                            leadingIcon = AmitiaIcons.BatteryFull
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "电池优化状态",
                            subtitle = if (info.batteryOptimized) "已开启电池优化" else "已忽略电池优化",
                            leadingIcon = AmitiaIcons.BatteryAlert,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "后台运行") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "后台限制",
                            subtitle = "系统对应用的后台限制状态",
                            checked = info.backgroundRestricted,
                            onCheckedChange = { onChange(info.copy(backgroundRestricted = it)) },
                            leadingIcon = AmitiaIcons.Block
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "通知常驻",
                            subtitle = "保持常驻通知以维持后台运行",
                            checked = info.persistentNotification,
                            onCheckedChange = { onChange(info.copy(persistentNotification = it)) },
                            leadingIcon = AmitiaIcons.Notifications
                        )
                    }
                }
            }
            AmitiaSection(title = "厂商系统设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "自启动管理",
                            subtitle = "跳转到厂商自启动设置",
                            leadingIcon = AmitiaIcons.Settings,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "电池设置",
                            subtitle = "跳转到系统电池设置",
                            leadingIcon = AmitiaIcons.BatteryFull,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "应用信息",
                            subtitle = "跳转到应用详情页",
                            leadingIcon = AmitiaIcons.Info,
                            onClick = {}
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "开机自启动与电池页 - 亮色", showBackground = true)
@Composable
private fun AutostartBatteryScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AutostartBatteryScreenContent(
            info = AutostartBatteryInfo(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "开机自启动与电池页 - 暗色", showBackground = true)
@Composable
private fun AutostartBatteryScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AutostartBatteryScreenContent(
            info = AutostartBatteryInfo(batteryOptimized = true),
            onBack = {},
            onChange = {}
        )
    }
}
