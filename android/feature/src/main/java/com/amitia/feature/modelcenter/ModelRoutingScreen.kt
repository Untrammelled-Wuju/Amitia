package com.amitia.feature.modelcenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SettingsRow

@Composable
fun ModelRoutingScreen(
    onBack: () -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.routingState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadRouting() }
    ModelRoutingContent(state = state, onBack = onBack, onRetry = viewModel::loadRouting)
}

@Composable
fun ModelRoutingContent(
    state: ScreenState<RoutingConfigUiModel>,
    onBack: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "模型路由", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = androidx.compose.ui.Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载路由配置...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Tune,
                title = "暂无路由配置",
                description = "请先添加模型",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> {
                val config = state.data
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                ) {
                    AmitiaSection(title = "默认模型") {
                        SettingsRow(
                            title = "默认文本模型",
                            subtitle = config.defaultModelName ?: "未设置",
                            leadingIcon = AmitiaIcons.SmartToy,
                            onClick = {}
                        )
                    }
                    AmitiaSection(title = "角色专属模型") {
                        if (config.characterRoutes.isEmpty()) {
                            SettingsRow(
                                title = "暂无角色专属路由",
                                subtitle = "所有角色使用默认模型",
                                leadingIcon = AmitiaIcons.Person
                            )
                        } else {
                            config.characterRoutes.forEach { route ->
                                SettingsRow(
                                    title = route.characterName,
                                    subtitle = "使用: ${route.modelName}",
                                    leadingIcon = AmitiaIcons.Person,
                                    onClick = {}
                                )
                            }
                        }
                    }
                    AmitiaSection(title = "任务类型路由") {
                        SettingsRow(
                            title = "对话生成",
                            subtitle = config.defaultModelName ?: "默认模型",
                            leadingIcon = AmitiaIcons.Chat,
                            onClick = {}
                        )
                        SettingsRow(
                            title = "摘要生成",
                            subtitle = "默认模型",
                            leadingIcon = AmitiaIcons.Assignment,
                            onClick = {}
                        )
                        SettingsRow(
                            title = "情感分析",
                            subtitle = "默认模型",
                            leadingIcon = AmitiaIcons.Psychology,
                            onClick = {}
                        )
                    }
                    AmitiaSection(title = "路由优先级") {
                        RoutingPriority.entries.forEach { priority ->
                            AmitiaSelectionRow(
                                title = priority.label,
                                selected = config.priorityMode == priority,
                                onSelect = {},
                                leadingIcon = when (priority) {
                                    RoutingPriority.CostFirst -> AmitiaIcons.SavingsFallback()
                                    RoutingPriority.LatencyFirst -> AmitiaIcons.Speed
                                    RoutingPriority.Balanced -> AmitiaIcons.Tune
                                    RoutingPriority.QualityFirst -> AmitiaIcons.Star
                                }
                            )
                        }
                    }
                    AmitiaSection(title = "本地/远程优先") {
                        AmitiaSwitchRow(
                            title = "本地模型优先",
                            subtitle = "优先使用本地部署的模型",
                            checked = config.localFirst,
                            onCheckedChange = {},
                            leadingIcon = AmitiaIcons.Storage
                        )
                    }
                    LoadingButton(
                        text = "保存路由配置",
                        onClick = {},
                        modifier = Modifier.fillMaxWidth(),
                        leadingIcon = AmitiaIcons.Check
                    )
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

private fun AmitiaIcons.SavingsFallback() = AmitiaIcons.Storage

@Preview(name = "ModelRouting - Light", showBackground = true)
@Composable
private fun ModelRoutingLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModelRoutingContent(
            state = ScreenState.Content(RoutingConfigUiModel(
                defaultModelName = "GPT-4o",
                priorityMode = RoutingPriority.Balanced,
                characterRoutes = listOf(CharacterRouteUiModel("1", "艾米", "2", "Claude 3.5"))
            )),
            onBack = {}, onRetry = {}
        )
    }
}

@Preview(name = "ModelRouting - Dark", showBackground = true)
@Composable
private fun ModelRoutingDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModelRoutingContent(
            state = ScreenState.Content(RoutingConfigUiModel(defaultModelName = "GPT-4o")),
            onBack = {}, onRetry = {}
        )
    }
}
