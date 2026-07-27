package com.amitia.feature.settings.about

import androidx.compose.foundation.layout.Arrangement
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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.feature.settings.AboutInfo
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun AboutScreen(
    onBack: () -> Unit,
    onNavigateLicenses: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val about = state.about

    AboutScreenContent(
        about = about,
        onBack = onBack,
        onNavigateLicenses = onNavigateLicenses
    )
}

@Composable
private fun AboutScreenContent(
    about: AboutInfo,
    onBack: () -> Unit,
    onNavigateLicenses: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "关于", onBack = onBack) }
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
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(AmitiaSpacing.Xl),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    androidx.compose.material3.Icon(
                        imageVector = AmitiaIcons.AutoAwesome,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(64.dp)
                    )
                    Text(
                        text = about.appName,
                        style = MaterialTheme.typography.headlineMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Text(
                        text = "v${about.version} (${about.buildNumber})",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "智能角色陪伴助手",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            AmitiaSection(title = "项目信息") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "开源许可",
                            subtitle = "第三方组件许可",
                            leadingIcon = AmitiaIcons.Code,
                            onClick = onNavigateLicenses
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "项目链接",
                            subtitle = "GitHub 仓库",
                            leadingIcon = AmitiaIcons.Link,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "隐私政策",
                            subtitle = "查看隐私政策",
                            leadingIcon = AmitiaIcons.Shield,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "用户协议",
                            subtitle = "查看用户协议",
                            leadingIcon = AmitiaIcons.Gavel,
                            onClick = {}
                        )
                    }
                }
            }
            AmitiaSection(title = "团队与致谢") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "开发团队",
                            subtitle = about.team,
                            leadingIcon = AmitiaIcons.People,
                            onClick = {}
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        SettingsRow(
                            title = "致谢",
                            subtitle = "感谢所有贡献者",
                            leadingIcon = AmitiaIcons.FavoriteHeart,
                            onClick = {}
                        )
                    }
                }
            }
            Text(
                text = "Made with care",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.fillMaxWidth().padding(vertical = AmitiaSpacing.Base),
                textAlign = androidx.compose.ui.text.style.TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "关于页 - 亮色", showBackground = true)
@Composable
private fun AboutScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        AboutScreenContent(
            about = AboutInfo(),
            onBack = {},
            onNavigateLicenses = {}
        )
    }
}

@Preview(name = "关于页 - 暗色", showBackground = true)
@Composable
private fun AboutScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        AboutScreenContent(
            about = AboutInfo(),
            onBack = {},
            onNavigateLicenses = {}
        )
    }
}
