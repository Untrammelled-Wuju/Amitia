package com.amitia.feature.modelcenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.ModelCard

@Composable
fun ModelCenterHomeScreen(
    onBack: () -> Unit,
    onProviders: () -> Unit,
    onTextModels: () -> Unit,
    onVisionModels: () -> Unit,
    onVoiceModels: () -> Unit,
    onVectorModels: () -> Unit,
    onRouting: () -> Unit,
    onFallback: () -> Unit,
    onUsage: () -> Unit,
    onDiagnostics: () -> Unit,
    onModelDetail: (String) -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val modelsState by viewModel.modelsState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadModels() }
    ModelCenterHomeContent(
        state = modelsState,
        onBack = onBack,
        onProviders = onProviders,
        onTextModels = onTextModels,
        onVisionModels = onVisionModels,
        onVoiceModels = onVoiceModels,
        onVectorModels = onVectorModels,
        onRouting = onRouting,
        onFallback = onFallback,
        onUsage = onUsage,
        onDiagnostics = onDiagnostics,
        onModelDetail = onModelDetail,
        onRetry = viewModel::loadModels
    )
}

@Composable
fun ModelCenterHomeContent(
    state: ScreenState<List<ModelUiModel>>,
    onBack: () -> Unit,
    onProviders: () -> Unit,
    onTextModels: () -> Unit,
    onVisionModels: () -> Unit,
    onVoiceModels: () -> Unit,
    onVectorModels: () -> Unit,
    onRouting: () -> Unit,
    onFallback: () -> Unit,
    onUsage: () -> Unit,
    onDiagnostics: () -> Unit,
    onModelDetail: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "模型中心", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载模型...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.SmartToy,
                title = "暂无模型",
                description = "请先添加 Provider 和模型配置",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item {
                    AmitiaSection(title = "模型类型") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            AmitiaEntryCard(
                                onClick = onTextModels,
                                title = "文本模型",
                                subtitle = "${state.data.count { it.type == ModelType.LLM }} 个可用",
                                leading = { Icon(AmitiaIcons.TextFields, null, modifier = Modifier.size(20.dp)) }
                            )
                            AmitiaEntryCard(
                                onClick = onVisionModels,
                                title = "视觉模型",
                                subtitle = "${state.data.count { it.type == ModelType.Vision }} 个可用",
                                leading = { androidx.compose.material3.Icon(AmitiaIcons.Image, null, modifier = Modifier.size(AmitiaIconSize.Small)) }
                            )
                            AmitiaEntryCard(
                                onClick = onVoiceModels,
                                title = "语音模型",
                                subtitle = "${state.data.count { it.type == ModelType.TTS || it.type == ModelType.STT }} 个可用",
                                leading = { androidx.compose.material3.Icon(AmitiaIcons.Mic, null, modifier = Modifier.size(AmitiaIconSize.Small)) }
                            )
                            AmitiaEntryCard(
                                onClick = onVectorModels,
                                title = "向量模型",
                                subtitle = "${state.data.count { it.type == ModelType.Embedding }} 个可用",
                                leading = { Icon(AmitiaIcons.Layers, null, modifier = Modifier.size(20.dp)) }
                            )
                        }
                    }
                }
                item {
                    AmitiaSection(title = "Provider 与路由") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            AmitiaEntryCard(
                                onClick = onProviders,
                                title = "Provider 管理",
                                subtitle = "AI 服务提供商配置",
                                leading = { Icon(AmitiaIcons.Hub, null, modifier = Modifier.size(20.dp)) }
                            )
                            AmitiaEntryCard(
                                onClick = onRouting,
                                title = "模型路由",
                                subtitle = "默认模型与任务路由策略",
                                leading = { androidx.compose.material3.Icon(AmitiaIcons.Tune, null, modifier = Modifier.size(AmitiaIconSize.Small)) }
                            )
                            AmitiaEntryCard(
                                onClick = onFallback,
                                title = "回退链",
                                subtitle = "主模型与回退模型配置",
                                leading = { Icon(AmitiaIcons.RestartAlt, null, modifier = Modifier.size(20.dp)) }
                            )
                        }
                    }
                }
                item {
                    AmitiaSection(title = "用量与诊断") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            AmitiaEntryCard(
                                onClick = onUsage,
                                title = "用量统计",
                                subtitle = "请求数、Token、延迟、失败率",
                                leading = { Icon(AmitiaIcons.Analytics, null, modifier = Modifier.size(20.dp)) }
                            )
                            AmitiaEntryCard(
                                onClick = onDiagnostics,
                                title = "模型诊断",
                                subtitle = "认证、限流、上下文、工具调用",
                                leading = { Icon(AmitiaIcons.BugReport, null, modifier = Modifier.size(20.dp)) }
                            )
                        }
                    }
                }
                item {
                    AmitiaSection(title = "已启用模型") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            state.data.filter { it.enabled }.take(5).forEach { model ->
                                ModelCard(
                                    name = model.name,
                                    provider = model.provider,
                                    isActive = model.isDefault,
                                    capabilities = model.capabilities,
                                    onClick = { onModelDetail(model.id) }
                                )
                            }
                        }
                    }
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Preview(name = "ModelCenter Home - Light", showBackground = true)
@Composable
private fun ModelCenterHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModelCenterHomeContent(
            state = ScreenState.Content(sampleModels),
            onBack = {}, onProviders = {}, onTextModels = {}, onVisionModels = {},
            onVoiceModels = {}, onVectorModels = {}, onRouting = {}, onFallback = {},
            onUsage = {}, onDiagnostics = {}, onModelDetail = {}, onRetry = {}
        )
    }
}

@Preview(name = "ModelCenter Home - Dark", showBackground = true)
@Composable
private fun ModelCenterHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModelCenterHomeContent(
            state = ScreenState.Content(sampleModels),
            onBack = {}, onProviders = {}, onTextModels = {}, onVisionModels = {},
            onVoiceModels = {}, onVectorModels = {}, onRouting = {}, onFallback = {},
            onUsage = {}, onDiagnostics = {}, onModelDetail = {}, onRetry = {}
        )
    }
}

internal val sampleModels = listOf(
    ModelUiModel(id = "1", name = "GPT-4o", provider = "OpenAI", type = ModelType.LLM, enabled = true, isDefault = true, capabilities = listOf("128K", "流式", "工具调用")),
    ModelUiModel(id = "2", name = "text-embedding-3", provider = "OpenAI", type = ModelType.Embedding, enabled = true, dimension = 1536),
    ModelUiModel(id = "3", name = "tts-1", provider = "OpenAI", type = ModelType.TTS, enabled = false)
)
