package com.amitia.feature.settings.developer

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.background
import androidx.compose.ui.draw.clip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.feature.settings.DeveloperOptionsState
import com.amitia.feature.settings.SettingsCenterViewModel

private const val REQUIRED_TAPS = 7

@Composable
fun DeveloperOptionsScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val developer = state.developer

    DeveloperOptionsScreenContent(
        state = developer,
        onBack = onBack,
        onChange = { viewModel.updateDeveloper(it) }
    )
}

@Composable
private fun DeveloperOptionsScreenContent(
    state: DeveloperOptionsState,
    onBack: () -> Unit,
    onChange: (DeveloperOptionsState) -> Unit
) {
    var tapCount by remember { mutableIntStateOf(0) }
    var showDisableDialog by remember { mutableStateOf(false) }

    AmitiaPageScaffold(
        topBar = {
            AmitiaTopBar(
                title = "开发者选项",
                onBack = onBack,
                actions = {
                    if (state.enabled) {
                        androidx.compose.material3.IconButton(
                            onClick = { showDisableDialog = true }
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.DeleteOutlined,
                                contentDescription = "关闭开发者选项"
                            )
                        }
                    }
                }
            )
        }
    ) { padding ->
        if (!state.enabled) {
            DeveloperLockedContent(
                tapCount = tapCount,
                onVersionTap = {
                    tapCount++
                    if (tapCount >= REQUIRED_TAPS) {
                        onChange(state.copy(enabled = true))
                        tapCount = 0
                    }
                },
                modifier = Modifier.padding(padding)
            )
        } else {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = AmitiaSpacing.Base),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))

                AmitiaSection(title = "调试工具") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column {
                            AmitiaSwitchRow(
                                title = "Prompt Trace",
                                subtitle = "记录模型调用的完整提示词和响应",
                                checked = state.promptTrace,
                                onCheckedChange = { onChange(state.copy(promptTrace = it)) },
                                leadingIcon = AmitiaIcons.Psychology
                            )
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            AmitiaSwitchRow(
                                title = "网络日志",
                                subtitle = "记录所有网络请求和响应",
                                checked = state.networkLog,
                                onCheckedChange = { onChange(state.copy(networkLog = it)) },
                                leadingIcon = AmitiaIcons.Lan
                            )
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            AmitiaSwitchRow(
                                title = "运行时控制台",
                                subtitle = "显示运行时服务控制台入口",
                                checked = state.runtimeConsole,
                                onCheckedChange = { onChange(state.copy(runtimeConsole = it)) },
                                leadingIcon = AmitiaIcons.Terminal
                            )
                            AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            AmitiaSwitchRow(
                                title = "UI 调试",
                                subtitle = "显示布局边界、重绘指示器等调试信息",
                                checked = state.uiDebug,
                                onCheckedChange = { onChange(state.copy(uiDebug = it)) },
                                leadingIcon = AmitiaIcons.Visibility
                            )
                        }
                    }
                }

                AmitiaSection(title = "实验功能") {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        AmitiaSwitchRow(
                            title = "实验性功能",
                            subtitle = "启用尚未稳定的功能，可能影响稳定性",
                            checked = state.experimentalFeatures,
                            onCheckedChange = { onChange(state.copy(experimentalFeatures = it)) },
                            leadingIcon = AmitiaIcons.Science
                        )
                    }
                }

                Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
            }
        }
    }

    if (showDisableDialog) {
        AmitiaConfirmDialog(
            onDismiss = { showDisableDialog = false },
            onConfirm = {
                onChange(DeveloperOptionsState(enabled = false))
                showDisableDialog = false
            },
            title = "关闭开发者选项",
            message = "将关闭所有开发者功能并重置设置。",
            confirmText = "关闭",
            destructive = true
        )
    }
}

@Composable
private fun DeveloperLockedContent(
    tapCount: Int,
    onVersionTap: () -> Unit,
    modifier: Modifier = Modifier
) {
    val remaining = REQUIRED_TAPS - tapCount
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(AmitiaSpacing.Xxl),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Box(
            modifier = Modifier
                .size(80.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.Code,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                modifier = Modifier.size(AmitiaIconSize.Huge)
            )
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
        Text(
            text = "开发者选项已隐藏",
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        Text(
            text = "连续点击下方版本号 ${remaining} 次以开启开发者选项",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
        Box(
            modifier = Modifier
                .clip(MaterialTheme.shapes.medium)
                .clickable(onClick = onVersionTap)
                .padding(horizontal = AmitiaSpacing.Xl, vertical = AmitiaSpacing.Base)
                .background(MaterialTheme.colorScheme.surfaceVariant)
        ) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                Text(
                    text = "Amitia",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = "版本 0.1.0 (Build 1)",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                )
            }
        }
        if (tapCount > 0 && tapCount < REQUIRED_TAPS) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "还需点击 $remaining 次",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.primary
            )
        }
    }
}

@Preview(name = "开发者选项页 - 锁定状态 - 亮色", showBackground = true)
@Composable
private fun DeveloperOptionsScreenLockedLightPreview() {
    AmitiaTheme(darkTheme = false) {
        DeveloperOptionsScreenContent(
            state = DeveloperOptionsState(enabled = false),
            onBack = {},
            onChange = {}
        )
    }
}

@Preview(name = "开发者选项页 - 已开启 - 暗色", showBackground = true)
@Composable
private fun DeveloperOptionsScreenEnabledDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        DeveloperOptionsScreenContent(
            state = DeveloperOptionsState(
                enabled = true,
                promptTrace = true,
                networkLog = false,
                runtimeConsole = true,
                uiDebug = false,
                experimentalFeatures = true
            ),
            onBack = {},
            onChange = {}
        )
    }
}
