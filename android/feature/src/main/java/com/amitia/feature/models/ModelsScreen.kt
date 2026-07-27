package com.amitia.feature.models

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.model.ModelDto

@Composable
fun ModelsScreen(
    onBack: () -> Unit,
    viewModel: ModelsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "模型",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    IconButton(onClick = viewModel::load) {
                        Icon(Icons.Outlined.Refresh, contentDescription = "刷新")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { padding ->
        when {
            state.loading && state.models.isEmpty() -> Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { AmitiaLoadingIndicator() }
            state.models.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Memory,
                title = "尚无模型",
                description = "在 Onboarding 或后端配置中添加模型",
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
            )
            else -> LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                item { AmitiaSectionHeader(title = "对话模型") }
                items(state.models.filter { it.type == "llm" || it.type.isNullOrBlank() }, key = { it.id }) { model ->
                    ModelRow(
                        model = model,
                        selected = state.config?.currentModelId == model.id,
                        onClick = { viewModel.setCurrentModel(model.id) }
                    )
                }
                item { AmitiaSectionHeader(title = "Embedding 模型") }
                items(state.models.filter { it.type == "embedding" }, key = { it.id }) { model ->
                    ModelRow(
                        model = model,
                        selected = state.config?.currentEmbeddingModelId == model.id,
                        onClick = { viewModel.setEmbeddingModel(model.id) }
                    )
                }
                item { AmitiaSectionHeader(title = "TTS 模型") }
                items(state.models.filter { it.type == "tts" }, key = { it.id }) { model ->
                    ModelRow(
                        model = model,
                        selected = state.config?.currentTtsModelId == model.id,
                        onClick = { viewModel.setTtsModel(model.id) }
                    )
                }
                item { AmitiaSectionHeader(title = "视觉模型") }
                items(state.models.filter { it.type == "vision" }, key = { it.id }) { model ->
                    ModelRow(
                        model = model,
                        selected = state.config?.currentVisionModelId == model.id,
                        onClick = { viewModel.setVisionModel(model.id) }
                    )
                }
            }
        }
    }
}

@Composable
private fun ModelRow(model: ModelDto, selected: Boolean, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = model.name,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(2.dp))
                Text(
                    text = "${model.provider ?: "未知"} · ${model.type ?: "未分类"}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                val apiKey = model.apiKey
                if (!apiKey.isNullOrBlank()) {
                    Text(
                        text = "API Key: ${maskApiKey(apiKey)}",
                        style = MaterialTheme.typography.labelSmall,
                        color = AmitiaColors.OnSurfaceMuted
                    )
                }
            }
            if (selected) {
                Text(
                    text = "已启用",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onPrimaryContainer
                )
            }
        }
    }
}

private fun maskApiKey(key: String): String {
    if (key.length <= 8) return "***"
    return key.take(4) + "****" + key.takeLast(4)
}
