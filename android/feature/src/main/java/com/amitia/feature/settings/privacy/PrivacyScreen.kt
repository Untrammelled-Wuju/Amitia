package com.amitia.feature.settings.privacy

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
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.ScreenState
import com.amitia.feature.settings.PrivacySettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun PrivacyScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val privacy = state.privacy

    PrivacyScreenContent(
        settings = privacy,
        onBack = onBack,
        onChange = { viewModel.updatePrivacy(it) }
    )
}

@Composable
private fun PrivacyScreenContent(
    settings: PrivacySettings,
    onBack: () -> Unit,
    onChange: (PrivacySettings) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "隐私", onBack = onBack) }
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
            AmitiaSection(title = "数据存储") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "数据存储位置",
                            subtitle = settings.dataStorageLocation,
                            leadingIcon = AmitiaIcons.Storage,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "远程发送说明",
                            subtitle = "了解数据如何发送到远程服务",
                            leadingIcon = AmitiaIcons.Info,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "允许远程发送",
                            subtitle = "允许将数据发送到远程服务",
                            checked = settings.remoteSendEnabled,
                            onCheckedChange = { onChange(settings.copy(remoteSendEnabled = it)) },
                            leadingIcon = AmitiaIcons.CloudUpload
                        )
                    }
                }
            }
            AmitiaSection(title = "日志与诊断") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "日志脱敏",
                            subtitle = "自动脱敏日志中的敏感信息",
                            checked = settings.logAnonymization,
                            onCheckedChange = { onChange(settings.copy(logAnonymization = it)) },
                            leadingIcon = AmitiaIcons.Shield
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "诊断数据",
                            subtitle = "发送匿名诊断数据帮助改进",
                            checked = settings.diagnosticsEnabled,
                            onCheckedChange = { onChange(settings.copy(diagnosticsEnabled = it)) },
                            leadingIcon = AmitiaIcons.BugReport
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "使用分析",
                            subtitle = "收集匿名使用统计",
                            checked = settings.analyticsEnabled,
                            onCheckedChange = { onChange(settings.copy(analyticsEnabled = it)) },
                            leadingIcon = AmitiaIcons.Analytics
                        )
                    }
                }
            }
            AmitiaSection(title = "权限使用记录") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "查看权限使用记录",
                            subtitle = "查看应用权限访问历史",
                            leadingIcon = AmitiaIcons.History,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "隐私导出",
                            subtitle = "导出隐私相关数据",
                            leadingIcon = AmitiaIcons.Download,
                            onClick = {}
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "隐私页 - 亮色", showBackground = true)
@Composable
private fun PrivacyScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PrivacyScreenContent(
            settings = PrivacySettings(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "隐私页 - 暗色", showBackground = true)
@Composable
private fun PrivacyScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PrivacyScreenContent(
            settings = PrivacySettings(diagnosticsEnabled = true),
            onBack = {},
            onChange = {}
        )
    }
}
