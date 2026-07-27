package com.amitia.feature.settings.appearance

import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaThemeConfig
import com.amitia.core.designsystem.AmitiaAppearance
import com.amitia.core.designsystem.BlurStrength
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.feature.settings.AppearanceSettings
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AppearanceScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val appearance = state.appearance

    AppearanceScreenContent(
        settings = appearance,
        onBack = onBack,
        onChange = { viewModel.updateAppearance(it) }
    )
}

@Composable
private fun AppearanceScreenContent(
    settings: AppearanceSettings,
    onBack: () -> Unit,
    onChange: (AppearanceSettings) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "外观", onBack = onBack) }
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
            AppearancePreview(settings = settings)
            AmitiaSection(title = "主题模式") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSelectionRow(
                            title = "跟随系统",
                            subtitle = "自动匹配系统深浅色设置",
                            selected = settings.appearance == AmitiaAppearance.System,
                            onSelect = { onChange(settings.copy(appearance = AmitiaAppearance.System)) },
                            leadingIcon = AmitiaIcons.BrightnessAuto
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "亮色",
                            subtitle = "始终使用亮色主题",
                            selected = settings.appearance == AmitiaAppearance.Light,
                            onSelect = { onChange(settings.copy(appearance = AmitiaAppearance.Light)) },
                            leadingIcon = AmitiaIcons.LightMode
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "暗色",
                            subtitle = "始终使用暗色主题",
                            selected = settings.appearance == AmitiaAppearance.Dark,
                            onSelect = { onChange(settings.copy(appearance = AmitiaAppearance.Dark)) },
                            leadingIcon = AmitiaIcons.DarkMode
                        )
                    }
                }
            }
            AmitiaSection(title = "色彩与模糊") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "动态色",
                            subtitle = "使用系统壁纸颜色生成配色",
                            checked = settings.dynamicColor,
                            onCheckedChange = { onChange(settings.copy(dynamicColor = it)) },
                            leadingIcon = AmitiaIcons.Palette
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "模糊强度：标准",
                            selected = settings.blurStrength == BlurStrength.Standard,
                            onSelect = { onChange(settings.copy(blurStrength = BlurStrength.Standard)) },
                            leadingIcon = AmitiaIcons.BrightnessAuto
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "模糊强度：降低",
                            subtitle = "减少模糊效果，提升性能",
                            selected = settings.blurStrength == BlurStrength.Reduced,
                            onSelect = { onChange(settings.copy(blurStrength = BlurStrength.Reduced)) },
                            leadingIcon = AmitiaIcons.Tune
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "模糊强度：关闭",
                            subtitle = "不使用任何模糊效果",
                            selected = settings.blurStrength == BlurStrength.Off,
                            onSelect = { onChange(settings.copy(blurStrength = BlurStrength.Off)) },
                            leadingIcon = AmitiaIcons.VisibilityOff
                        )
                    }
                }
            }
            AmitiaSection(title = "显示与动效") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "高对比度",
                            subtitle = "增强文字与背景对比",
                            checked = settings.highContrast,
                            onCheckedChange = { onChange(settings.copy(highContrast = it)) },
                            leadingIcon = AmitiaIcons.Contrast
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "减少动态效果",
                            subtitle = "减少动画和过渡效果",
                            checked = settings.reduceMotion,
                            onCheckedChange = { onChange(settings.copy(reduceMotion = it)) },
                            leadingIcon = AmitiaIcons.Speed
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "字体大小",
                            subtitle = "跟随系统",
                            leadingIcon = AmitiaIcons.FormatSize,
                            onClick = {}
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun AppearancePreview(settings: AppearanceSettings) {
    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Text(
                text = "实时预览",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            AmitiaTheme(config = settings.toThemeConfig()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    PrimaryButton(
                        text = "主按钮",
                        onClick = {},
                        modifier = Modifier.weight(1f)
                    )
                    SecondaryButton(
                        text = "次按钮",
                        onClick = {},
                        modifier = Modifier.weight(1f)
                    )
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Box(
                        modifier = Modifier
                            .size(32.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primary)
                    )
                    Text(
                        text = "主题色预览区域",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }
        }
    }
}

@Preview(name = "外观页 - 亮色", showBackground = true)
@Composable
private fun AppearanceScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AppearanceScreenContent(
            settings = AppearanceSettings(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "外观页 - 暗色", showBackground = true)
@Composable
private fun AppearanceScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AppearanceScreenContent(
            settings = AppearanceSettings(appearance = AmitiaAppearance.Dark),
            onBack = {},
            onChange = {}
        )
    }
}
