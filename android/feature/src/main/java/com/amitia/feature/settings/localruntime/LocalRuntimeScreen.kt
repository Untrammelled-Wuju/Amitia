package com.amitia.feature.settings.localruntime

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton
import com.amitia.feature.settings.LocalRuntimeInfo
import com.amitia.feature.settings.RuntimeService
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun LocalRuntimeScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val runtime = state.localRuntime

    LocalRuntimeScreenContent(
        runtime = runtime,
        onBack = onBack
    )
}

@Composable
private fun LocalRuntimeScreenContent(
    runtime: LocalRuntimeInfo,
    onBack: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "本地运行时", onBack = onBack) }
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
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "运行时状态",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Text(
                            text = "运行中",
                            style = MaterialTheme.typography.titleLarge,
                            color = AmitiaStateColors.Running
                        )
                    }
                    AmitiaStatusDot(color = AmitiaStateColors.Running)
                    Row(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        SecondaryButton(text = "停止", onClick = {}, leadingIcon = AmitiaIcons.Stop)
                        PrimaryButton(text = "重启", onClick = {}, leadingIcon = AmitiaIcons.RestartAlt)
                    }
                }
            }
            AmitiaSection(title = "服务列表") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        runtime.services.forEachIndexed { index, service ->
                            RuntimeServiceRow(service = service)
                            if (index < runtime.services.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            AmitiaSection(title = "启动设置") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "自动启动",
                            subtitle = "应用启动时自动运行运行时",
                            checked = true,
                            onCheckedChange = {},
                            leadingIcon = AmitiaIcons.PowerSettingsNew
                        )
                        AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                        AmitiaSwitchRow(
                            title = "后台保活",
                            subtitle = "应用在后台时保持运行时",
                            checked = true,
                            onCheckedChange = {},
                            leadingIcon = AmitiaIcons.Sync
                        )
                    }
                }
            }
            AmitiaSection(title = "资源使用摘要") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(AmitiaSpacing.Base),
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "CPU",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "12%",
                                style = MaterialTheme.typography.titleLarge,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "内存",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "384MB",
                                style = MaterialTheme.typography.titleLarge,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "磁盘",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            Text(
                                text = "1.2GB",
                                style = MaterialTheme.typography.titleLarge,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun RuntimeServiceRow(service: RuntimeService) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Text(
                    text = service.name,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "v${service.version}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            if (service.metrics.isNotEmpty()) {
                Text(
                    text = service.metrics.entries.joinToString(" · ") { "${it.key}: ${it.value}" },
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        AmitiaStatusDot(color = AmitiaStateColors.Running)
        Text(
            text = service.status,
            style = MaterialTheme.typography.labelMedium,
            color = AmitiaStateColors.Running
        )
    }
}

@Preview(name = "本地运行时页 - 亮色", showBackground = true)
@Composable
private fun LocalRuntimeScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        LocalRuntimeScreenContent(
            runtime = LocalRuntimeInfo(),
            onBack = {}
        )
    }
}

@Preview(name = "本地运行时页 - 暗色", showBackground = true)
@Composable
private fun LocalRuntimeScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        LocalRuntimeScreenContent(
            runtime = LocalRuntimeInfo(),
            onBack = {}
        )
    }
}
