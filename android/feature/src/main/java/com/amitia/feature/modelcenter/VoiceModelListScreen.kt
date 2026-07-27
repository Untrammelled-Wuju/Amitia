package com.amitia.feature.modelcenter

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun VoiceModelListScreen(
    onBack: () -> Unit,
    onModelDetail: (String) -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.modelsState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadModels() }
    val filtered = (state as? ScreenState.Content)?.data?.filter { it.type == ModelType.TTS || it.type == ModelType.STT } ?: emptyList()
    VoiceModelListContent(
        state = if (state is ScreenState.Content) ScreenState.Content(filtered) else state,
        onBack = onBack,
        onModelDetail = onModelDetail,
        onRetry = viewModel::loadModels
    )
}

@Composable
fun VoiceModelListContent(
    state: ScreenState<List<ModelUiModel>>,
    onBack: () -> Unit,
    onModelDetail: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "语音模型", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载语音模型...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Mic,
                title = "暂无语音模型",
                description = "请添加 TTS 或 STT 模型",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> {
                val ttsModels = state.data.filter { it.type == ModelType.TTS }
                val sttModels = state.data.filter { it.type == ModelType.STT }
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    if (ttsModels.isNotEmpty()) {
                        item { Text("TTS 模型", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(vertical = AmitiaSpacing.Xs)) }
                        items(ttsModels, key = { it.id }) { model ->
                            VoiceModelRow(model = model, onClick = { onModelDetail(model.id) })
                        }
                    }
                    if (sttModels.isNotEmpty()) {
                        item { Text("STT 模型", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(vertical = AmitiaSpacing.Xs)) }
                        items(sttModels, key = { it.id }) { model ->
                            VoiceModelRow(model = model, onClick = { onModelDetail(model.id) })
                        }
                    }
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun VoiceModelRow(model: ModelUiModel, onClick: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = if (model.isDefault) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surface,
        onClick = onClick
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Box(
                    modifier = Modifier.size(40.dp).clip(CircleShape)
                        .background(if (model.isDefault) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (model.type == ModelType.TTS) AmitiaIcons.MusicNote else AmitiaIcons.GraphicEq,
                        contentDescription = null,
                        tint = if (model.isDefault) MaterialTheme.colorScheme.onPrimary
                        else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(20.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = model.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = model.provider,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Box(
                    modifier = Modifier.size(8.dp).clip(CircleShape)
                        .background(if (model.enabled) AmitiaStateColors.Connected else AmitiaStateColors.Disconnected)
                )
            }
            Row(
                modifier = Modifier.padding(top = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                VoiceInfoChip(label = "语言", value = model.languages.joinToString(", ").ifBlank { "多语言" })
                if (model.streaming) VoiceInfoChip(label = "流式", value = "支持")
                model.latency?.let { VoiceInfoChip(label = "延迟", value = it) }
                model.voiceCount?.let { VoiceInfoChip(label = "声音数", value = it.toString()) }
            }
        }
    }
}

@Composable
private fun VoiceInfoChip(label: String, value: String) {
    Surface(shape = RoundedCornerShape(8.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
        Text(
            text = "$label: $value",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
        )
    }
}

@Preview(name = "VoiceModel List - Light", showBackground = true)
@Composable
private fun VoiceModelListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceModelListContent(
            state = ScreenState.Content(listOf(
                ModelUiModel("1", "tts-1", "OpenAI", ModelType.TTS, enabled = true, isDefault = true, streaming = true, languages = listOf("zh", "en")),
                ModelUiModel("2", "whisper-1", "OpenAI", ModelType.STT, enabled = true, languages = listOf("zh", "en"))
            )),
            onBack = {}, onModelDetail = {}, onRetry = {}
        )
    }
}

@Preview(name = "VoiceModel List - Dark", showBackground = true)
@Composable
private fun VoiceModelListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceModelListContent(
            state = ScreenState.Content(listOf(
                ModelUiModel("1", "tts-1", "OpenAI", ModelType.TTS, enabled = true, isDefault = true)
            )),
            onBack = {}, onModelDetail = {}, onRetry = {}
        )
    }
}
