package com.amitia.feature.settings.accessibility

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
import com.amitia.feature.settings.AccessibilitySettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AccessibilityScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val settings = state.accessibility

    AccessibilityScreenContent(
        settings = settings,
        onBack = onBack,
        onChange = { viewModel.updateAccessibility(it) }
    )
}

@Composable
private fun AccessibilityScreenContent(
    settings: AccessibilitySettings,
    onBack: () -> Unit,
    onChange: (AccessibilitySettings) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "辅助功能", onBack = onBack) }
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
            AmitiaSection(title = "视觉辅助") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "高对比度",
                            subtitle = "增强文字与背景的对比度",
                            checked = settings.highContrast,
                            onCheckedChange = { onChange(settings.copy(highContrast = it)) },
                            leadingIcon = AmitiaIcons.Contrast
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "关闭玻璃模糊",
                            subtitle = "禁用所有模糊和玻璃效果",
                            checked = settings.disableBlur,
                            onCheckedChange = { onChange(settings.copy(disableBlur = it)) },
                            leadingIcon = AmitiaIcons.VisibilityOff
                        )
                    }
                }
            }
            AmitiaSection(title = "动效辅助") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "减少动态效果",
                            subtitle = "减少动画和过渡效果",
                            checked = settings.reduceMotion,
                            onCheckedChange = { onChange(settings.copy(reduceMotion = it)) },
                            leadingIcon = AmitiaIcons.Speed
                        )
                    }
                }
            }
            AmitiaSection(title = "交互辅助") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "大触控目标",
                            subtitle = "增大按钮和控件的触控区域",
                            checked = settings.largeTouchTarget,
                            onCheckedChange = { onChange(settings.copy(largeTouchTarget = it)) },
                            leadingIcon = AmitiaIcons.TouchApp
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "语音字幕",
                            subtitle = "语音消息显示文字字幕",
                            checked = settings.voiceCaption,
                            onCheckedChange = { onChange(settings.copy(voiceCaption = it)) },
                            leadingIcon = AmitiaIcons.GraphicEq
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "图谱列表替代模式",
                            subtitle = "以列表形式显示图谱和关系",
                            checked = settings.graphListMode,
                            onCheckedChange = { onChange(settings.copy(graphListMode = it)) },
                            leadingIcon = AmitiaIcons.ViewList
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "辅助功能页 - 亮色", showBackground = true)
@Composable
private fun AccessibilityScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AccessibilityScreenContent(
            settings = AccessibilitySettings(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "辅助功能页 - 暗色", showBackground = true)
@Composable
private fun AccessibilityScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AccessibilityScreenContent(
            settings = AccessibilitySettings(highContrast = true, reduceMotion = true),
            onBack = {},
            onChange = {}
        )
    }
}
