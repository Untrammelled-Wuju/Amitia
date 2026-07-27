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
fun VisionModelListScreen(
    onBack: () -> Unit,
    onModelDetail: (String) -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.modelsState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadModels() }
    val filtered = (state as? ScreenState.Content)?.data?.filter { it.type == ModelType.Vision } ?: emptyList()
    VisionModelListContent(
        state = if (state is ScreenState.Content) ScreenState.Content(filtered) else state,
        onBack = onBack,
        onModelDetail = onModelDetail,
        onRetry = viewModel::loadModels
    )
}

@Composable
fun VisionModelListContent(
    state: ScreenState<List<ModelUiModel>>,
    onBack: () -> Unit,
    onModelDetail: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "视觉模型", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载视觉模型...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Image,
                title = "暂无视觉模型",
                description = "请添加支持视觉理解的模型",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(state.data, key = { it.id }) { model ->
                    VisionModelRow(model = model, onClick = { onModelDetail(model.id) })
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun VisionModelRow(model: ModelUiModel, onClick: () -> Unit) {
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
                        imageVector = AmitiaIcons.Image,
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
                InfoChip(label = "输入类型", value = model.inputTypes.joinToString(", ").ifBlank { "图片" })
                InfoChip(label = "图片限制", value = model.imageLimit?.let { "${it} 张" } ?: "未知")
                if (model.boundCharacters.isNotEmpty()) {
                    InfoChip(label = "使用角色", value = model.boundCharacters.size.toString())
                }
            }
        }
    }
}

@Composable
private fun InfoChip(label: String, value: String) {
    Surface(shape = RoundedCornerShape(8.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
        Text(
            text = "$label: $value",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
        )
    }
}

@Preview(name = "VisionModel List - Light", showBackground = true)
@Composable
private fun VisionModelListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VisionModelListContent(
            state = ScreenState.Content(listOf(
                ModelUiModel("1", "GPT-4o Vision", "OpenAI", ModelType.Vision, enabled = true, isDefault = true, inputTypes = listOf("图片"), imageLimit = 10)
            )),
            onBack = {}, onModelDetail = {}, onRetry = {}
        )
    }
}

@Preview(name = "VisionModel List - Dark", showBackground = true)
@Composable
private fun VisionModelListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VisionModelListContent(
            state = ScreenState.Content(listOf(
                ModelUiModel("1", "GPT-4o Vision", "OpenAI", ModelType.Vision, enabled = true, isDefault = true)
            )),
            onBack = {}, onModelDetail = {}, onRetry = {}
        )
    }
}
