package com.amitia.feature.settings.network

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
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.feature.settings.NetworkProxyInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun NetworkProxyScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val network = state.networkProxy

    NetworkProxyScreenContent(
        network = network,
        onBack = onBack,
        onChange = { viewModel.updateNetworkProxy(it) }
    )
}

@Composable
private fun NetworkProxyScreenContent(
    network: NetworkProxyInfo,
    onBack: () -> Unit,
    onChange: (NetworkProxyInfo) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "网络与代理", onBack = onBack) }
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
            AmitiaSection(title = "代理设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "启用代理",
                            subtitle = "通过代理服务器连接网络",
                            checked = network.proxyEnabled,
                            onCheckedChange = { onChange(network.copy(proxyEnabled = it)) },
                            leadingIcon = AmitiaIcons.VpnKey
                        )
                        if (network.proxyEnabled) {
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            SettingsRow(
                                title = "代理类型",
                                subtitle = network.proxyType,
                                leadingIcon = AmitiaIcons.Settings,
                                onClick = {}
                            )
                        }
                    }
                }
            }
            if (network.proxyEnabled) {
                AmitiaSection(title = "代理配置") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            AmitiaTextField(
                                value = network.proxyAddress,
                                onValueChange = { onChange(network.copy(proxyAddress = it)) },
                                label = "代理地址",
                                placeholder = "127.0.0.1",
                                leadingIcon = AmitiaIcons.Router
                            )
                            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                            AmitiaTextField(
                                value = network.proxyPort,
                                onValueChange = { onChange(network.copy(proxyPort = it)) },
                                label = "端口",
                                placeholder = "7890",
                                leadingIcon = AmitiaIcons.Tune
                            )
                        }
                    }
                }
            }
            AmitiaSection(title = "网络诊断") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "DNS 状态",
                            subtitle = network.dnsStatus,
                            leadingIcon = AmitiaIcons.Dns,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "证书状态",
                            subtitle = network.certificateStatus,
                            leadingIcon = AmitiaIcons.VerifiedUser,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "WebSocket 状态",
                            subtitle = network.websocketStatus,
                            leadingIcon = AmitiaIcons.Sensors,
                            onClick = {}
                        )
                    }
                }
                PrimaryButton(
                    text = "运行网络诊断",
                    onClick = {},
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Speed
                )
            }
            AmitiaSection(title = "下载设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "仅 Wi-Fi 下载大型运行时",
                            subtitle = "移动网络下不下载大文件",
                            checked = network.wifiOnlyDownload,
                            onCheckedChange = { onChange(network.copy(wifiOnlyDownload = it)) },
                            leadingIcon = AmitiaIcons.Wifi
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "网络与代理页 - 亮色", showBackground = true)
@Composable
private fun NetworkProxyScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        NetworkProxyScreenContent(
            network = NetworkProxyInfo(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "网络与代理页 - 暗色", showBackground = true)
@Composable
private fun NetworkProxyScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        NetworkProxyScreenContent(
            network = NetworkProxyInfo(proxyEnabled = true, proxyAddress = "127.0.0.1", proxyPort = "7890"),
            onBack = {},
            onChange = {}
        )
    }
}
