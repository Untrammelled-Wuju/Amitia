package com.amitia.feature.settings.language

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
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
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.feature.settings.LanguageInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun LanguageScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val language = state.language

    LanguageScreenContent(
        language = language,
        onBack = onBack,
        onChange = { viewModel.updateLanguage(it) }
    )
}

@Composable
private fun LanguageScreenContent(
    language: LanguageInfo,
    onBack: () -> Unit,
    onChange: (LanguageInfo) -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "语言", onBack = onBack) }
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
            AmitiaSection(title = "语言设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSelectionRow(
                            title = "跟随系统",
                            subtitle = "自动匹配系统语言设置",
                            selected = language.followSystem,
                            onSelect = { onChange(language.copy(followSystem = true)) },
                            leadingIcon = AmitiaIcons.BrightnessAuto
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSelectionRow(
                            title = "简体中文",
                            subtitle = "zh-CN",
                            selected = !language.followSystem && language.currentLanguage == "zh-CN",
                            onSelect = {
                                onChange(language.copy(followSystem = false, currentLanguage = "zh-CN"))
                            },
                            leadingIcon = AmitiaIcons.Language
                        )
                    }
                }
            }
            AmitiaSection(title = "其他语言", subtitle = "即将支持") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        LanguageDisabledRow(
                            title = "English",
                            subtitle = "en-US"
                        )
                        AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                        LanguageDisabledRow(
                            title = "日本語",
                            subtitle = "ja-JP"
                        )
                        AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                        LanguageDisabledRow(
                            title = "한국어",
                            subtitle = "ko-KR"
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun LanguageDisabledRow(title: String, subtitle: String) {
    androidx.compose.foundation.layout.Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = 12.dp),
        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        androidx.compose.material3.Icon(
            imageVector = AmitiaIcons.Language,
            contentDescription = null,
            tint = androidx.compose.material3.MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f),
            modifier = Modifier.size(32.dp)
        )
        androidx.compose.foundation.layout.Column(modifier = Modifier.weight(1f)) {
            androidx.compose.material3.Text(
                text = title,
                style = androidx.compose.material3.MaterialTheme.typography.bodyLarge,
                color = androidx.compose.material3.MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f)
            )
            androidx.compose.material3.Text(
                text = "$subtitle · 即将支持",
                style = androidx.compose.material3.MaterialTheme.typography.bodySmall,
                color = androidx.compose.material3.MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f)
            )
        }
    }
}

@Preview(name = "语言页 - 亮色", showBackground = true)
@Composable
private fun LanguageScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        LanguageScreenContent(
            language = LanguageInfo(),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "语言页 - 暗色", showBackground = true)
@Composable
private fun LanguageScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        LanguageScreenContent(
            language = LanguageInfo(followSystem = false, currentLanguage = "zh-CN"),
            onBack = {},
            onChange = {}
        )
    }
}
